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

package pixiu

import (
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
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
	"time"
)

const maxRetries = 15

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = apps.SchemeGroupVersion.WithKind("Pixiu")

// PixiuController is responsible for synchronizing pixiu objects stored
// in the system.
type PixiuController struct {
	client        clientset.Interface
	eventRecorder record.EventRecorder

	syncHandler  func(pKey string) error
	enqueuePixiu func(pixiu *apps.Deployment)

	// podLister can list/get pods from the shared informer's store
	podLister corelisters.PodLister

	// podListerSynced returns true if the pod store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	podListerSynced cache.InformerSynced

	// PixiuController that need to be synced
	queue workqueue.RateLimitingInterface
}

func NewPixiuController(podInformer coreinformers.PodInformer, client clientset.Interface) (*PixiuController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("deployment_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	pc := &PixiuController{
		client:        client,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "pixiu-controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pixiu"),
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: pc.deletePod,
	})

	pc.podListerSynced = podInformer.Informer().HasSynced

	return pc, nil
}

func (pc *PixiuController) deletePod(obj interface{}) {
	pod := obj.(*v1.Pod)
	klog.V(4).Infof("Pod %s deleted.", pod.Name)
}

// Run begins watching and syncing.
func (pc *PixiuController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	klog.Infof("Starting Pixiu Controller")
	defer klog.Infof("Shutting down Pixiu Controller")

	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForNamedCacheSync("pixiu-controller", stopCh, pc.podListerSynced) {
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
