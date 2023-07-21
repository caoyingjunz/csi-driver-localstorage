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

package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
)

// AssignedLocalstorage selects ls that are assigned (scheduled and running).
func AssignedLocalstorage(ls *localstoragev1.LocalStorage, nodeId string) bool {
	if ls.Spec.Node != nodeId {
		return false
	}
	return LocalStorageIsInitiating(ls)
}

func SetLocalStoragePhase(ls *localstoragev1.LocalStorage, phase localstoragev1.LocalStoragePhase) {
	ls.Status.Phase = phase
}

func LocalStorageIsPending(ls *localstoragev1.LocalStorage) bool {
	return ls.Status.Phase == localstoragev1.LocalStoragePending
}

func LocalStorageIsInitiating(ls *localstoragev1.LocalStorage) bool {
	return ls.Status.Phase == localstoragev1.LocalStorageInitiating
}

func LocalStorageIsReady(ls *localstoragev1.LocalStorage) bool {
	return ls.Status.Phase == localstoragev1.LocalStorageReady
}

func LocalStorageIsTerminating(ls *localstoragev1.LocalStorage) bool {
	return ls.Status.Phase == localstoragev1.LocalStorageTerminating
}

// AddVolume accepts a volume and adds the provided volume if not present.
func AddVolume(ls *localstoragev1.LocalStorage, volume localstoragev1.Volume) {
	if ContainsVolume(ls, volume.VolID) {
		return
	}

	volumes := GetVolumes(ls)
	SetVolume(ls, append(volumes, volume))
}

// RemoveVolume accepts a volume ID and removes the provided volID if present.
func RemoveVolume(ls *localstoragev1.LocalStorage, volID string) localstoragev1.Volume {
	volumes := GetVolumes(ls)
	var vol localstoragev1.Volume
	for i := 0; i < len(volumes); i++ {
		if volumes[i].VolID == volID {
			vol = volumes[i]
			volumes = append(volumes[:i], volumes[i+1:]...)
		}
	}

	SetVolume(ls, volumes)
	return vol
}

// ContainsVolume checks a volume that the volumeId is present.
func ContainsVolume(ls *localstoragev1.LocalStorage, volID string) bool {
	volumes := GetVolumes(ls)
	for _, v := range volumes {
		if v.VolID == volID {
			return true
		}
	}

	return false
}

func SetVolume(ls *localstoragev1.LocalStorage, volumes []localstoragev1.Volume) {
	ls.Status.Volumes = volumes
}

func GetVolumes(ls *localstoragev1.LocalStorage) []localstoragev1.Volume {
	return ls.Status.Volumes
}

func TryUpdateLocalStorage(client versioned.Interface, ls *localstoragev1.LocalStorage) error {
	_, err := client.
		StorageV1().
		LocalStorages().
		Update(context.TODO(), ls, metav1.UpdateOptions{})
	return err
}
