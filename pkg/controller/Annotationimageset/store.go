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

package Annotationimageset

import (
	"sync"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
)

type SafeStoreInterface1 interface {
	// Adds a hpa into the store.
	Add(key string, obj *appsv1alpha1.ImageSet)
	// Update a hpa into the store if exists, or Add it.
	Update(key string, obj *appsv1alpha1.ImageSet)
	// Delete the hpa from store by key
	Delete(key string)
	// Get the hpa from store by gived key
	Get(key string) (*appsv1alpha1.ImageSet, bool)
}

type SafeStore1 struct {
	lock  sync.RWMutex
	items map[string]*appsv1alpha1.ImageSet
}

func (s *SafeStore1) Get(key string) (*appsv1alpha1.ImageSet, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	item, exists := s.items[key]
	return item, exists
}

func (s *SafeStore1) Add(key string, obj *appsv1alpha1.ImageSet) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.items[key] = obj
}

func (s *SafeStore1) Update(key string, obj *appsv1alpha1.ImageSet) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.items[key] = obj
}

func (s *SafeStore1) Delete(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.items[key]; ok {
		delete(s.items, key)
	}
}

// NewSafeStore creates and returns a reference to an empty store. Operations
// on the resulting set are thread safe.
func newSafeStore() SafeStoreInterface1 {
	return &SafeStore1{
		items: map[string]*appsv1alpha1.ImageSet{},
	}
}

// Empty is public since it is used by some internal API objects for conversions between external
// string arrays and internal sets, and conversion logic requires public types today.
type Empty struct{}

// SafeSet is the primary interface. It
// represents an unordered set of data and a large number of
// operations that can be applied to that set.
type SafeSetInterface1 interface {
	// Adds an element to the set.
	Add(i interface{})

	// Returns whether the given item
	// is in the set.
	Has(i interface{}) bool
}

type SafeSet1 struct {
	lock sync.RWMutex
	set  map[interface{}]Empty
}

func (s *SafeSet1) Add(i interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.set[i] = Empty{}
}

// Has returns true if and only if item is contained in the set.
func (s *SafeSet1) Has(i interface{}) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	_, containd := s.set[i]
	return containd
}

// NewSafeSet creates and returns a reference to an empty set. Operations
// on the resulting set are thread safe.
func NewSafeSet(s ...interface{}) SafeSetInterface1 {
	ss := &SafeSet1{
		set: map[interface{}]Empty{},
	}
	for _, item := range s {
		ss.Add(item)
	}

	return ss
}
