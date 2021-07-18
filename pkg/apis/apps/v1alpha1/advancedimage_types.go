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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdvancedImage enables declarative updates for imageSet.
type AdvancedImage struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the AdvancedImage.
	// +optional
	Spec AdvancedImageSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Most recently observed status of the AdvancedDeployment.
	// +optional
	Status AdvancedImageStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AdvancedImageList contains a list of advancedImage
type AdvancedImageList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of AdvancedDeployments.
	Items []AdvancedImage `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// AdvancedImageSpec defines the desired state of advancedImage
type AdvancedImageSpec struct {
	// Number of desired nodes. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	ImageNodes *int32 `json:"imageNodes,omitempty" protobuf:"varint,1,opt,name=imageNodes"`

	// Label selector for ImageSet.
	Selector *metav1.LabelSelector `json:"selector" protobuf:"bytes,2,opt,name=selector"`

	// Template describes the imageSets that will be created.
	Template ImageSetTemplateSpec `json:"template,omitempty" protobuf:"bytes,3,opt,name=template"`

	// Specify number of parallel processes to pull images. Default to 5
	Parallels *int32 `json:"parallels,omitempty" protobuf:"bytes,4,opt,name=parallels"`

	// The maximum time in seconds for a advancedImage to make progress before it
	// is considered to be failed. The advancedImage controller will continue to
	// process failed advancedImage. Defaults to 600s.
	// +optional
	ProgressDelaySeconds *int32 `json:"progressDelaySeconds,omitempty" protobuf:"varint,5,opt,name=progressDelaySeconds"`
}

type ImageSetTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the ImageSet.
	Spec ImageSetSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type AdvancedImageStatus struct {
	// The generation observed by the advancedImage controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty" protobuf:"varint,1,opt,name=observedGeneration"`

	AvailableImageNodes *int32 `json:"availableImageNodes"`

	UnAvailableImageNodes *int32 `json:"unAvailableImageNodes"`
}
