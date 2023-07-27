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

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	admissionv1 "k8s.io/api/admission/v1"
	validationutils "k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
)

type LocalstorageValidator struct {
	Client  client.Client
	decoder *admission.Decoder
}

var _ admission.Handler = &LocalstorageValidator{}
var _ admission.DecoderInjector = &LocalstorageValidator{}

func (v *LocalstorageValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	ls := &localstoragev1.LocalStorage{}
	if err := v.decoder.Decode(req, ls); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.Infof("Validating localstorage %s for: %s", ls.Name, req.Operation)

	var err error
	if err = v.validateName(ctx, ls); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Create:
		err = v.ValidateCreate(ctx, ls)
	case admissionv1.Update:
		oldLocalstorage := &localstoragev1.LocalStorage{}
		if err = v.decoder.DecodeRaw(req.OldObject, oldLocalstorage); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = v.ValidateUpdate(ctx, oldLocalstorage, ls)
	case admissionv1.Delete:
		err = v.ValidateDelete(ctx, ls)
	}
	if err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected by InjectDecoderInto.
func (v *LocalstorageValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

func (v *LocalstorageValidator) ValidateCreate(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate create %s %s", "name:", ls.Name)

	// validate kubernetes binding node
	if err := v.validateKubeNode(ctx, ls); err != nil {
		return err
	}

	// validate local storage backend
	if err := v.validateStorageBackend(ctx, nil, ls, admissionv1.Create); err != nil {
		return err
	}

	return nil
}

func (v *LocalstorageValidator) ValidateUpdate(ctx context.Context, old, cur *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate update %s %s", "name:", cur.Name)

	// validate common object
	if old.Name != cur.Name || old.APIVersion != cur.APIVersion || old.Kind != cur.Kind {
		return fmt.Errorf("at least one of apiVersion, kind and name was changed")
	}

	// validate node spec
	if old.Spec.Node != cur.Spec.Node {
		return fmt.Errorf("spec.node: Invalid value: %v: field is immutable", cur.Spec.Node)
	}

	// validate local storage backend spec
	if err := v.validateStorageBackend(ctx, old, cur, admissionv1.Update); err != nil {
		return err
	}

	return nil
}

func (v *LocalstorageValidator) ValidateDelete(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate delete %s %s", "name:", ls.Name)
	return nil
}

func (v *LocalstorageValidator) validateName(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	if len(ls.Name) == 0 {
		return fmt.Errorf("localstorage (%s) name must be than 0 characters", ls.Name)
	}
	if len(ls.Name) > validationutils.DNS1035LabelMaxLength-11 {
		return fmt.Errorf("localstorage (%s) name must be no more than 52 characters", ls.Name)
	}

	return nil
}

// validate kube node
// 1. localstorage node can't be empty
// 2. localstorage node must be in kubernetes
// 3. only the one node to binding to
func (v *LocalstorageValidator) validateKubeNode(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	if len(ls.Spec.Node) == 0 {
		return fmt.Errorf("localstraoge (%s) binding node may not be empty", ls.Name)
	}
	node := &v1.Node{}
	if err := v.Client.Get(ctx, types.NamespacedName{Name: ls.Spec.Node}, node); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("localstorage (%s) binding node (%s) not found in kubernetes", ls.Name, ls.Spec.Node)
		}
		return fmt.Errorf("failed to find binding node (%s) object: %v", ls.Spec.Node, err)
	}

	obj := &localstoragev1.LocalStorageList{}
	if err := v.Client.List(ctx, obj); err != nil {
		return fmt.Errorf("failed to list localstorage objects: %v", err)
	}
	for _, localstorage := range obj.Items {
		if localstorage.Spec.Node == ls.Spec.Node {
			return fmt.Errorf("node (%s) already binded to the other localstorage", ls.Spec.Node)
		}
	}

	return nil
}

// validate local storage backend
func (v *LocalstorageValidator) validateStorageBackend(ctx context.Context, old, cur *localstoragev1.LocalStorage, op admissionv1.Operation) error {
	if cur.Spec.Path != nil && cur.Spec.Lvm != nil {
		return fmt.Errorf("path and lvm can only be used at most one")
	}

	// valid hostPath backend spec
	if err := v.validatePath(ctx, old, cur, op); err != nil {
		return err
	}
	// valid vlm backend spec
	if err := v.validateLvm(ctx, old, cur, op); err != nil {
		return err
	}

	return nil
}

func (v *LocalstorageValidator) validatePath(ctx context.Context, old, cur *localstoragev1.LocalStorage, op admissionv1.Operation) error {
	pathSpec := cur.Spec.Path
	pathValidator := func(p *localstoragev1.PathSpec) error {
		if len(p.VolumeDir) == 0 {
			return fmt.Errorf("spec.path.path may not be empty when use path")
		}
		return nil
	}

	switch op {
	case admissionv1.Create:
		if pathSpec == nil {
			return nil
		}
		if err := pathValidator(pathSpec); err != nil {
			return err
		}
	case admissionv1.Update:
		oldPathSpec := old.Spec.Path
		if oldPathSpec == nil && pathSpec != nil {
			if err := pathValidator(pathSpec); err != nil {
				return err
			}
		} else {
			if !reflect.DeepEqual(oldPathSpec, pathSpec) {
				return fmt.Errorf("spec.path: Invalid value: %v: field is immutable", oldPathSpec)
			}
		}
	}

	return nil
}

func (v *LocalstorageValidator) validateLvm(ctx context.Context, old, cur *localstoragev1.LocalStorage, op admissionv1.Operation) error {
	lvmSpec := cur.Spec.Lvm
	lvmValidator := func(l *localstoragev1.LvmSpec) error {
		if len(l.VolumeGroup) == 0 {
			return fmt.Errorf("spec.lvm.volumeGroup may not be empty when use lvm")
		}
		if len(l.Disks) == 0 {
			return fmt.Errorf("spec.lvm.disks may not be empty when use lvm")
		}
		return nil
	}

	switch op {
	case admissionv1.Create:
		if lvmSpec == nil {
			return nil
		}
		if err := lvmValidator(lvmSpec); err != nil {
			return err
		}
	case admissionv1.Update:
		oldLvmSpec := old.Spec.Lvm
		if oldLvmSpec == nil && lvmSpec != nil {
			if err := lvmValidator(lvmSpec); err != nil {
				return err
			}
		} else {
			if !reflect.DeepEqual(oldLvmSpec, lvmSpec) {
				return fmt.Errorf("spec.lvm: Invalid value: %v: field is immutable", oldLvmSpec)
			}
		}
	}

	return nil
}
