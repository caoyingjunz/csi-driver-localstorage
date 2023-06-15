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
	if err = v.ValidateName(ctx, ls); err != nil {
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

func (v *LocalstorageValidator) ValidateName(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	if len(ls.Name) == 0 {
		return fmt.Errorf("localstorage (%s) name must be than 0 characters", ls.Name)
	}
	if len(ls.Name) > validationutils.DNS1035LabelMaxLength-11 {
		return fmt.Errorf("localstorage (%s) name must be no more than 52 characters", ls.Name)
	}

	return nil
}

func (v *LocalstorageValidator) ValidateCreate(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate create", "name", ls.Name)

	if err := v.validateLocalStorageNode(ctx, ls); err != nil {
		return err
	}
	if err := v.validateVolumeGroup(ctx, ls); err != nil {
		return err
	}

	return nil
}

func (v *LocalstorageValidator) ValidateUpdate(ctx context.Context, old, cur *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate update", "name", cur.Name)

	// validate common object
	if old.Name != cur.Name || old.APIVersion != cur.APIVersion || old.Kind != cur.Kind {
		return fmt.Errorf("at least one of apiVersion, kind and name was changed")
	}

	// validate spec
	if old.Spec.Node != cur.Spec.Node {
		return fmt.Errorf("spec.node: Invalid value: %v: field is immutable", cur.Spec.Node)
	}
	if old.Spec.VolumeGroup != cur.Spec.VolumeGroup {
		return fmt.Errorf("spec.volumeGroup: Invalid value: %v: field is immutable", cur.Spec.VolumeGroup)
	}

	return nil
}

func (v *LocalstorageValidator) ValidateDelete(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate delete", "name", ls.Name)
	return nil
}

// validate localstorage node
// 1. localstorage node can't be empty
// 2. localstorage node must be in kubernetes
// 3. only the one node to binding to
func (v *LocalstorageValidator) validateLocalStorageNode(ctx context.Context, ls *localstoragev1.LocalStorage) error {
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

func (v *LocalstorageValidator) validateVolumeGroup(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	if len(ls.Spec.VolumeGroup) == 0 {
		return fmt.Errorf("spec.volumeGroup may not be empty")
	}

	return nil
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected by InjectDecoderInto.
func (v *LocalstorageValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
