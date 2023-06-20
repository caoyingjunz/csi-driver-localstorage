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

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster,shortName={pls, ls}
// +kubebuilder:printcolumn:JSONPath=".status.phase",name=Status,type=string
// +kubebuilder:printcolumn:JSONPath=".spec.node",name=kubeNode,type=string
// +kubebuilder:printcolumn:JSONPath=".status.allocatable",name=Allocatable,type=string
// +kubebuilder:printcolumn:JSONPath=".status.capacity",name=Capacity,type=string
// +kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=AGE,type=date
// +kubebuilder:printcolumn:JSONPath=".spec.volumeGroup",name=VolumeGroup,type=string,priority=1
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LocalStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LocalStorageSpec   `json:"spec,omitempty"`
	Status LocalStorageStatus `json:"status,omitempty"`
}

type LocalStoragePhase string

const (
	LocalStoragePending     LocalStoragePhase = "Pending"
	LocalStorageInitiating  LocalStoragePhase = "Initiating"
	LocalStorageTerminating LocalStoragePhase = "Terminating"
	LocalStorageExtending   LocalStoragePhase = "Extending"
	LocalStorageMaintaining LocalStoragePhase = "Maintaining"
	LocalStorageReady       LocalStoragePhase = "Ready"
	LocalStorageUnknown     LocalStoragePhase = "Unknown"
)

type LocalStorageSpec struct {
	VolumeGroup string `json:"volumeGroup,omitempty"`
	// Node kubernetes node name
	// +kubebuilder:validation:MinLength=1
	Node  string     `json:"node,omitempty"`
	Disks []DiskSpec `json:"disks,omitempty"`
}

type DiskSpec struct {
	Name string `json:"name,omitempty"`
	// disk identifier, plugin will fill it
	Identifier string `json:"identifier,omitempty"`
}

type LocalStorageCondition string

type LocalStorageStatus struct {
	Phase       LocalStoragePhase  `json:"phase,omitempty"`
	Allocatable *resource.Quantity `json:"allocatable,omitempty"`
	Capacity    *resource.Quantity `json:"capacity,omitempty"`

	// List of mount volumes on this node
	// +optional
	Volumes []Volume `json:"volumes,omitempty"`

	Conditions LocalStorageCondition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LocalStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []LocalStorage `json:"items"`
}

type Volume struct {
	VolName  string `json:"volName,omitempty"`
	VolID    string `json:"volId,omitempty"`
	VolPath  string `json:"volPath,omitempty"`
	VolSize  int64  `json:"volSize,omitempty"`
	NodeID   string `json:"nodeId,omitempty"`
	Attached bool   `json:"attached,omitempty"`
}
