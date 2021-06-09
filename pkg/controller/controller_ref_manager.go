/*
Copyright 2016 The Kubernetes Authors.

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

package controller

import (
	"encoding/json"
	"fmt"
	"sync"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
)

// RecheckDeletionTimestamp returns a CanAdopt() function to recheck deletion.
//
// The CanAdopt() function calls getObject() to fetch the latest value,
// and denies adoption attempts if that object has a non-nil DeletionTimestamp.
func RecheckDeletionTimestamp(getObject func() (metav1.Object, error)) func() error {
	return func() error {
		obj, err := getObject()
		if err != nil {
			return fmt.Errorf("can't recheck DeletionTimestamp: %v", err)
		}
		if obj.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", obj.GetNamespace(), obj.GetName(), obj.GetDeletionTimestamp())
		}
		return nil
	}
}

func NewPodControllerRefManager(
	podControl PodControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *PodControllerRefManager {
	return &PodControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		podControl:     podControl,
	}
}

type PodControllerRefManager struct {
	BaseControllerRefManager
	controllerKind schema.GroupVersionKind
	podControl     PodControlInterface
}
type BaseControllerRefManager struct {
	Controller metav1.Object
	Selector   labels.Selector

	canAdoptErr  error
	canAdoptOnce sync.Once
	CanAdoptFunc func() error
}

func (m *PodControllerRefManager) ClaimPods(pods []*v1.Pod, filters ...func(*v1.Pod) bool) ([]*v1.Pod, error) {
	var claimed []*v1.Pod
	var errlist []error

	match := func(obj metav1.Object) bool {
		pod := obj.(*v1.Pod)
		// Check selector first so filters only run on potentially matching Pods.
		if !m.Selector.Matches(labels.Set(pod.Labels)) {
			return false
		}
		for _, filter := range filters {
			if !filter(pod) {
				return false
			}
		}
		return true
	}
	adopt := func(obj metav1.Object) error {
		return m.AdoptPod(obj.(*v1.Pod))
	}
	release := func(obj metav1.Object) error {
		return m.ReleasePod(obj.(*v1.Pod))
	}

	for _, pod := range pods {
		ok, err := m.ClaimObject(pod, match, adopt, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, pod)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

func (m *BaseControllerRefManager) ClaimObject(obj metav1.Object, match func(metav1.Object) bool, adopt, release func(metav1.Object) error) (bool, error) {
	controllerRef := metav1.GetControllerOfNoCopy(obj)
	if controllerRef != nil {
		if controllerRef.UID != m.Controller.GetUID() {
			// Owned by someone else. Ignore.
			return false, nil
		}
		if match(obj) {
			// We already own it and the selector matches.
			// Return true (successfully claimed) before checking deletion timestamp.
			// We're still allowed to claim things we already own while being deleted
			// because doing so requires taking no actions.
			return true, nil
		}
		// Owned by us but selector doesn't match.
		// Try to release, unless we're being deleted.
		if m.Controller.GetDeletionTimestamp() != nil {
			return false, nil
		}
		if err := release(obj); err != nil {
			// If the pod no longer exists, ignore the error.
			if errors.IsNotFound(err) {
				return false, nil
			}
			// Either someone else released it, or there was a transient error.
			// The controller should requeue and try again if it's still stale.
			return false, err
		}
		// Successfully released.
		return false, nil
	}

	// It's an orphan.
	if m.Controller.GetDeletionTimestamp() != nil || !match(obj) {
		// Ignore if we're being deleted or selector doesn't match.
		return false, nil
	}
	if obj.GetDeletionTimestamp() != nil {
		// Ignore if the object is being deleted
		return false, nil
	}
	// Selector matches. Try to adopt.
	if err := adopt(obj); err != nil {
		// If the pod no longer exists, ignore the error.
		if errors.IsNotFound(err) {
			return false, nil
		}
		// Either someone else claimed it first, or there was a transient error.
		// The controller should requeue and try again if it's still orphaned.
		return false, err
	}
	// Successfully adopted.
	return true, nil
}

// AdoptPod sends a patch to take control of the pod. It returns the error if
// the patching fails.
func (m *PodControllerRefManager) AdoptPod(pod *v1.Pod) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt Pod %v/%v (%v): %v", pod.Namespace, pod.Name, pod.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.

	patchBytes, err := ownerRefControllerPatch(m.Controller, m.controllerKind, pod.UID)
	if err != nil {
		return err
	}
	return m.podControl.PatchPod(pod.Namespace, pod.Name, patchBytes)
}

func (m *BaseControllerRefManager) CanAdopt() error {
	m.canAdoptOnce.Do(func() {
		if m.CanAdoptFunc != nil {
			m.canAdoptErr = m.CanAdoptFunc()
		}
	})
	return m.canAdoptErr
}

type objectForDeleteOwnerRefStrategicMergePatch struct {
	Metadata objectMetaForMergePatch `json:"metadata"`
}

type objectMetaForMergePatch struct {
	UID             types.UID           `json:"uid"`
	OwnerReferences []map[string]string `json:"ownerReferences"`
}

func deleteOwnerRefStrategicMergePatch(dependentUID types.UID, ownerUIDs ...types.UID) ([]byte, error) {
	var pieces []map[string]string
	for _, ownerUID := range ownerUIDs {
		pieces = append(pieces, map[string]string{"$patch": "delete", "uid": string(ownerUID)})
	}
	patch := objectForDeleteOwnerRefStrategicMergePatch{
		Metadata: objectMetaForMergePatch{
			UID:             dependentUID,
			OwnerReferences: pieces,
		},
	}
	patchBytes, err := json.Marshal(&patch)
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}

// ReleasePod sends a patch to free the pod from the control of the controller.
// It returns the error if the patching fails. 404 and 422 errors are ignored.
func (m *PodControllerRefManager) ReleasePod(pod *v1.Pod) error {
	klog.V(2).Infof("patching pod %s_%s to remove its controllerRef to %s/%s:%s",
		pod.Namespace, pod.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	patchBytes, err := deleteOwnerRefStrategicMergePatch(pod.UID, m.Controller.GetUID())
	if err != nil {
		return err
	}
	err = m.podControl.PatchPod(pod.Namespace, pod.Name, patchBytes)
	if err != nil {
		if errors.IsNotFound(err) {
			// If the pod no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the pod
			// has no owner reference, 2. the uid of the pod doesn't
			// match, which means the pod is deleted and then recreated.
			// In both cases, the error can be ignored.

			// TODO: If the pod has owner references, but none of them
			// has the owner.UID, server will silently ignore the patch.
			// Investigate why.
			return nil
		}
	}
	return err
}

type objectForAddOwnerRefPatch struct {
	Metadata objectMetaForPatch `json:"metadata"`
}
type objectMetaForPatch struct {
	OwnerReferences []metav1.OwnerReference `json:"ownerReferences"`
	UID             types.UID               `json:"uid"`
}

func ownerRefControllerPatch(controller metav1.Object, controllerKind schema.GroupVersionKind, uid types.UID) ([]byte, error) {
	blockOwnerDeletion := true
	isController := true
	addControllerPatch := objectForAddOwnerRefPatch{
		Metadata: objectMetaForPatch{
			UID: uid,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         controllerKind.GroupVersion().String(),
					Kind:               controllerKind.Kind,
					Name:               controller.GetName(),
					UID:                controller.GetUID(),
					Controller:         &isController,
					BlockOwnerDeletion: &blockOwnerDeletion,
				},
			},
		},
	}
	patchBytes, err := json.Marshal(&addControllerPatch)
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
}
