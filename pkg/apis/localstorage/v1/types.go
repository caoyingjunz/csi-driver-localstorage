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

package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LocalStoragePhase string

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster,shortName=ls
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=AGE,type=date
// +kubebuilder:printcolumn:JSONPath=".spec.node",name=Node,type=string
// +kubebuilder:printcolumn:JSONPath=".status.capacity",name=Capacity,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.volumeGroup",name=VolumeGroup,type=string,priority=1
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LocalStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocalStorageSpec   `json:"spec,omitempty"`
	Status LocalStorageStatus `json:"status,omitempty"`
}

type LocalStorageSpec struct {
	VolumeGroup string `json:"volumeGroup"`
	Node        string `json:"node"`
}

type LocalStorageStatus struct {
	Capacity resource.Quantity `json:"capacity,omitempty"`
	Phase    LocalStoragePhase `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LocalStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LocalStorage `json:"items"`
}
