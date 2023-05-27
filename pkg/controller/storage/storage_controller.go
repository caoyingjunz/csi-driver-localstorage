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
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
)

const (
	maxRetries = 15
)

type StorageController struct {
	client     versioned.Interface
	kubeClient kubernetes.Interface

	syncHandler         func(ctx context.Context, dKey string) error
	enqueueLocalstorage func(ls *localstoragev1.LocalStorage)

	lsLister       localstorage.LocalStorageLister
	lsListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

// NewStorageController creates a new StorageController.
func NewStorageController(ctx context.Context, lsInformer v1.LocalStorageInformer, lsClientSet versioned.Interface, kubeClientSet kubernetes.Interface) (*StorageController, error) {
	sc := &StorageController{
		client:     lsClientSet,
		kubeClient: kubeClientSet,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "localstorage"),
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
	fmt.Println("new", obj)
}

func (s *StorageController) updateStorage(old, cur interface{}) {
	fmt.Println("update", old)
}

func (s *StorageController) deleteStorage(obj interface{}) {
	fmt.Println("del", obj)
}

func (s *StorageController) syncStorage(ctx context.Context, dKey string) error {
	return nil
}

func (s *StorageController) Run(ctx context.Context, workers int) {
	defer utilruntime.HandleCrash()

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
		klog.V(2).Info("Error syncing deployment", "deployment", klog.KRef(ns, name), "err", err)
		s.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).Info("Dropping deployment out of the queue", "deployment", klog.KRef(ns, name), "err", err)
	s.queue.Forget(key)
}

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

func (s *StorageController) enqueue(ls *localstoragev1.LocalStorage) {
	key, err := KeyFunc(ls)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", ls, err))
		return
	}

	s.queue.Add(key)
}
