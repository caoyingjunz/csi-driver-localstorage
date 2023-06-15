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

package localstorage

import (
	"context"
	"fmt"
	"path"
	"sync"
	"time"

	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	kubecache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	v1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

const (
	DefaultDriverName = "localstorage.csi.caoyingjunz.io"
	StoreFile         = "localstorage.json"

	annNodeSize = "volume.caoyingjunz.io/node-size"
	maxRetries  = 15
)

type localStorage struct {
	config Config
	cache  cache.Cache

	lock sync.Mutex

	client     versioned.Interface
	kubeClient kubernetes.Interface

	lsLister       localstorage.LocalStorageLister
	lsListerSynced kubecache.InformerSynced

	queue workqueue.RateLimitingInterface
}

type Config struct {
	DriverName    string
	Endpoint      string
	VendorVersion string
	NodeId        string
	// Deprecated: 临时使用，后续删除
	VolumeDir string
}

func NewLocalStorage(ctx context.Context, cfg Config, lsInformer v1.LocalStorageInformer, lsClientSet versioned.Interface, kubeClientSet kubernetes.Interface) (*localStorage, error) {
	if cfg.DriverName == "" {
		return nil, fmt.Errorf("no driver name provided")
	}
	if len(cfg.NodeId) == 0 {
		return nil, fmt.Errorf("no node id provided")
	}
	klog.V(2).Infof("Driver: %v version: %v, nodeId: %v", cfg.DriverName, cfg.VendorVersion, cfg.NodeId)

	if err := makeVolumeDir(cfg.VolumeDir); err != nil {
		return nil, err
	}
	storeFile := path.Join(cfg.VolumeDir, StoreFile)
	klog.V(2).Infof("localstorage will be store in %s", storeFile)

	s, err := cache.New(storeFile)
	if err != nil {
		return nil, err
	}

	ls, err := &localStorage{
		config:     cfg,
		cache:      s,
		kubeClient: kubeClientSet,
		client:     lsClientSet,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "plugin"),
	}, nil
	if err != nil {
		return nil, err
	}

	lsInformer.Informer().AddEventHandler(kubecache.FilteringResourceEventHandler{
		Handler: kubecache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ls.addStorage(obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				ls.updateStorage(oldObj, newObj)
			},
		},
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *localstoragev1.LocalStorage:
				return util.AssignedLocalstorage(t, cfg.NodeId)
			default:
				klog.Infof("handle object error")
				return false
			}
		},
	})

	ls.lsLister = lsInformer.Lister()
	ls.lsListerSynced = lsInformer.Informer().HasSynced
	return ls, nil
}

func (ls *localStorage) Run(ctx context.Context) error {
	defer utilruntime.HandleCrash()
	defer ls.queue.ShutDown()

	if !kubecache.WaitForNamedCacheSync("localstorage-plugin", ctx.Done(), ls.lsListerSynced) {
		return fmt.Errorf("failed to WaitForNamedCacheSync")
	}
	go wait.UntilWithContext(ctx, ls.worker, time.Second)

	s := NewNonBlockingGRPCServer()

	s.Start(ls.config.Endpoint, ls, ls, ls)
	s.Wait()

	return nil
}

func (ls *localStorage) sync(ctx context.Context, dKey string) error {
	ls.lock.Lock()
	defer ls.lock.Unlock()

	startTime := time.Now()
	klog.V(2).InfoS("Started syncing localstorage plugin", "localstorage", "startTime", startTime)
	defer func() {
		klog.V(2).InfoS("Finished syncing localstorage plugin", "localstorage", "duration", time.Since(startTime))
	}()

	localstorage, err := ls.lsLister.Get(dKey)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).Infof("localstorage has been deleted", dKey)
			return nil
		}
		return err
	}

	// Deep copy otherwise we are mutating the cache.
	l := localstorage.DeepCopy()
	nodeSize, ok := l.Annotations[annNodeSize]
	if !ok {
		return fmt.Errorf("failed to found node localstorage size")
	}
	klog.Infof("get node size %s from annotations", nodeSize)
	quantity, err := resource.ParseQuantity(nodeSize)
	if err != nil {
		return fmt.Errorf("failed to parse node quantity: %v", err)
	}

	l.Status.Capacity = &quantity
	l.Status.Allocatable = &quantity
	l.Status.Phase = localstoragev1.LocalStorageReady
	_, err = ls.client.StorageV1().LocalStorages().Update(ctx, l, metav1.UpdateOptions{})
	return err
}

func (ls *localStorage) updateStorage(old, cur interface{}) {
	oldLs := old.(*localstoragev1.LocalStorage)
	curLs := cur.(*localstoragev1.LocalStorage)
	klog.V(2).Info("Updating localstorage", "localstorage", klog.KObj(oldLs))

	ls.enqueue(curLs)
}

func (ls *localStorage) addStorage(obj interface{}) {
	localstorage := obj.(*localstoragev1.LocalStorage)
	klog.V(2).Info("Adding localstorage", "localstorage", klog.KObj(localstorage))
	ls.enqueue(localstorage)
}

func (ls *localStorage) worker(ctx context.Context) {
	for ls.processNextWorkItem(ctx) {
	}
}

func (ls *localStorage) processNextWorkItem(ctx context.Context) bool {
	key, quit := ls.queue.Get()
	if quit {
		return false
	}
	defer ls.queue.Done(key)

	ls.handleErr(ctx, ls.sync(ctx, key.(string)), key)
	return true
}

func (ls *localStorage) handleErr(ctx context.Context, err error, key interface{}) {
	if err == nil || errors.HasStatusCause(err, v1core.NamespaceTerminatingCause) {
		ls.queue.Forget(key)
		return
	}
	ns, name, keyErr := kubecache.SplitMetaNamespaceKey(key.(string))
	if keyErr != nil {
		klog.Error(err, "Failed to split meta namespace cache key", "cacheKey", key)
	}

	if ls.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Info("Error syncing localstorage", "localstorage", klog.KRef(ns, name), "err", err)
		ls.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	klog.V(2).Info("Dropping localstorage out of the queue", "localstorage", klog.KRef(ns, name), "err", err)
	ls.queue.Forget(key)
}

func (ls *localStorage) enqueue(s *localstoragev1.LocalStorage) {
	key, err := util.KeyFunc(s)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", ls, err))
		return
	}

	ls.queue.Add(key)
}

func (ls *localStorage) GetNode() string {
	return ls.config.NodeId
}
