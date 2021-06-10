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
	"k8s.io/apimachinery/pkg/types"
	"sort"
	"sync"
	"time"

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
	"k8s.io/utils/integer"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/advanceddeployment/v1alpha1"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	adClientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	adInformers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions/advanceddeployment/v1alpha1"
	adListers "github.com/caoyingjunz/pixiu/pkg/generated/listers/advanceddeployment/v1alpha1"
)

const (
	maxRetries = 15

	BurstReplicas = 500
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = apps.SchemeGroupVersion.WithKind("AdvancedDeployment")

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
		AddFunc:    pc.addPod,
		UpdateFunc: pc.updatePod,
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
	ad, ok := obj.(*appsv1alpha1.AdvancedDeployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		ad, ok = tombstone.Obj.(*appsv1alpha1.AdvancedDeployment)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ReplicaSet %#v", obj))
			return
		}
	}
	klog.V(4).Infof("AdvancedDeployment %s deleted.", ad.Name)
	pc.enqueueAdvancedDeployment(ad)
}

// When a pod is created, enqueue the AdvancedDeployment that manages it
func (pc *PixiuController) addPod(obj interface{}) {
	pod := obj.(*v1.Pod)

	if pod.DeletionTimestamp != nil {
		// on a restart of the controller manager, it's possible a new pod shows up in a state that
		// is already pending deletion. Prevent the pod from being a creation observation.
		pc.deletePod(pod)
		return
	}

	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef == nil {
		return
	}
	ad := pc.resolveControllerRef(pod.Namespace, controllerRef)
	if ad == nil {
		return
	}
	pc.enqueueAdvancedDeployment(ad)
}

func (pc *PixiuController) updatePod(obj, cur interface{}) {}

func (pc *PixiuController) deletePod(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	klog.V(2).Infof("Pod %s deleted.", pod.Name)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*v1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod %#v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef == nil {
		return
	}
	ad := pc.resolveControllerRef(pod.Namespace, controllerRef)
	pc.enqueueAdvancedDeployment(ad)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (pc *PixiuController) resolveControllerRef(namespace string, controllerRef *metav1.OwnerReference) *appsv1alpha1.AdvancedDeployment {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}
	ad, err := pc.adLister.AdvancedDeployments(namespace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}
	if ad.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return ad
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
		klog.V(0).Infof("Advanced Deployment %v has been deleted", key)
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

	var manageAdErr error
	if ad.DeletionTimestamp == nil {
		manageAdErr = pc.manageAdvancedDeployments(filteredPods, ad)
	}

	return manageAdErr
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

func slowStartBatch(count int, initialBatchSize int, fn func() error) (int, error) {
	remaining := count
	successes := 0
	for batchSize := integer.IntMin(remaining, initialBatchSize); batchSize > 0; batchSize = integer.IntMin(2*batchSize, remaining) {
		errCh := make(chan error, batchSize)
		var wg sync.WaitGroup
		wg.Add(batchSize)
		for i := 0; i < batchSize; i++ {
			go func() {
				defer wg.Done()
				if err := fn(); err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		curSuccesses := batchSize - len(errCh)
		successes += curSuccesses
		if len(errCh) > 0 {
			return successes, <-errCh
		}
		remaining -= batchSize
	}
	return successes, nil
}

// manageAdvancedDeployments checks and updates replicas for the given AdvancedDeployment.
// Does NOT modify <filteredPods>.
// It will requeue the replica set in case of an error while creating/deleting pods.
func (pc *PixiuController) manageAdvancedDeployments(filteredPods []*v1.Pod, ad *appsv1alpha1.AdvancedDeployment) error {
	diff := len(filteredPods) - int(*(ad.Spec.Replicas))
	// 小于 0 以为这需要增加 pod
	if diff < 0 {
		diff *= -1
		if diff > BurstReplicas {
			diff = BurstReplicas
		}
		klog.V(2).Infof("Too few replicas for %v %s/%s, need %d, creating %d", controllerKind.Kind, ad.Namespace, ad.Name, *(ad.Spec.Replicas), diff)

		fn := func() error {
			template := &ad.Spec.Template
			if template.Labels == nil {
				template.Labels = make(map[string]string)
				template.Labels["app"] = "nginx"
				template.Labels["controller"] = "example-ad"
			}

			err := pc.podControl.CreatePodsWithControllerRef(ad.Namespace, template, ad, metav1.NewControllerRef(ad, controllerKind))
			if errors.HasStatusCause(err, v1.NamespaceTerminatingCause) {
				return nil
			}
			return err
		}

		if _, err := slowStartBatch(diff, controller.SlowStartInitialBatchSize, fn); err != nil {
			return err
		}
	} else if diff > 0 {
		if diff > BurstReplicas {
			diff = BurstReplicas
		}
		klog.V(2).Infof("Too many replicas for %v %s/%s, need %d, deleting %d", controllerKind.Kind, ad.Namespace, ad.Name, *(ad.Spec.Replicas), diff)

		relatedPods, err := pc.getRelatedPods(ad)
		if err != nil {
			return err
		}

		// Choose which Pods to delete, preferring those in earlier phases of startup.
		podsToDelete := getPodsToDelete(filteredPods, relatedPods, diff)

		errCh := make(chan error, diff)
		var wg sync.WaitGroup
		wg.Add(diff)
		for _, p := range podsToDelete {
			go func(p *v1.Pod) {
				defer wg.Done()
				if err := pc.podControl.DeletePod(p.Namespace, p.Name, ad); err != nil {
					errCh <- err
				}
			}(p)
		}
		wg.Wait()

		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		default:
		}

	}

	return nil
}

// getRelatedPods returns a list of pods with the same owner as the given AdvancedDeployments.
func (pc *PixiuController) getRelatedPods(ad *appsv1alpha1.AdvancedDeployment) ([]*v1.Pod, error) {
	relatedPods := make([]*v1.Pod, 0)
	founds := make(map[types.UID]struct{})

	for _, relatedAD := range pc.getRelatedAdvancedDeployments(ad) {
		selector, err := metav1.LabelSelectorAsSelector(relatedAD.Spec.Selector)
		if err != nil {
			return nil, err
		}

		pods, err := pc.podLister.Pods(ad.Namespace).List(selector)
		if err != nil {
			return nil, err
		}

		for _, p := range pods {
			if _, exits := founds[p.UID]; exits {
				continue
			}
			founds[p.UID] = struct{}{}
			relatedPods = append(relatedPods, p)
		}
	}

	return relatedPods, nil
}

// getRelatedAdvancedDeployments returns a list of AdvancedDeployments with the same
// owner as the given AdvancedDeployments.
func (pc *PixiuController) getRelatedAdvancedDeployments(ad *appsv1alpha1.AdvancedDeployment) []*appsv1alpha1.AdvancedDeployment {
	controllerRef := metav1.GetControllerOf(ad)
	if controllerRef == nil {
		utilruntime.HandleError(fmt.Errorf("AdvancedDeployment has no controller: %v", ad))
		return nil
	}

	allADs, err := pc.adLister.AdvancedDeployments(ad.Namespace).List(labels.Everything())
	if err != nil {
		utilruntime.HandleError(err)
		return nil
	}

	var relatedADs []*appsv1alpha1.AdvancedDeployment
	for _, d := range allADs {
		if ref := metav1.GetControllerOf(d); ref != nil && ref.UID == ad.UID {
			relatedADs = append(relatedADs, d)
		}
	}

	return relatedADs
}

func getPodsToDelete(filteredPods, relatedPods []*v1.Pod, diff int) []*v1.Pod {
	// No need to sort pods if we are about to delete all of them.
	// diff will always be <= len(filteredPods), so not need to handle > case.
	if diff < len(filteredPods) {
		podsWithRanks := getPodsRankedByRelatedPodsOnSameNode(filteredPods, relatedPods)
		sort.Sort(podsWithRanks)
	}
	return filteredPods[:diff]
}

// getPodsRankedByRelatedPodsOnSameNode returns an ActivePodsWithRanks value
// that wraps podsToRank and assigns each pod a rank equal to the number of
// active pods in relatedPods that are colocated on the same node with the pod.
// relatedPods generally should be a superset of podsToRank.
func getPodsRankedByRelatedPodsOnSameNode(podsToRank, relatedPods []*v1.Pod) controller.ActivePodsWithRanks {
	podsOnNode := make(map[string]int)
	for _, pod := range relatedPods {
		if controller.IsPodActive(pod) {
			podsOnNode[pod.Spec.NodeName]++
		}
	}
	ranks := make([]int, len(podsToRank))
	for i, pod := range podsToRank {
		ranks[i] = podsOnNode[pod.Spec.NodeName]
	}
	return controller.ActivePodsWithRanks{Pods: podsToRank, Rank: ranks}
}
