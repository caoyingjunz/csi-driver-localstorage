/*
Copyright 2021 The Caoyingjunz Authors.

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

package storage

import (
	"context"
	"fmt"
	"time"

	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
	storageutil "github.com/caoyingjunz/csi-driver-localstorage/pkg/util/storage"
)

const (
	maxRetries = 15
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type StorageController struct {
	client     versioned.Interface
	kubeClient kubernetes.Interface

	eventBroadcaster record.EventBroadcaster
	eventRecorder    record.EventRecorder

	syncHandler         func(ctx context.Context, dKey string) error
	enqueueLocalstorage func(ls *localstoragev1.LocalStorage)

	// lsLister can list/get localstorage from the shared informer's store
	lsLister localstorage.LocalStorageLister

	// lsListerSynced returns true if the localstorage store has been synced at least once.
	lsListerSynced cache.InformerSynced

	// localstorage that need to be synced
	queue workqueue.RateLimitingInterface
}

// NewStorageController creates a new StorageController.
func NewStorageController(ctx context.Context, lsInformer v1.LocalStorageInformer, lsClientSet versioned.Interface, kubeClientSet kubernetes.Interface) (*StorageController, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedv1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})

	sc := &StorageController{
		client:           lsClientSet,
		kubeClient:       kubeClientSet,
		eventBroadcaster: eventBroadcaster,
		eventRecorder:    eventBroadcaster.NewRecorder(scheme.Scheme, v1core.EventSource{Component: util.LocalstorageManagerUserAgent}),
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "localstorage"),
	}

	lsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sc.addStorage(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			sc.updateStorage(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			sc.deleteStorage(obj)
		},
	})

	sc.syncHandler = sc.syncStorage
	sc.enqueueLocalstorage = sc.enqueue

	sc.lsLister = lsInformer.Lister()
	sc.lsListerSynced = lsInformer.Informer().HasSynced
	return sc, nil
}

func (s *StorageController) addStorage(obj interface{}) {
	ls, ok := obj.(*localstoragev1.LocalStorage)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("expected localstorage in addStorage, but got %#v", obj))
		return
	}
	klog.V(2).Info("Adding localstorage", klog.KObj(ls))
	s.enqueueLocalstorage(ls)
}

func (s *StorageController) updateStorage(old, cur interface{}) {
	oldLs := old.(*localstoragev1.LocalStorage)
	curLs := cur.(*localstoragev1.LocalStorage)
	klog.V(2).Info("Updating localstorage", klog.KObj(oldLs))

	s.enqueueLocalstorage(curLs)
}

func (s *StorageController) deleteStorage(obj interface{}) {
	ls, ok := obj.(*localstoragev1.LocalStorage)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		ls, ok = tombstone.Obj.(*localstoragev1.LocalStorage)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a localstorage %#v", obj))
			return
		}
	}
	klog.V(2).Info("Deleting localstorage", "localstorage", klog.KObj(ls))
	s.enqueueLocalstorage(ls)
}

func (s *StorageController) syncStorage(ctx context.Context, dKey string) error {
	startTime := time.Now()
	klog.V(2).InfoS("Started syncing localstorage manager", "startTime", startTime)
	defer func() {
		klog.V(2).InfoS("Finished syncing localstorage manager", "duration", time.Since(startTime))
	}()

	localstorage, err := s.lsLister.Get(dKey)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).Infof("localstorage has been deleted", dKey)
			return nil
		}
		return err
	}
	// Deep copy otherwise we are mutating the cache.
	ls := localstorage.DeepCopy()

	// Handler deletion event
	if !ls.DeletionTimestamp.IsZero() {
		if !util.LocalStorageIsTerminating(ls) {
			util.SetLocalStoragePhase(ls, localstoragev1.LocalStorageTerminating)
			return storageutil.TryUpdateLocalStorage(s.client, ls)
		}
		return nil
	}

	// Handler initialize Phase
	if util.LocalStorageIsPending(ls) {
		util.SetLocalStoragePhase(ls, localstoragev1.LocalStorageInitiating)
		if err = storageutil.TryUpdateLocalStorage(s.client, ls); err != nil {
			return err
		}
		s.eventRecorder.Eventf(ls, v1core.EventTypeNormal, "initialize", fmt.Sprintf("waiting for plugin to initialize %s localstorage", ls.Name))
	}

	return nil
}

func (s *StorageController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()
	defer s.eventBroadcaster.Shutdown()
	defer s.queue.ShutDown()

	klog.Infof("Starting Localstorage Manager")
	defer klog.Infof("Shutting down Localstorage Manager")

	if !cache.WaitForNamedCacheSync("localstorage-manager", ctx.Done(), s.lsListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, s.worker, time.Second)
	}

	<-ctx.Done()
}

func (s *StorageController) worker(ctx context.Context) {
	for s.processNextWorkItem(ctx) {
	}
}

func (s *StorageController) processNextWorkItem(ctx context.Context) bool {
	key, quit := s.queue.Get()
	if quit {
		return false
	}
	defer s.queue.Done(key)

	err := s.syncHandler(ctx, key.(string))
	s.handleErr(ctx, err, key)

	return true
}

func (s *StorageController) handleErr(ctx context.Context, err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1core.NamespaceTerminatingCause) {
		s.queue.Forget(key)
		return
	}
	ns, name, keyErr := cache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		klog.Error(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if s.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Info("Error syncing localstorage", "localstorage", klog.KRef(ns, name), "err", err)
		s.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).Info("Dropping localstorage out of the queue", "localstorage", klog.KRef(ns, name), "err", err)
	s.queue.Forget(key)
}

func (s *StorageController) enqueue(ls *localstoragev1.LocalStorage) {
	key, err := KeyFunc(ls)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", ls, err))
		return
	}

	s.queue.Add(key)
}

func (s *StorageController) enqueueRateLimited(ls *localstoragev1.LocalStorage) {
	key, err := KeyFunc(ls)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", ls, err))
		return
	}

	s.queue.AddRateLimited(key)
}

func (s *StorageController) enqueueAfter(ls *localstoragev1.LocalStorage, after time.Duration) {
	key, err := KeyFunc(ls)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", ls, err))
		return
	}

	s.queue.AddAfter(key, after)
}
