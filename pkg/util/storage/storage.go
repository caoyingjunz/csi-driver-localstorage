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
	v1 "k8s.io/api/core/v1"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/types"
)

// GetLocalStorageByNode Get localstorage object by nodeName, error when not found
func GetLocalStorageByNode(nodeName string) (*localstoragev1.LocalStorage, error) {
	//TODO

	return nil, nil
}

func UpdateNodeIDInNode(node *v1.Node, csiDriverNodeID string) *v1.Node {
	if node.ObjectMeta.Annotations == nil {
		node.ObjectMeta.Annotations = make(map[string]string)
	}
	if !IsNodeIDInNode(node) {
		return node
	}

	node.ObjectMeta.Annotations[types.AnnotationKeyNodeID] = csiDriverNodeID
	return node
}

func IsNodeIDInNode(node *v1.Node) bool {
	if node.ObjectMeta.Annotations == nil {
		return false
	}

	_, found := node.ObjectMeta.Annotations[types.AnnotationKeyNodeID]
	return found
}
