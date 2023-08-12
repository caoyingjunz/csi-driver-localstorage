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
	"os"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"

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

// CreateLocalStorages create localstorage by node if not present
func CreateLocalStorages(lsClientSet versioned.Interface, nodeNames ...string) error {
	if len(nodeNames) == 0 {
		return nil
	}

	// Get all exist localstorage object
	localStorages, err := lsClientSet.StorageV1().LocalStorages().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	lsNodeNames := sets.NewString()
	for _, ls := range localStorages.Items {
		// do nothing if the localstorage present
		lsNodeNames.Insert(ls.Spec.Node)
	}

	for _, nodeName := range nodeNames {
		if lsNodeNames.Has(nodeName) {
			continue
		}
		if _, err = lsClientSet.StorageV1().LocalStorages().Create(context.TODO(), &localstoragev1.LocalStorage{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ls-" + nodeName,
			},
			Spec: localstoragev1.LocalStorageSpec{
				Node: nodeName,
				// TODO: 先阶段仅支持 path 模式
				Path: &localstoragev1.PathSpec{
					VolumeDir: "/data",
				},
			},
		}, metav1.CreateOptions{}); err != nil {
			return err
		}

		klog.Infof("Create %s localstorage success", nodeName)
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

func TryUpdateLocalStorage(client versioned.Interface, ls *localstoragev1.LocalStorage) error {
	_, err := client.
		StorageV1().
		LocalStorages().
		Update(context.TODO(), ls, metav1.UpdateOptions{})
	return err
}

func UpdateLocalStoragePhase(client versioned.Interface, ls *localstoragev1.LocalStorage, Phase localstoragev1.LocalStoragePhase) error {
	ls.Status.Phase = Phase
	return TryUpdateLocalStorage(client, ls)
}

func GetVolumeDirFromLocalStorage(ls *localstoragev1.LocalStorage) (string, error) {
	if ls.Spec.Path != nil {
		return ls.Spec.Path.VolumeDir, nil
	}

	return "", fmt.Errorf("spec.path is nil")
}

func CreateVolumeDir(path string) error {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}
	// 给目录赋权限（目录在创建时由于umask原因，即使是给满权限，也有可能会把你的权限给降低，所以采用后置赋权）
	if err := os.Chmod(path, os.ModePerm); err != nil {
		return err
	}

	return nil
}

func DeleteVolumeDir(path string) error {
	if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}
