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

package imageset

import (
	"fmt"
	"time"

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

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/imageset/v1alpha1"
	isClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	isInformers "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions/imageset/v1alpha1"
	isListers "github.com/caoyingjunz/pixiu/pkg/client/listers/imageset/v1alpha1"
	"github.com/caoyingjunz/pixiu/pkg/controller"
)

const maxRetries = 15

var controllerKind = v1.SchemeGroupVersion.WithKind("ImageSet")

// ImageSetController is responsible for synchronizing pixiu objects stored
// in the system.
type ImageSetController struct {
	client        clientset.Interface
	eventRecorder record.EventRecorder
	hostName      string

	isClient isClientset.Interface

	syncHandler     func(iKey string) error
	enqueueImageSet func(imageSet *appsv1alpha1.ImageSet)

	isLister isListers.ImageSetLister

	isListerSynced cache.InformerSynced

	// ImageSet that need to be synced
	queue workqueue.RateLimitingInterface
}

func NewImageSetController(
	isClient isClientset.Interface,
	isInformer isInformers.ImageSetInformer,
	client clientset.Interface,
	hostName string) (*ImageSetController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("pixiu_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	isc := &ImageSetController{
		client:        client,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "imageset-controller"}),
		hostName:      hostName,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "imageset"),
	}

	isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    isc.addImageSet,
		UpdateFunc: isc.updateImageSet,
		DeleteFunc: isc.deleteImageSet,
	})

	isc.syncHandler = isc.syncImageSet
	isc.enqueueImageSet = isc.enqueue

	isc.isLister = isInformer.Lister()
	isc.isListerSynced = isInformer.Informer().HasSynced

	return isc, nil
}

func (isc *ImageSetController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer isc.queue.ShutDown()

	klog.Infof("Starting imageSet Controller")
	defer klog.Infof("Shutting down imageSet Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("ImageSet-controller", stopCh, isc.isListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(isc.worker, time.Second, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes then, and marks them done.
func (isc *ImageSetController) worker() {
	for isc.processNextWorkItem() {
	}
}

func (isc *ImageSetController) processNextWorkItem() bool {
	key, quit := isc.queue.Get()
	if quit {
		return false
	}
	defer isc.queue.Done(key)

	err := isc.syncHandler(key.(string))
	isc.handleErr(err, key)

	return true
}

func (isc *ImageSetController) handleErr(err error, key interface{}) {
	if err == nil {
		isc.queue.Forget(key)
		return
	}

	if isc.queue.NumRequeues(key) < maxRetries {
		isc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	isc.queue.Forget(key)
}

func (isc *ImageSetController) syncImageSet(key string) error {
	startTime := time.Now()
	klog.V(4).Infof("Started syncing imageSet %q (%v)", key, startTime)
	defer func() {
		klog.V(4).Infof("Finished syncing imageSet %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	image, err := isc.isLister.ImageSets(namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("Image Sets %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	klog.Infof("image is: %v/%v, %v", image.Namespace, image.Name, image)
	return nil
}

func (isc *ImageSetController) enqueue(imageSet *appsv1alpha1.ImageSet) {
	key, err := controller.KeyFunc(imageSet)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", imageSet, err))
		return
	}

	isc.queue.Add(key)
}

func (isc *ImageSetController) addImageSet(obj interface{}) {
	is := obj.(*appsv1alpha1.ImageSet)
	klog.V(0).Infof("ImageSet %s added.", is.Name)

	isc.enqueueImageSet(is)
}

func (isc *ImageSetController) updateImageSet(old, cur interface{}) {
	oldImageSet := old.(*appsv1alpha1.ImageSet)
	curImageSet := cur.(*appsv1alpha1.ImageSet)
	if oldImageSet.ResourceVersion == curImageSet.ResourceVersion {
		return
	}
	klog.V(0).Infof("ImageSet updated.")
}

func (isc *ImageSetController) deleteImageSet(cur interface{}) {
	klog.V(0).Infof("ImageSet deleted.")
}
