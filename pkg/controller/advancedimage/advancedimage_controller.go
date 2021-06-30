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

package advancedimage

import (
	"fmt"
	"time"

	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
	pClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	pInformers "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions/apps/v1alpha1"
	pListers "github.com/caoyingjunz/pixiu/pkg/client/listers/apps/v1alpha1"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const (
	maxRetries = 15
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = apps.SchemeGroupVersion.WithKind("AdvancedImage")

// AdvancedImageController is responsible for synchronizing advancedImage objects stored
// in the system with actual running image sets.
type AdvancedImageController struct {
	// imgClient is used for adopting/releasing imgs.
	imgClient     pClientset.Interface
	client        clientset.Interface
	eventRecorder record.EventRecorder

	syncHandler          func(imgKey string) error
	enqueueAdvancedImage func(advancedImage *appsv1alpha1.AdvancedImage)

	// imgLister is able to list/get endpoints and is populated by the shared informer passed to
	// NewAdvancedImageController.
	imgLister pListers.AdvancedImageLister

	// imgListerSynced returns true if the img shared informer has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	imgListerSynced cache.InformerSynced

	// advancedImage that need to be updated. A channel is inappropriate here,
	//  it also would cause a advancedImage that's inserted multiple times to
	// be processed more than necessary.
	queue workqueue.RateLimitingInterface
}

func NewAdvancedImageController(
	aiClient pClientset.Interface,
	aiInformer pInformers.AdvancedImageInformer,
	isInformer pInformers.ImageSetInformer,
	client clientset.Interface) (*AdvancedImageController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("advancedimage_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	ai := &AdvancedImageController{
		client:        client,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "advancedimage-controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "advancedimage"),
		imgClient:     aiClient,
	}

	aiInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ai.addAdvancedImage,
		UpdateFunc: ai.updateAdvancedImage,
		DeleteFunc: ai.deleteAdvancedImage,
	})

	ai.syncHandler = ai.syncAdvancedImage
	ai.enqueueAdvancedImage = ai.enqueue

	ai.imgLister = aiInformer.Lister()
	ai.imgListerSynced = aiInformer.Informer().HasSynced

	return ai, nil
}

func (ai *AdvancedImageController) addAdvancedImage(obj interface{}) {
	img := obj.(*appsv1alpha1.AdvancedImage)
	klog.V(4).Infof("AdvancedImage %s added.", img.Name)

	ai.enqueueAdvancedImage(img)
}

func (ai *AdvancedImageController) updateAdvancedImage(old, cur interface{}) {
	oldImg := old.(*appsv1alpha1.AdvancedImage)
	curImg := cur.(*appsv1alpha1.AdvancedImage)
	if oldImg.ResourceVersion == curImg.ResourceVersion {
		return
	}

	klog.V(4).Infof("AdvancedImage %s updated.", curImg.Name)
	ai.enqueueAdvancedImage(curImg)
}

func (ai *AdvancedImageController) deleteAdvancedImage(obj interface{}) {
	img, ok := obj.(*appsv1alpha1.AdvancedImage)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		img, ok = tombstone.Obj.(*appsv1alpha1.AdvancedImage)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ReplicaSet %#v", obj))
			return
		}
	}
	klog.V(4).Infof("AdvancedImage %s deleted.", img.Name)
	ai.enqueueAdvancedImage(img)
}

func (ai *AdvancedImageController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ai.queue.ShutDown()

	klog.Infof("Starting AdvancedImage Controller")
	defer klog.Infof("Shutting down AdvancedImage Controller")

	if !cache.WaitForNamedCacheSync("advancedImage-controller", stopCh, ai.imgListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ai.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (ai *AdvancedImageController) enqueue(img *appsv1alpha1.AdvancedImage) {
	key, err := controller.KeyFunc(img)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", img, err))
		return
	}

	ai.queue.Add(key)
}

// syncAdvancedImage will sync the advancedImage with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (ai *AdvancedImageController) syncAdvancedImage(key string) error {
	startTime := time.Now()
	klog.V(4).Infof("Started syncing advanced deployment %q (%v)", key, startTime)
	defer func() {
		klog.V(4).Infof("Finished syncing advanced deployment %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	img, err := ai.imgLister.AdvancedImages(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("Advanced Deployment %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	klog.V(0).Infof("get adviced image is: %+v", img)

	return nil
}

func (ai *AdvancedImageController) worker() {
	for ai.processNextWorkItem() {
	}
}

func (ai *AdvancedImageController) processNextWorkItem() bool {
	key, quit := ai.queue.Get()
	if quit {
		return false
	}
	defer ai.queue.Done(key)

	err := ai.syncHandler(key.(string))
	ai.handleErr(err, key)

	return true
}

func (ai *AdvancedImageController) handleErr(err error, key interface{}) {
	if err == nil {
		ai.queue.Forget(key)
		return
	}

	if ai.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Infof("Error syncing pods for advanced deployments %q, retrying. Error: %v", key, err)
		ai.queue.AddRateLimited(key)
		return
	}

	klog.Warningf("Dropping advanced deployments %q out of the queue: %v", key, err)
	utilruntime.HandleError(err)
	ai.queue.Forget(key)
}
