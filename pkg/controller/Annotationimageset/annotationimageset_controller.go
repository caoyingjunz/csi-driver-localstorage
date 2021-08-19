package Annotationimageset

import (
	"context"
	"fmt"
	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
	isClientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	pInformers "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions/apps/v1alpha1"
	isListers "github.com/caoyingjunz/pixiu/pkg/client/listers/apps/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog/v2"
	"time"
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

type AnnotationImageSetController struct {
	client        clientset.Interface
	isClient 	  isClientset.Interface

	eventRecorder record.EventRecorder

	enqueueAnnotationimageset func(img *appsv1alpha1.ImageSet)
	syncHandler     func(iKey string) error

	dLister appslisters.DeploymentLister
	sLister appslisters.StatefulSetLister
	isLister	isListers.ImageSetLister

	dListerSynced cache.InformerSynced
	sListerSynced cache.InformerSynced
	isListerSynced cache.InformerSynced

	// ImageSet that need to be synced
	queue workqueue.RateLimitingInterface

	store SafeStoreInterface
}

const (
	Deployment  string = "Deployment"
	StatefulSet string = "StatefulSet"
)

// NewAnnotationimagesetController creates a new AnnotationimagesetController.
func NewAnnotationimagesetController(
	dInformer appsinformers.DeploymentInformer,
	sInformer appsinformers.StatefulSetInformer,
	isInformer pInformers.ImageSetInformer,
	client clientset.Interface,
	isClient isClientset.Interface) (*AnnotationImageSetController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("Annotationimageset_Controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	ais := &AnnotationImageSetController{
		client: client,
		isClient: isClient,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "Annotationimageset-Controller"}),
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),"Annotationimageset"),
	}

	dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ais.addEvents,
	})
	sInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ais.addEvents,
	})
	isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    ais.addEvents,
	})

	ais.dLister = dInformer.Lister()
	ais.dListerSynced = dInformer.Informer().HasSynced

	ais.sLister = sInformer.Lister()
	ais.sListerSynced = sInformer.Informer().HasSynced

	ais.isLister = isInformer.Lister()
	ais.dListerSynced = dInformer.Informer().HasSynced

	ais.syncHandler = ais.syncAnnotationimageset
	ais.enqueueAnnotationimageset = ais.enqueue

	return ais, nil
}

// Run begins watching and syncing.
func (ais *AnnotationImageSetController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ais.queue.ShutDown()

	klog.Infof("Starting AnnotationImageSet Controller")
	defer klog.Infof("Shutting Down AnnotationImageSet Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("AnnotationImageSet-Manager", stopCh, ais.dListerSynced, ais.sListerSynced, ais.isListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(ais.worker, time.Second, stopCh)
	}

	<-stopCh
}

// syncAutoscaler will sync the autoscaler with the given key.
func (ais *AnnotationImageSetController) syncAnnotationimageset(key string) error {
	starTime := time.Now()
	klog.V(2).Infof("Start syncing Annotationimageset %q (%v)", key, starTime)
	defer func() {
		klog.V(2).Infof("Finished syncing Annotationimageset %q (%v)", key, time.Since(starTime))
	}()

	// Delete the obj from store even though the syncAutoscalers failed
	defer ais.store.Delete(key)

	img, exists := ais.store.Get(key)
	if !exists {
		// Do nothing and return directly
		return nil
	}

	kind := img.Kind

	var err error
	event := ais.popInnerEvent(img)
	klog.V(0).Infof("Handlering %s event for %s/%s from %s", event, img.Namespace, img.Name, kind)

	switch event {
	case AddEvent:
		_, err = ais.isClient.AppsV1alpha1().ImageSets(img.Namespace).Create(context.TODO(),img,metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			msg := fmt.Sprintf("Failed to create IMG %s/%s for %s", img.Namespace, img.Name, kind)
			ais.eventRecorder.Eventf(img, v1.EventTypeWarning, "FailedCreateImageSet", msg)
			return err
		}
		ais.eventRecorder.Eventf(img, v1.EventTypeNormal, "CreateImageSet",
			fmt.Sprintf("Create IS %s/%s for %s success", img.Namespace, img.Name, kind))
	default:
		return fmt.Errorf("Unsupported handlers event %s", event)
	}
	return err
}

func (ais *AnnotationImageSetController) wrapInnerEvent(img *appsv1alpha1.ImageSet, event string) {
	// there is no necessary to lock the Annotations
	if img.Annotations == nil {
		img.Annotations = map[string]string{
			markInnerEvent: event,
		}
		return
	}
	img.Annotations[markInnerEvent] = event
}

// To pop kubez annotation and clean up kubez marker from HPA
func (ais *AnnotationImageSetController) popInnerEvent(img *appsv1alpha1.ImageSet) string {
	event, exists := img.Annotations[markInnerEvent]
	// This shouldn't happen, because we only insert annotation for hpa
	if exists {
		delete(img.Annotations, markInnerEvent)
	}
	return event
}

func (ais *AnnotationImageSetController) enqueue(img *appsv1alpha1.ImageSet) {
	key, err := KeyFunc(img)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", img, err))
		return
	}
	ais.queue.Add(key)
}

func (ais *AnnotationImageSetController) worker() {
	for ais.processNextWorkItem() {
	}
}

func (ais *AnnotationImageSetController) processNextWorkItem() bool {
	key, quit := ais.queue.Get()
	if quit {
		return false
	}
	defer ais.queue.Done(key)

	err := ais.syncHandler(key.(string))
	ais.handleErr(err, key)
	return true
}

func (ac *AnnotationImageSetController) handleErr(err error, key interface{}) {
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


func (ais *AnnotationImageSetController) addEvents(obj interface{}) {
	imgCtx := NewAnnotationImageSetContext(obj)
	klog.V(2).Infof("Adding %s %s/%s", imgCtx.Kind, imgCtx.Namespace, imgCtx.Name,imgCtx.Image)
	if !IsNeedForIMGs(imgCtx.Annotations) {
		return
	}

	img, err := CreateImageSet(
		imgCtx.Name, imgCtx.Namespace, imgCtx.Annotations, imgCtx.Image)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	if img == nil {
		return
	}

	key, err := KeyFunc(img)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", img, err))
		return
	}
	ais.wrapInnerEvent(img, AddEvent)
	ais.store.Update(key, img)

	ais.enqueueAnnotationimageset(img)
}