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
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/types"
)

// GetLocalStorageByNode Get localstorage object by nodeName, error when not found
func GetLocalStorageByNode(lsLister localstorage.LocalStorageLister, nodeName string) (*localstoragev1.LocalStorage, error) {
	rets, err := lsLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var target *localstoragev1.LocalStorage
	for _, ret := range rets {
		if ret.Spec.Node == nodeName {
			target = ret
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("failed to found localstorage with node %s", nodeName)
	}

	return target, nil
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

func GetNameFromNode(node *v1.Node) string {
	if node.ObjectMeta.Annotations == nil {
		return ""
	}

	return node.ObjectMeta.Annotations[types.AnnotationKeyNodeID]
}
