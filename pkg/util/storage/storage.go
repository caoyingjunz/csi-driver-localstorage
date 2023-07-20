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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/types"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

const (
	DefaultDriverName = "localstorage.csi.caoyingjunz.io"
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

func GetLocalStorageMap(lsLister localstorage.LocalStorageLister) (map[string]*localstoragev1.LocalStorage, error) {
	ret, err := lsLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	lsMap := make(map[string]*localstoragev1.LocalStorage)
	for _, ls := range ret {
		lsMap[ls.Spec.Node] = ls
	}

	return lsMap, nil
}

// CreateLocalStorage create localstorage if not present
func CreateLocalStorage(kubeClientSet kubernetes.Interface, lsClientSet versioned.Interface) error {
	// TODO: need to optimise
	// Get all exists kubernetes nodes
	nodes, err := kubeClientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	// Get all exist localstorage object
	localstorages, err := lsClientSet.StorageV1().LocalStorages().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	localstorageMap := make(map[string]localstoragev1.LocalStorage)
	for _, localstorage := range localstorages.Items {
		localstorageMap[localstorage.Spec.Node] = localstorage
	}

	for _, node := range nodes.Items {
		labels := node.GetLabels()
		if labels == nil {
			continue
		}

		// check whether need to created
		_, exists := labels[types.LabelStorageNode]
		if exists {
			_, found := localstorageMap[node.Name]
			if found {
				continue
			}

			// TODO: to optimise
			if _, err = lsClientSet.StorageV1().LocalStorages().Create(context.TODO(), &localstoragev1.LocalStorage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ls-" + node.Name,
					Annotations: map[string]string{
						"volume.caoyingjunz.io/node-size": "500Gi",
					},
				},
				Spec: localstoragev1.LocalStorageSpec{
					VolumeGroup: "k8s",
					Node:        node.Name,
					Disks: []localstoragev1.DiskSpec{
						{
							Name: "test-disk",
						},
					},
				},
			}, metav1.CreateOptions{}); err != nil {
				return err
			}
		}
	}

	return nil
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

func GetLocalStoragePersistentVolumeClaimFromPod(pod *v1.Pod, pvcLister corelisters.PersistentVolumeClaimLister, scLister storagelisters.StorageClassLister) (*v1.PersistentVolumeClaim, error) {
	volumes := pod.Spec.Volumes
	if volumes == nil || len(volumes) == 0 {
		return nil, nil
	}

	for _, volume := range volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}

		claimName := volume.PersistentVolumeClaim.ClaimName
		pvc, err := util.WaitUntilPersistentVolumeClaimIsCreated(pvcLister, pod.Namespace, claimName, 2*time.Second)
		if err != nil {
			return nil, err
		}

		storageClassName := pvc.Spec.StorageClassName
		if storageClassName == nil || len(*storageClassName) == 0 {
			continue
		}
		sc, err := scLister.Get(*storageClassName)
		if err != nil {
			return nil, fmt.Errorf("failed to get sc %s from indexer: %v", *storageClassName, err)
		}
		if sc.Provisioner == DefaultDriverName {
			return pvc, nil
		}
	}

	return nil, nil
}

func PodIsUseLocalStorage(pod *v1.Pod, pvcLister corelisters.PersistentVolumeClaimLister, scLister storagelisters.StorageClassLister) (bool, error) {
	pvc, err := GetLocalStoragePersistentVolumeClaimFromPod(pod, pvcLister, scLister)
	if err != nil {
		return false, err
	}

	return pvc != nil, nil
}
