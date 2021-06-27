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

	dockertypes "github.com/docker/docker/api/types"
	kc "github.com/ericchiang/k8s"
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
	"github.com/caoyingjunz/pixiu/pkg/controller/libdocker"
)

const (
	maxRetries     = 15
	DefaultLockTTL = time.Second * 5
	// images action, only should pull and remove action, is equal to docker command
	PullAction   = "pull"
	RemoveAction = "remove"
)

var controllerKind = v1.SchemeGroupVersion.WithKind("ImageSet")

// ImageSetController is responsible for synchronizing images objects stored
// in the system.
type ImageSetController struct {
	client        clientset.Interface
	eventRecorder record.EventRecorder
	hostName      string

	dc       libdocker.Interface
	isClient isClientset.Interface

	syncHandler     func(iKey string) error
	enqueueImageSet func(imageSet *appsv1alpha1.ImageSet)

	isLister       isListers.ImageSetLister
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
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("imageset_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	isc := &ImageSetController{
		client:        client,
		isClient:      isClient,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "imageset-controller"}),
		hostName:      hostName,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "imageset"),
	}

	isc.dc = libdocker.ConnectToDockerOrDie(dockerHost, time.Duration(60)*time.Second, 1800)

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
		klog.V(2).Infof("Error syncing image for imageset %q, retrying. Error: %v", key, err)
		isc.queue.AddRateLimited(key)
		return
	}

	klog.Warningf("Dropping imageset %q out of the queue: %v", key, err)
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

	ims, err := isc.isLister.ImageSets(namespace).Get(name)
	if errors.IsNotFound(err) {
		// kubernetes will use finalizers to handler the delete event
		klog.V(4).Infof("Image Sets %v has been deleted", key)
		return nil
	}
	if err != nil {
		klog.Errorf("Get imageSet from index failed: %v", err)
		return err
	}

	var imageRef string
	image := ims.Spec.Image

	switch ims.Spec.Action {
	case PullAction:
		auth := ims.Spec.Auth
		authConfig := dockertypes.AuthConfig{}
		if auth != nil {
			authConfig.Username = auth.Username
			authConfig.Password = auth.Password
			authConfig.ServerAddress = auth.ServerAddress
			authConfig.IdentityToken = auth.IdentityToken
			authConfig.RegistryToken = auth.RegistryToken
		}
		// TODO, add event supported
		if isc.isSelectNode(ims) {
			imageRef, err = isc.dc.PullImage(image, authConfig, dockertypes.ImagePullOptions{})
		}
	case RemoveAction:
		_, err = isc.dc.RemoveImage(image, dockertypes.ImageRemoveOptions{})
	default:
		return fmt.Errorf("unsupported imageset action: %s", ims.Spec.Action)
	}
	if err != nil {
		klog.Errorf("%s imageset: %s failed: %v", ims.Spec.Action, image, err)
		return err
	}

	c, err := kc.NewInClusterClient()
	if err != nil {
		klog.Errorf("Cannot create k8s client: %#v\n", err)
	}

	var l PixiuLock
	l, err = NewDaemonSetLock(ims.Namespace, ims.Name, c, "", "", DefaultLockTTL)
	if err != nil {
		klog.Errorf("Cannot create new daemonlock: %#v\n", err)
	}
	for i := 1; i <= 5; i++ {
		err := l.Acquire()
		if err != nil {
			klog.Errorf("Cannot acquire lock: %v\n", err)
			time.Sleep(time.Second * 1)
			continue
		}
		klog.Infof("Lock acquired")
		newStatus := calculateImageSetStatus(isc.isClient.AppsV1alpha1().ImageSets(ims.Namespace), ims.Name, isc.hostName, imageRef, err)
		ims = ims.DeepCopy()
		// Always try to update as sync come up or failed.
		_, err = updateImageSetStatus(isc.isClient.AppsV1alpha1().ImageSets(ims.Namespace), ims, newStatus)
		if err != nil {
			klog.Errorf("update %s imageset: %s  status failed: %v", ims.Spec.Action, image, err)
			return err
		}
		klog.Infof("Imageset: %s has been %s success", image, ims.Spec.Action)
		break
	}
	defer func() {
		err = l.Release()
		if err != nil {
			klog.Errorf("Failed to release lock: %#v\n", err)
		} else {
			klog.Infof("Lock release")
		}
	}()
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

	klog.V(2).Infof("Adding ImageSet %s/%s", is.Namespace, is.Name)
	isc.enqueueImageSet(is)
}

func (isc *ImageSetController) updateImageSet(old, cur interface{}) {
	oldImageSet := old.(*appsv1alpha1.ImageSet)
	curImageSet := cur.(*appsv1alpha1.ImageSet)
	if oldImageSet.ResourceVersion == curImageSet.ResourceVersion {
		return
	}
	// Just update the status
	if oldImageSet.Generation == curImageSet.Generation {
		return
	}

	klog.V(2).Infof("Updating ImageSet %s/%s", curImageSet.Namespace, curImageSet.Name)
	isc.enqueueImageSet(curImageSet)
}

func (isc *ImageSetController) deleteImageSet(obj interface{}) {
	is := obj.(*appsv1alpha1.ImageSet)
	klog.V(2).Infof("deleting ImageSet %s/%s", is.Namespace, is.Name)
}

func (isc *ImageSetController) isSelectNode (ims *appsv1alpha1.ImageSet)  bool {
	result := false
	nodes :=ims.Spec.Selector.Nodes
	for _, node := range nodes {
		if isc.hostName == node {
			result = true
			break
		}
	}
	return  result
}
