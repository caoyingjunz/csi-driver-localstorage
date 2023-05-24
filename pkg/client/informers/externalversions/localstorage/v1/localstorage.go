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

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	versioned "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	internalinterfaces "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// LocalStorageInformer provides access to a shared informer and lister for
// LocalStorages.
type LocalStorageInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.LocalStorageLister
}

type localStorageInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewLocalStorageInformer constructs a new informer for LocalStorage type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewLocalStorageInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredLocalStorageInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredLocalStorageInformer constructs a new informer for LocalStorage type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredLocalStorageInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LocalstorageV1().LocalStorages().List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LocalstorageV1().LocalStorages().Watch(context.TODO(), options)
			},
		},
		&localstoragev1.LocalStorage{},
		resyncPeriod,
		indexers,
	)
}

func (f *localStorageInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredLocalStorageInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *localStorageInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&localstoragev1.LocalStorage{}, f.defaultInformer)
}

func (f *localStorageInformer) Lister() v1.LocalStorageLister {
	return v1.NewLocalStorageLister(f.Informer().GetIndexer())
}
