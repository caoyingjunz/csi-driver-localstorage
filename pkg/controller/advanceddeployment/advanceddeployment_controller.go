/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package advanceddeployment

import (
	"context"
	"fmt"
	"time"

	"github.com/caoyingjunz/pixiu/pkg/controller"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/advanceddeployment/v1alpha1"
	adClientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	adInformers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions/advanceddeployment/v1alpha1"
	adListers "github.com/caoyingjunz/pixiu/pkg/generated/listers/advanceddeployment/v1alpha1"
)

const (
	maxRetries = 15

	BurstReplicas = 500
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = apps.SchemeGroupVersion.WithKind("Pixiu")

// PixiuController is responsible for synchronizing pixiu objects stored
// in the system.
type PixiuController struct {
	client        clientset.Interface
	podControl    controller.PodControlInterface
	eventRecorder record.EventRecorder

	adClient adClientset.Interface

	syncHandler               func(pKey string) error
	enqueueAdvancedDeployment func(advancedDeployment *appsv1alpha1.AdvancedDeployment)

	// podLister can list/get pods from the shared informer's store
	podLister corelisters.PodLister

	// adLister can list/get AdvancedDeployments from the shared informer's store
	adLister adListers.AdvancedDeploymentLister

	// podListerSynced returns true if the pod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	podListerSynced cache.InformerSynced

	adListerSynced cache.InformerSynced

	// PixiuController that need to be synced
	queue workqueue.RateLimitingInterface
}

// NewPixiuController creates a new PixiuController.
func NewPixiuController(
	adClient adClientset.Interface,
	adInformer adInformers.AdvancedDeploymentInformer,
	podInformer coreinformers.PodInformer,
	client clientset.Interface) (*PixiuController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("pixiu_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	pc := &PixiuController{
		client: client,
		podControl: controller.RealPodControl{
			KubeClient: client,
			Recorder:   eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "pixiu-controller"}),
		},
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "pixiu-controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pixiu"),
		adClient:      adClient,
	}

	adInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addAdvancedDeployment,
		UpdateFunc: pc.updateAdvancedDeployment,
		DeleteFunc: pc.deleteAdvancedDeployment,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: pc.deletePod,
	})

	pc.syncHandler = pc.syncAdvancedDeployment
	pc.enqueueAdvancedDeployment = pc.enqueue

	pc.podLister = podInformer.Lister()
	pc.adLister = adInformer.Lister()

	pc.podListerSynced = podInformer.Informer().HasSynced
	pc.adListerSynced = adInformer.Informer().HasSynced

	return pc, nil
}

func (pc *PixiuController) addAdvancedDeployment(obj interface{}) {
	ad := obj.(*appsv1alpha1.AdvancedDeployment)
	klog.V(4).Infof("AdvancedDeployment %s added.", ad.Name)

	pc.enqueueAdvancedDeployment(ad)
}

func (pc *PixiuController) updateAdvancedDeployment(old, cur interface{}) {
	oldAd := old.(*appsv1alpha1.AdvancedDeployment)
	curAd := cur.(*appsv1alpha1.AdvancedDeployment)
	if oldAd.ResourceVersion == curAd.ResourceVersion {
		return
	}

	klog.V(4).Infof("AdvancedDeployment %s updated.", curAd.Name)
	pc.enqueueAdvancedDeployment(curAd)
}

func (pc *PixiuController) deleteAdvancedDeployment(obj interface{}) {
	ad := obj.(*appsv1alpha1.AdvancedDeployment)
	klog.V(4).Infof("AdvancedDeployment %s deleted.", ad.Name)
	pc.enqueueAdvancedDeployment(ad)
}

func (pc *PixiuController) deletePod(obj interface{}) {
	pod := obj.(*v1.Pod)
	klog.V(4).Infof("Pod %s deleted.", pod.Name)
}

func (pc *PixiuController) enqueue(advancedDeployment *appsv1alpha1.AdvancedDeployment) {
	key, err := controller.KeyFunc(advancedDeployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", advancedDeployment, err))
		return
	}

	pc.queue.Add(key)
}

// Run begins watching and syncing.
func (pc *PixiuController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	klog.Infof("Starting Pixiu Controller")
	defer klog.Infof("Shutting down Pixiu Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("pixiu-controller", stopCh, pc.podListerSynced, pc.adListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(pc.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes then, and marks them done.
func (pc *PixiuController) worker() {
	for pc.processNextWorkItem() {
	}
}

func (pc *PixiuController) processNextWorkItem() bool {
	key, quit := pc.queue.Get()
	if quit {
		return false
	}
	defer pc.queue.Done(key)

	err := pc.syncHandler(key.(string))
	pc.handleErr(err, key)
	return true

}

func (pc *PixiuController) handleErr(err error, key interface{}) {
	if err == nil {
		pc.queue.Forget(key)
		return
	}

	if pc.queue.NumRequeues(key) < maxRetries {
		pc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	pc.queue.Forget(key)
}

// syncAdvancedDeployment will sync the advancedDeployment with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (pc *PixiuController) syncAdvancedDeployment(key string) error {
	startTime := time.Now()
	klog.V(4).Infof("Started syncing advanced deployment %q (%v)", key, startTime)
	defer func() {
		klog.V(4).Infof("Finished syncing advanced deployment %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	ad, err := pc.adLister.AdvancedDeployments(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("Advanced Deployment %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	selector, err := metav1.LabelSelectorAsSelector(ad.Spec.Selector)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("error converting pod selector to selector: %v", err))
		return nil
	}

	// list all pods to include the pods that don't match the ad`s selector
	allPods, err := pc.podLister.Pods(ad.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	// Ignore inactive pods.
	filteredPods := controller.FilterActivePods(allPods)
	filteredPods, err = pc.claimPods(ad, selector, filteredPods)
	if err != nil {
		return err
	}

	return pc.manageReplicas(filteredPods, ad)
}

func (pc *PixiuController) claimPods(ad *appsv1alpha1.AdvancedDeployment, selector labels.Selector, filteredPods []*v1.Pod) ([]*v1.Pod, error) {
	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Pods (see #42639).
	canAdoptFunc := controller.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := pc.adClient.AppsV1alpha1().AdvancedDeployments(ad.Namespace).Get(context.TODO(), ad.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != ad.UID {
			return nil, fmt.Errorf("original %v %v/%v is gone: got uid %v, wanted %v", ad.Kind, ad.Namespace, ad.Name, fresh.UID, ad.UID)
		}
		return fresh, nil
	})
	cm := controller.NewPodControllerRefManager(pc.podControl, ad, selector, controllerKind, canAdoptFunc)
	return cm.ClaimPods(filteredPods)
}

// manageReplicas checks and updates replicas for the given AdvancedDeployment.
// Does NOT modify <filteredPods>.
// It will requeue the replica set in case of an error while creating/deleting pods.
func (pc *PixiuController) manageReplicas(filteredPods []*v1.Pod, ad *appsv1alpha1.AdvancedDeployment) error {
	diff := len(filteredPods) - int(*(ad.Spec.Replicas))
	adKey, err := controller.KeyFunc(ad)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for %v %#v: %v", "Pixiu", ad, err))
		return nil
	}
	// TODO
	return nil
}
