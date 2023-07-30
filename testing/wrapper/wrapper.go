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

package wrapper

import (
	v1 "k8s.io/api/core/v1"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
)

type NodeWrapper struct{ v1.Node }

func MakeNode() *NodeWrapper {
	return &NodeWrapper{v1.Node{}}
}

// 返回内部封装的 node
func (n *NodeWrapper) Obj() *v1.Node {
	return &n.Node
}

// 为 node 设置 name
func (n *NodeWrapper) WithName(name string) *NodeWrapper {
	n.SetName(name)
	return n
}

// 为 node 设置 annotations
func (n *NodeWrapper) WithDefaultAnnots(annot string) *NodeWrapper {
	if n.Annotations == nil {
		n.Annotations = make(map[string]string)
	}
	n.Annotations["default"] = annot
	return n
}

func (n *NodeWrapper) WithCsiDriverNodeIDAnnots(csiDriverNodeID string) *NodeWrapper {
	n.Annotations["csi.volume.caoyingjunz.io/nodeid"] = csiDriverNodeID
	return n
}

type LocalStorageWrapper struct{ localstoragev1.LocalStorage }

func MakeLocalStorage() *LocalStorageWrapper {
	return &LocalStorageWrapper{localstoragev1.LocalStorage{}}
}

// 返回内部封装的 LocalStorage
func (ls *LocalStorageWrapper) Obj() *localstoragev1.LocalStorage {
	return &ls.LocalStorage
}

// 为 LocalStorage 设置 name
func (ls *LocalStorageWrapper) WithName(name string) *LocalStorageWrapper {
	ls.SetName(name)
	return ls
}

// 为 LocalStorage 设置 node
func (ls *LocalStorageWrapper) WithNode(node string) *LocalStorageWrapper {
	ls.Spec.Node = node
	return ls
}
