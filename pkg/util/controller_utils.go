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
	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
)

// AssignedLocalstorage selects ls that are assigned (scheduled and running).
func AssignedLocalstorage(ls *localstoragev1.LocalStorage, nodeId string) bool {
	if ls.Spec.Node != nodeId {
		return false
	}

	return ls.Status.Phase == localstoragev1.LocalStoragePending || ls.Status.Phase == localstoragev1.LocalStorageMaintaining
}

func IsPendingStatus(ls *localstoragev1.LocalStorage) bool {
	return ls.Status.Phase == localstoragev1.LocalStoragePending
}

func AddVolume(ls *localstoragev1.LocalStorage, volume localstoragev1.Volume) {
}

func RemoveVolume(ls *localstoragev1.LocalStorage, volID string) {
}

func SetVolume(ls *localstoragev1.LocalStorage, volume localstoragev1.Volume) {
}
