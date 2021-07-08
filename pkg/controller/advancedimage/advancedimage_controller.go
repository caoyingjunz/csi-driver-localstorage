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
	"context"
	"fmt"
	"reflect"
	"time"

	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	pxClient      pClientset.Interface
	client        clientset.Interface
	eventRecorder record.EventRecorder

	syncHandler          func(imgKey string) error
	enqueueAdvancedImage func(advancedImage *appsv1alpha1.AdvancedImage)

	// imgLister is able to list/get endpoints and is populated by the shared informer passed to
	// NewAdvancedImageController.
	imgLister pListers.AdvancedImageLister

	// iSetLister is able to list/get imageSet from indexer
	iSetLister pListers.ImageSetLister

	// imgListerSynced returns true if the img shared informer has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	imgListerSynced cache.InformerSynced

	// iSetListerSynced returns true if the imageSet shared informer has been synced at least once.
	iSetListerSynced cache.InformerSynced

	// advancedImage that need to be updated. A channel is inappropriate here,
	// it also would cause a advancedImage that's inserted multiple times to
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
		pxClient:      aiClient,
	}

	aiInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ai.addAdvancedImage,
		UpdateFunc: ai.updateAdvancedImage,
		DeleteFunc: ai.deleteAdvancedImage,
	})
	ai.imgLister = aiInformer.Lister()
	ai.imgListerSynced = aiInformer.Informer().HasSynced

	isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ai.addImageSet,
		UpdateFunc: ai.updateImageSet,
		DeleteFunc: ai.deleteImageSet,
	})
	ai.iSetLister = isInformer.Lister()
	ai.iSetListerSynced = isInformer.Informer().HasSynced

	ai.syncHandler = ai.syncAdvancedImage
	ai.enqueueAdvancedImage = ai.enqueue

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

func (ai *AdvancedImageController) addImageSet(obj interface{}) {
	iSet := obj.(*appsv1alpha1.ImageSet)

	if iSet.DeletionTimestamp != nil {
		// On a restart of the controller, it's possible for an obj to show up in a state
		// that is already pending deletion.
		ai.deleteImageSet(iSet)
		return
	}

	// if it has a ControllerRef, that's all that matters
	if controllerRef := metav1.GetControllerOf(iSet); controllerRef != nil {
		img := ai.resolveControllerRef(iSet.Namespace, controllerRef)
		if img == nil {
			return
		}
		klog.V(4).Infof("ImageSet %s added.", iSet.Name)
		ai.enqueueAdvancedImage(img)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching AdvancedImages and sync
	// them to see if anyone wants to adopt it.
	ais := ai.getAdvancedImageForImageSet(iSet)
	if len(ais) == 0 {
		return
	}

	klog.V(4).Infof("Orphan ImageSet %s added.", iSet.Name)
	for _, i := range ais {
		ai.enqueueAdvancedImage(i)
	}
}

// updateImageSet figures out what advancedImage(s) manage a imageSet when the imageSet
// is updated and wake them up. If the anything of the imageSet have changed, we need to
// awaken both the old and new advancedImage. old and cur must be *appsv1alpha1.ImageSet
// types.
func (ai *AdvancedImageController) updateImageSet(old, cur interface{}) {
	oldiSet := old.(*appsv1alpha1.ImageSet)
	curiSet := cur.(*appsv1alpha1.ImageSet)
	if oldiSet.ResourceVersion == curiSet.ResourceVersion {
		// Periodic resync will send update events for all known image sets.
		// Two different versions of the same image set will always have different RVs.
		return
	}

	oldControllerRef := metav1.GetControllerOf(oldiSet)
	curControllerRef := metav1.GetControllerOf(curiSet)
	isControllerRefChanged := !reflect.DeepEqual(oldControllerRef, curControllerRef)
	if isControllerRefChanged && oldControllerRef != nil {
		if a := ai.resolveControllerRef(oldiSet.Namespace, oldControllerRef); a != nil {
			ai.enqueueAdvancedImage(a)
		}
	}

	if curControllerRef != nil {
		if a := ai.resolveControllerRef(curiSet.Namespace, curControllerRef); a != nil {
			klog.V(4).Infof("ImageSet %s updated.", curiSet.Name)
			ai.enqueueAdvancedImage(a)
			return
		}
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	labelChanged := !reflect.DeepEqual(curiSet.Labels, curiSet.Labels)
	if labelChanged || isControllerRefChanged {
		as := ai.getAdvancedImageForImageSet(curiSet)
		if len(as) == 0 {
			return
		}
		klog.V(4).Infof("Orphan ImageSet %s updated.", curiSet.Name)
		for _, a := range as {
			ai.enqueueAdvancedImage(a)
		}
	}
}

func (ai *AdvancedImageController) deleteImageSet(obj interface{}) {
	iSet, ok := obj.(*appsv1alpha1.ImageSet)

	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		iSet, ok = tombstone.Obj.(*appsv1alpha1.ImageSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a ImageSet %#v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(iSet)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}

	a := ai.resolveControllerRef(iSet.Namespace, controllerRef)
	if a == nil {
		return
	}
	klog.V(4).Infof("ImageSet %s deleted.", iSet.Name)
	ai.enqueueAdvancedImage(a)
}

func (ai *AdvancedImageController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ai.queue.ShutDown()

	klog.Infof("Starting AdvancedImage Controller")
	defer klog.Infof("Shutting down AdvancedImage Controller")

	if !cache.WaitForNamedCacheSync("advancedImage-controller", stopCh, ai.imgListerSynced, ai.iSetListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ai.worker, time.Second, stopCh)
	}

	<-stopCh
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct King
func (ai *AdvancedImageController) resolveControllerRef(nameSpace string, controllerRef *metav1.OwnerReference) *appsv1alpha1.AdvancedImage {
	if controllerKind.Kind != controllerRef.Kind {
		return nil
	}
	img, err := ai.imgLister.AdvancedImages(nameSpace).Get(controllerRef.Name)
	if err != nil {
		return nil
	}
	if img.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}

	return img
}

// getAdvancedImageForImageSet returns a list of AdvancedImage that potentially
// match a ImageSet. Only the one specified in the ImageSet's ControllerRef
// will actually manage it.
// Returns nil only if no matching AdvancedImage are found.
func (ai *AdvancedImageController) getAdvancedImageForImageSet(iSet *appsv1alpha1.ImageSet) []*appsv1alpha1.AdvancedImage {
	if len(iSet.Labels) == 0 {
		// no advancedImage found for ImageSet %v because it has no labels
		return nil
	}

	imageList, err := ai.imgLister.AdvancedImages(iSet.Namespace).List(labels.Everything())
	if err != nil || len(imageList) == 0 {
		return nil
	}

	var advancedImages []*appsv1alpha1.AdvancedImage
	for _, img := range imageList {
		selector, err := metav1.LabelSelectorAsSelector(img.Spec.Selector)
		if err != nil {
			return nil
		}
		// If a advancedImage with a nil or emtpy selector creeps in, it should match nothing, not everything.
		if selector.Empty() || !selector.Matches(labels.Set(img.Labels)) {
			continue
		}
		advancedImages = append(advancedImages, img)
	}

	if len(advancedImages) == 0 {
		return nil
	}

	if len(advancedImages) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		klog.V(4).Infof("user error! more than one advancedImage is selecting image set %s/%s with labels: %#v, returning %s/%s",
			iSet.Namespace, iSet.Name, iSet.Labels, advancedImages[0].Namespace, advancedImages[0].Name)
	}

	return advancedImages
}

func (ai *AdvancedImageController) enqueue(img *appsv1alpha1.AdvancedImage) {
	key, err := controller.KeyFunc(img)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", img, err))
		return
	}

	ai.queue.Add(key)
}

func (ai *AdvancedImageController) getImageSetForAdvancedImage(i *appsv1alpha1.AdvancedImage) ([]*appsv1alpha1.ImageSet, error) {
	selector, err := metav1.LabelSelectorAsSelector(i.Spec.Selector)
	if err != nil {
		return nil, err
	}
	imageSets, err := ai.iSetLister.ImageSets(i.Namespace).List(selector)
	if err != nil {
		return nil, err
	}

	var requiredISets []*appsv1alpha1.ImageSet
	for _, iSet := range imageSets {
		controllerRef := metav1.GetControllerOf(iSet)
		if controllerRef == nil {
			continue
		}
		if controllerRef.UID != i.UID {
			continue
		}
		requiredISets = append(requiredISets, iSet)
	}

	return requiredISets, nil
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

	if img.DeletionTimestamp != nil {
		klog.V(4).Infof("Advanced Deployment %v is deleting", key)
		// TODO：just to update the status if needed
		return nil
	}

	// Deep copy otherwise we are mutating our indexer cache.
	m := img.DeepCopy()

	everything := metav1.LabelSelector{}
	if reflect.DeepEqual(m.Spec.Selector, &everything) {
		ai.eventRecorder.Eventf(m, v1.EventTypeWarning, "SelectingAll", "This advanced image is selecting all imageSet. A non-empty selector is required.")
		if m.Status.ObservedGeneration < m.Generation {
			m.Status.ObservedGeneration = m.Generation
			_, _ = ai.pxClient.AppsV1alpha1().AdvancedImages(m.Namespace).UpdateStatus(context.TODO(), m, metav1.UpdateOptions{})
		}

		return nil
	}

	// List imageSets owned by the advancedImage, while reconciling ControllerRef
	// through adoption/orphaning.
	imageSets, err := ai.getImageSetForAdvancedImage(m)
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
