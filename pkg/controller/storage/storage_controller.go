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
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
)

type StorageController struct {
	client     versioned.Interface
	kubeClient kubernetes.Interface

	lsLister       localstorage.LocalStorageLister
	lsListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

// NewStorageController creates a new StorageController.
func NewStorageController(ctx context.Context, lsInformer v1.LocalStorageInformer, lsClientSet versioned.Interface, kubeClientSet kubernetes.Interface) (*StorageController, error) {
	sc := &StorageController{
		client:     lsClientSet,
		kubeClient: kubeClientSet,
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

	sc.lsLister = lsInformer.Lister()
	sc.lsListerSynced = lsInformer.Informer().HasSynced
	return sc, nil
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

func (s *StorageController) addStorage(obj interface{}) {
	fmt.Println("new", obj)
}

func (s *StorageController) updateStorage(old, cur interface{}) {
	fmt.Println("update", old)
}

func (s *StorageController) deleteStorage(obj interface{}) {
	fmt.Println("del", obj)
}

func (s *StorageController) worker(ctx context.Context) {
	fmt.Println("worker")
}
