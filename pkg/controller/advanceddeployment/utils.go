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

package advanceddeployment

import (
	"k8s.io/api/core/v1"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/advanceddeployment/v1alpha1"
)

func calculateStatus(ad *appsv1alpha1.AdvancedDeployment, pods []*v1.Pod, manageAdErr error) appsv1alpha1.AdvancedDeploymentStatus {
	newStatus := ad.Status

	return newStatus
}
