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
	"reflect"
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	clientsetfake "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned/fake"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/testing/wrapper"
)

// 测试环境为 3 node + 2 localstorage
var (
	nodes = []runtime.Object{
		wrapper.MakeNode().WithName("node1").WithDefaultAnnots("default").WithCsiDriverNodeIDAnnots("node1").Obj(),
		wrapper.MakeNode().WithName("node2").WithDefaultAnnots("default").Obj(),
		wrapper.MakeNode().WithName("node3").Obj(),
	}

	ls = []runtime.Object{
		wrapper.MakeLocalStorage().WithName("ls-node1").WithNode("node1").Obj(),
		wrapper.MakeLocalStorage().WithName("ls-node2").WithNode("node2").Obj(),
	}
)

func TestCreateLocalStorage(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		expect   []string
	}{
		{
			name:     "create-ls-node2",
			nodeName: "node2",
			expect:   []string{"ls-node1", "ls-node2"},
		},
		{
			name:     "create-ls-node3",
			nodeName: "node3",
			expect:   []string{"ls-node1", "ls-node2", "ls-node3"},
		},
		{
			name:     "re-create-ls-node3",
			nodeName: "node3",
			expect:   []string{"ls-node1", "ls-node2", "ls-node3"},
		},
	}

	ctx := context.Background()

	lsClient := clientsetfake.NewSimpleClientset(ls...)
	informerFactory := externalversions.NewSharedInformerFactory(lsClient, 1*time.Second)
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	for _, test := range tests {
		if err := CreateLocalStorage(lsClient, test.nodeName); err != nil {
			t.Errorf("case name: %s, create localstorage failed, err: %v\n", test.name, err)
		}

		lsList, err := lsClient.StorageV1().LocalStorages().List(ctx, v1.ListOptions{})
		if err != nil {
			t.Errorf("case name: %s, list localstorage failed, err: %v\n", test.name, err)
		}

		if len(lsList.Items) != len(test.expect) {
			t.Errorf("case name: %s, expected ls num is not equal get", test.name)
		}

		var lsSet []string
		for _, ls := range lsList.Items {
			lsSet = append(lsSet, ls.Name)
		}

		if !reflect.DeepEqual(lsSet, test.expect) {
			t.Errorf("case name: %s, after create localstorage, got is not equal expect", test.name)
		}

		t.Logf("case name: %s tests succeed", test.name)
	}
}

func TestGetLocalStorageByNode(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		expect   *localstoragev1.LocalStorage
	}{
		{
			name:     "get-ls-node1",
			nodeName: "node1",
			expect:   wrapper.MakeLocalStorage().WithName("ls-node1").WithNode("node1").Obj(),
		},
		{
			name:     "get-ls-node3",
			nodeName: "node3",
			expect:   nil,
		},
	}

	ctx := context.Background()

	lsClient := clientsetfake.NewSimpleClientset(ls...)
	informerFactory := externalversions.NewSharedInformerFactory(lsClient, 1*time.Second)
	lsLister := informerFactory.Storage().V1().LocalStorages().Lister()
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	for _, test := range tests {
		target, err := GetLocalStorageByNode(lsLister, test.nodeName)
		if test.expect != nil {
			if err != nil && target == nil {
				t.Errorf("case name: %s, get localstorage by nodename failed, err: %v", test.name, err)
			}

			if target.Name != test.expect.Name {
				t.Errorf("case name: %s, get localstorage is not same as expect, get: %#v, expect: %#v", test.name, target, test.expect)
			}

		} else {
			if err == nil && target != nil {
				t.Errorf("case name: %s, get localstorage is not same as expect, get: %#v, expect: %#v", test.name, target, test.expect)
			}
		}

		t.Logf("case name: %s tests succeed", test.name)
	}
}

func TestGetLocalStorageMap(t *testing.T) {
	tests := []struct {
		name   string
		expect map[string]*localstoragev1.LocalStorage
	}{
		{
			name: "get-localstorage-map",
			expect: map[string]*localstoragev1.LocalStorage{
				"node1": wrapper.MakeLocalStorage().WithName("ls-node1").WithNode("node1").Obj(),
				"node2": wrapper.MakeLocalStorage().WithName("ls-node2").WithNode("node2").Obj(),
			},
		},
	}

	ctx := context.Background()

	lsClient := clientsetfake.NewSimpleClientset(ls...)
	informerFactory := externalversions.NewSharedInformerFactory(lsClient, 1*time.Second)
	lsLister := informerFactory.Storage().V1().LocalStorages().Lister()
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	for _, test := range tests {
		lsMap, err := GetLocalStorageMap(lsLister)
		if err != nil {
			t.Errorf("case name: %s, get localstorage map failed, err: %v", test.name, err)
		}

		if !reflect.DeepEqual(lsMap, test.expect) {
			t.Errorf("case name: %s, get localstorage map is not same as expect, get: %#v, expect: %#v", test.name, lsMap, test.expect)
		}

		t.Logf("case name: %s tests succeed", test.name)
	}

}
