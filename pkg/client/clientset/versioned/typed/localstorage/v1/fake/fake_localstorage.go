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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeLocalStorages implements LocalStorageInterface
type FakeLocalStorages struct {
	Fake *FakeLocalstorageV1
}

var localstoragesResource = schema.GroupVersionResource{Group: "localstorage.caoyingjunz.io", Version: "v1", Resource: "localstorages"}

var localstoragesKind = schema.GroupVersionKind{Group: "localstorage.caoyingjunz.io", Version: "v1", Kind: "LocalStorage"}

// Get takes name of the localStorage, and returns the corresponding localStorage object, and an error if there is any.
func (c *FakeLocalStorages) Get(ctx context.Context, name string, options v1.GetOptions) (result *localstoragev1.LocalStorage, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(localstoragesResource, name), &localstoragev1.LocalStorage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*localstoragev1.LocalStorage), err
}

// List takes label and field selectors, and returns the list of LocalStorages that match those selectors.
func (c *FakeLocalStorages) List(ctx context.Context, opts v1.ListOptions) (result *localstoragev1.LocalStorageList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(localstoragesResource, localstoragesKind, opts), &localstoragev1.LocalStorageList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &localstoragev1.LocalStorageList{ListMeta: obj.(*localstoragev1.LocalStorageList).ListMeta}
	for _, item := range obj.(*localstoragev1.LocalStorageList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested localStorages.
func (c *FakeLocalStorages) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(localstoragesResource, opts))
}

// Create takes the representation of a localStorage and creates it.  Returns the server's representation of the localStorage, and an error, if there is any.
func (c *FakeLocalStorages) Create(ctx context.Context, localStorage *localstoragev1.LocalStorage, opts v1.CreateOptions) (result *localstoragev1.LocalStorage, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(localstoragesResource, localStorage), &localstoragev1.LocalStorage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*localstoragev1.LocalStorage), err
}

// Update takes the representation of a localStorage and updates it. Returns the server's representation of the localStorage, and an error, if there is any.
func (c *FakeLocalStorages) Update(ctx context.Context, localStorage *localstoragev1.LocalStorage, opts v1.UpdateOptions) (result *localstoragev1.LocalStorage, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(localstoragesResource, localStorage), &localstoragev1.LocalStorage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*localstoragev1.LocalStorage), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeLocalStorages) UpdateStatus(ctx context.Context, localStorage *localstoragev1.LocalStorage, opts v1.UpdateOptions) (*localstoragev1.LocalStorage, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(localstoragesResource, "status", localStorage), &localstoragev1.LocalStorage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*localstoragev1.LocalStorage), err
}

// Delete takes name of the localStorage and deletes it. Returns an error if one occurs.
func (c *FakeLocalStorages) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(localstoragesResource, name, opts), &localstoragev1.LocalStorage{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeLocalStorages) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(localstoragesResource, listOpts)

	_, err := c.Fake.Invokes(action, &localstoragev1.LocalStorageList{})
	return err
}

// Patch applies the patch and returns the patched localStorage.
func (c *FakeLocalStorages) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *localstoragev1.LocalStorage, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(localstoragesResource, name, pt, data, subresources...), &localstoragev1.LocalStorage{})
	if obj == nil {
		return nil, err
	}
	return obj.(*localstoragev1.LocalStorage), err
}
