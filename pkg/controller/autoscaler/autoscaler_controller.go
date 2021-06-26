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

package autoscaler

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/caoyingjunz/kubez-autoscaler/pkg/controller"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	autoscalinginformers "k8s.io/client-go/informers/autoscaling/v2beta2"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	autoscalinglisters "k8s.io/client-go/listers/autoscaling/v2beta2"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
)

const (
	maxRetries = 15

	AddEvent           string = "Add"
	UpdateEvent        string = "Update"
	DeleteEvent        string = "Delete"
	RecoverDeleteEvent string = "RecoverDelete"
	RecoverUpdateEvent string = "RecoverUpdate"

	markInnerEvent string = "markInnerEvent"
)

const (
	appsAPIVersion string = "apps/v1"

	Deployment  string = "Deployment"
	StatefulSet string = "StatefulSet"
)

// AutoscalerController is responsible for synchronizing HPA objects stored
// in the system.
type AutoscalerController struct {
	client        clientset.Interface
	eventRecorder record.EventRecorder

	syncHandler       func(hpaKey string) error
	enqueueAutoscaler func(hpa *autoscalingv2.HorizontalPodAutoscaler)

	// dLister can list/get deployments from the shared informer's store
	dLister appslisters.DeploymentLister
	// sLister can list/get statefulset from the shared informer's store
	sLister appslisters.StatefulSetLister
	// hpaLister is able to list/get HPAs from the shared informer's cache
	hpaLister autoscalinglisters.HorizontalPodAutoscalerLister

	// dListerSynced returns true if the Deployment store has been synced at least once.
	dListerSynced cache.InformerSynced
	// sListerSynced returns true if the StatefulSet store has been synced at least once.
	sListerSynced cache.InformerSynced
	// hpaListerSynced returns true if the HPA store has been synced at least once.
	hpaListerSynced cache.InformerSynced

	// AutoscalerController that need to be synced
	queue workqueue.RateLimitingInterface
	// Safe Store than to store the obj
	store SafeStoreInterface
}

// NewAutoscalerController creates a new AutoscalerController.
func NewAutoscalerController(
	dInformer appsinformers.DeploymentInformer,
	sInformer appsinformers.StatefulSetInformer,
	hpaInformer autoscalinginformers.HorizontalPodAutoscalerInformer,
	client clientset.Interface) (*AutoscalerController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("autoscaler_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	ac := &AutoscalerController{
		client:        client,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "autoscaler-controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "autoscaler"),
		store:         newSafeStore(),
	}

	// Deployment
	dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ac.addEvents,
		UpdateFunc: ac.updateEvents,
		DeleteFunc: ac.deleteEvents,
	})

	// StatefulSet
	sInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ac.addEvents,
		UpdateFunc: ac.updateEvents,
		DeleteFunc: ac.deleteEvents,
	})

	// HorizontalPodAutoscaler
	hpaInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ac.addHPA,
		UpdateFunc: ac.updateHPA,
		DeleteFunc: ac.deleteHPA,
	})

	ac.dLister = dInformer.Lister()
	ac.sLister = sInformer.Lister()
	ac.hpaLister = hpaInformer.Lister()

	// syncAutoscalers
	ac.syncHandler = ac.syncAutoscalers
	ac.enqueueAutoscaler = ac.enqueue

	ac.dListerSynced = dInformer.Informer().HasSynced
	ac.sListerSynced = sInformer.Informer().HasSynced
	ac.hpaListerSynced = hpaInformer.Informer().HasSynced

	return ac, nil
}

// Run begins watching and syncing.
func (ac *AutoscalerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ac.queue.ShutDown()

	klog.Infof("Starting Autoscaler Controller")
	defer klog.Infof("Shutting down Autoscaler Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("kubez-autoscaler-manager", stopCh, ac.dListerSynced, ac.sListerSynced, ac.hpaListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ac.worker, time.Second, stopCh)
	}

	<-stopCh
}

// syncAutoscaler will sync the autoscaler with the given key.
func (ac *AutoscalerController) syncAutoscalers(key string) error {
	starTime := time.Now()
	klog.V(2).Infof("Start syncing autoscaler %q (%v)", key, starTime)
	defer func() {
		klog.V(2).Infof("Finished syncing autoscaler %q (%v)", key, time.Since(starTime))
	}()

	// Delete the obj from store even though the syncAutoscalers failed
	defer ac.store.Delete(key)

	hpa, exists := ac.store.Get(key)
	if !exists {
		// Do nothing and return directly
		return nil
	}

	kind := hpa.Spec.ScaleTargetRef.Kind

	var err error
	event := ac.popInnerEvent(hpa)
	klog.V(0).Infof("Handlering %s event for %s/%s from %s", event, hpa.Namespace, hpa.Name, kind)

	switch event {
	case AddEvent:
		_, err = ac.client.AutoscalingV2beta2().
			HorizontalPodAutoscalers(hpa.Namespace).Create(context.TODO(), hpa, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("Failed to create HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedCreateHPA", msg)
			return err
		}
		ac.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "CreateHPA",
			fmt.Sprintf("Create HPA %s/%s for %s success", hpa.Namespace, hpa.Name, kind))
	case UpdateEvent:
		_, err = ac.client.AutoscalingV2beta2().
			HorizontalPodAutoscalers(hpa.Namespace).Update(context.TODO(), hpa, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to update HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedUpdateHPA", msg)
			return err
		}
		ac.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "UpdateHPA",
			fmt.Sprintf("Update HPA %s/%s for %s success", hpa.Namespace, hpa.Name, kind))
	case DeleteEvent:
		err = ac.client.AutoscalingV2beta2().
			HorizontalPodAutoscalers(hpa.Namespace).Delete(context.TODO(), hpa.Name, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to delete HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedDeleteHPA", msg)
			return err
		}
		ac.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "DeleteHPA",
			fmt.Sprintf("Delete HPA %s/%s for %s success", hpa.Namespace, hpa.Name, kind))
	case RecoverUpdateEvent:
		// Since the HPA has been updated, we need to get origin spec to check whether it shouled be recover
		newHPA, err := ac.GetNewestHPAFromResource(hpa)
		if err != nil {
			msg := fmt.Sprintf("Failed to get update newest HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedNewestHPA", msg)
			return err
		}
		// no need to Recover HPA from Update event
		if newHPA == nil {
			return nil
		}
		if reflect.DeepEqual(hpa.Spec, newHPA.Spec) {
			klog.V(2).Infof("HPA: %s/%s spec is not changed, no need to updated", hpa.Namespace, hpa.Name)
			return nil
		}
		_, err = ac.client.AutoscalingV2beta2().
			HorizontalPodAutoscalers(newHPA.Namespace).Update(context.TODO(), newHPA, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			msg := fmt.Sprintf("Failed to Recover update HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedRecoverHPA", msg)
			return err
		}
		ac.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "RecoverHPA",
			fmt.Sprintf("Recover Update HPA %s/%s for %s success", hpa.Namespace, hpa.Name, kind))
	case RecoverDeleteEvent:
		newHPA, err := ac.GetNewestHPAFromResource(hpa)
		if err != nil {
			msg := fmt.Sprintf("Failed to get delete newest HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedNewestHPA", msg)
			return err
		}
		// no need to Recover HPA from Delete event
		if newHPA == nil {
			return nil
		}
		_, err = ac.client.AutoscalingV2beta2().
			HorizontalPodAutoscalers(newHPA.Namespace).Create(context.TODO(), newHPA, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("Failed to get delete newest HPA %s/%s for %s", hpa.Namespace, hpa.Name, kind)
			ac.eventRecorder.Eventf(hpa, v1.EventTypeWarning, "FailedCreateHPA", msg)
			return err
		}
		ac.eventRecorder.Eventf(hpa, v1.EventTypeNormal, "RecoverHPA",
			fmt.Sprintf("Recover Delete HPA %s/%s for %s success", hpa.Namespace, hpa.Name, kind))
	default:
		return fmt.Errorf("Unsupported handlers event %s", event)
	}

	return err
}

// GetNewestHPA will get newest HPA from kubernetes resources
func (ac *AutoscalerController) GetNewestHPAFromResource(
	hpa *autoscalingv2.HorizontalPodAutoscaler) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	var annotations map[string]string
	var uid types.UID
	kind := hpa.Spec.ScaleTargetRef.Kind
	switch kind {
	case Deployment:
		d, err := ac.dLister.Deployments(hpa.Namespace).Get(hpa.Name)
		if err != nil {
			return nil, err
		}
		// check
		if !controller.IsOwnerReference(d.UID, hpa.OwnerReferences) {
			return nil, nil
		}
		uid = d.UID
		annotations = d.Annotations
	case StatefulSet:
		s, err := ac.sLister.StatefulSets(hpa.Namespace).Get(hpa.Name)
		if err != nil {
			return nil, err
		}
		if !controller.IsOwnerReference(s.UID, hpa.OwnerReferences) {
			return nil, nil
		}
		uid = s.UID
		annotations = s.Annotations
	}
	if !controller.IsNeedForHPAs(annotations) {
		return nil, nil
	}

	return controller.CreateHorizontalPodAutoscaler(
		hpa.Name, hpa.Namespace, uid, appsAPIVersion, kind, annotations)
}

// To insert annotation to distinguish the event type
func (ac *AutoscalerController) wrapInnerEvent(hpa *autoscalingv2.HorizontalPodAutoscaler, event string) {
	// there is no necessary to lock the Annotations
	if hpa.Annotations == nil {
		hpa.Annotations = map[string]string{
			markInnerEvent: event,
		}
		return
	}
	hpa.Annotations[markInnerEvent] = event
}

// To pop kubez annotation and clean up kubez marker from HPA
func (ac *AutoscalerController) popInnerEvent(hpa *autoscalingv2.HorizontalPodAutoscaler) string {
	event, exists := hpa.Annotations[markInnerEvent]
	// This shouldn't happen, because we only insert annotation for hpa
	if exists {
		delete(hpa.Annotations, markInnerEvent)
	}
	return event
}

func (ac *AutoscalerController) addEvents(obj interface{}) {
	ascCtx := controller.NewAutoscalerContext(obj)
	klog.V(2).Infof("Adding %s %s/%s", ascCtx.Kind, ascCtx.Namespace, ascCtx.Name)
	if !controller.IsNeedForHPAs(ascCtx.Annotations) {
		return
	}

	hpa, err := controller.CreateHorizontalPodAutoscaler(
		ascCtx.Name, ascCtx.Namespace, ascCtx.UID, appsAPIVersion, ascCtx.Kind, ascCtx.Annotations)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	if hpa == nil {
		return
	}

	key, err := controller.KeyFunc(hpa)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", hpa, err))
		return
	}
	ac.wrapInnerEvent(hpa, AddEvent)
	ac.store.Update(key, hpa)

	ac.enqueueAutoscaler(hpa)
}

func (ac *AutoscalerController) updateEvents(old, cur interface{}) {
	oldCtx := controller.NewAutoscalerContext(old)
	curCtx := controller.NewAutoscalerContext(cur)
	klog.V(2).Infof("Updating %s %s/%s", oldCtx.Kind, oldCtx.Namespace, oldCtx.Name)

	if reflect.DeepEqual(oldCtx.Annotations, curCtx.Annotations) {
		return
	}
	oldExists := controller.IsNeedForHPAs(oldCtx.Annotations)
	curExists := controller.IsNeedForHPAs(curCtx.Annotations)

	// Delete HPAs
	if oldExists && !curExists {
		hpa, err := ac.hpaLister.HorizontalPodAutoscalers(oldCtx.Namespace).Get(oldCtx.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				return
			}
			utilruntime.HandleError(err)
			return
		}

		key, err := controller.KeyFunc(hpa)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", hpa, err))
			return
		}
		ac.wrapInnerEvent(hpa, DeleteEvent)
		ac.store.Add(key, hpa)

		ac.enqueueAutoscaler(hpa)
		return
	}

	// Add or Update HPAs
	hpa, err := controller.CreateHorizontalPodAutoscaler(
		curCtx.Name, curCtx.Namespace, curCtx.UID, appsAPIVersion, curCtx.Kind, curCtx.Annotations)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	key, err := controller.KeyFunc(hpa)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", hpa, err))
		return
	}

	if !oldExists && curExists {
		ac.wrapInnerEvent(hpa, AddEvent)
	} else if oldExists && curExists {
		ac.wrapInnerEvent(hpa, UpdateEvent)
	}
	ac.store.Add(key, hpa)

	ac.enqueueAutoscaler(hpa)
}

func (ac *AutoscalerController) deleteEvents(obj interface{}) {
	ascCtx := controller.NewAutoscalerContext(obj)
	klog.V(2).Infof("Deleting %s %s/%s", ascCtx.Kind, ascCtx.Namespace, ascCtx.Name)

	hpa, err := ac.hpaLister.HorizontalPodAutoscalers(ascCtx.Namespace).Get(ascCtx.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			// HPA has been deleted
			return
		}
		utilruntime.HandleError(err)
		return
	}
	if !controller.IsOwnerReference(ascCtx.UID, hpa.OwnerReferences) {
		return
	}

	key, err := controller.KeyFunc(hpa)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", hpa, err))
		return
	}
	ac.wrapInnerEvent(hpa, DeleteEvent)
	ac.store.Update(key, hpa)

	ac.enqueueAutoscaler(hpa)
}

func (ac *AutoscalerController) addHPA(obj interface{}) {
	h := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !controller.ManagerByKubezController(h) {
		return
	}
	klog.V(0).Infof("Adding HPA(manager by kubez) %s/%s", h.Namespace, h.Name)
}

// updateHPA figures out what HPA(s) is updated and wake them up.
// old and cur must be *autoscalingv2.HorizontalPodAutoscaler types.
func (ac *AutoscalerController) updateHPA(old, cur interface{}) {
	oldH := old.(*autoscalingv2.HorizontalPodAutoscaler)
	curH := cur.(*autoscalingv2.HorizontalPodAutoscaler)
	if oldH.ResourceVersion == curH.ResourceVersion {
		// Periodic resync will send update events for all known HPAs.
		// Two different versions of the same HPA will always have different ResourceVersions.
		return
	}
	if !controller.ManagerByKubezController(oldH) {
		return
	}
	klog.V(0).Infof("Updating HPA %s/%s", oldH.Namespace, oldH.Name)

	key, err := controller.KeyFunc(curH)
	if err != nil {
		return
	}
	// To insert annotation to distinguish the event type is Update
	ac.wrapInnerEvent(curH, RecoverUpdateEvent)
	ac.store.Update(key, curH)

	ac.enqueueAutoscaler(curH)
}

func (ac *AutoscalerController) deleteHPA(obj interface{}) {
	h := obj.(*autoscalingv2.HorizontalPodAutoscaler)
	if !controller.ManagerByKubezController(h) {
		return
	}
	klog.V(0).Infof("Deleting HPA %s/%s", h.Namespace, h.Name)

	key, err := controller.KeyFunc(h)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", h, err))
		return
	}
	ac.wrapInnerEvent(h, RecoverDeleteEvent)
	ac.store.Update(key, h)

	ac.enqueueAutoscaler(h)
}

func (ac *AutoscalerController) enqueue(hpa *autoscalingv2.HorizontalPodAutoscaler) {
	key, err := controller.KeyFunc(hpa)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", hpa, err))
		return
	}

	ac.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes then, and marks them done.
func (ac *AutoscalerController) worker() {
	for ac.processNextWorkItem() {
	}
}

func (ac *AutoscalerController) processNextWorkItem() bool {
	key, quit := ac.queue.Get()
	if quit {
		return false
	}
	defer ac.queue.Done(key)

	err := ac.syncHandler(key.(string))
	ac.handleErr(err, key)
	return true
}

func (ac *AutoscalerController) handleErr(err error, key interface{}) {
	if err == nil {
		ac.queue.Forget(key)
		return
	}

	if ac.queue.NumRequeues(key) < maxRetries {
		klog.V(0).Infof("Error syncing HPA %v: %v", key, err)
		ac.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(0).Infof("Dropping HPA %q out of the queue: %v", key, err)
	ac.queue.Forget(key)
}
