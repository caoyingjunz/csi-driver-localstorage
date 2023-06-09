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

	"k8s.io/klog/v2"

	admissionv1 "k8s.io/api/admission/v1"
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
	switch req.Operation {
	case admissionv1.Create:
		err = v.ValidateCreate(ctx, ls)
	case admissionv1.Update:
		err = v.ValidateUpdate(ctx, ls)
	case admissionv1.Delete:
		err = v.ValidateDelete(ctx, ls)
	}
	if err != nil {
		return admission.Denied(err.Error())
	}

	if err := v.ValidateLocalStorageNode(ctx, ls); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

func (v *LocalstorageValidator) ValidateCreate(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate create", "name", ls.Name)
	return nil
}

func (v *LocalstorageValidator) ValidateUpdate(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate update", "name", ls.Name)
	return nil
}

func (v *LocalstorageValidator) ValidateDelete(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate delete", "name", ls.Name)
	return nil
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected by InjectDecoderInto.
func (v *LocalstorageValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// make sure one node just have one LocalStorage
func (v *LocalstorageValidator) ValidateLocalStorageNode(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	lsList := &localstoragev1.LocalStorageList{}
	if err := v.Client.List(ctx, lsList); err != nil {
		return err
	}

	for _, lsObj := range lsList.Items {
		if lsObj.Spec.Node == ls.Spec.Node {
			return fmt.Errorf("node: %s, already have a LocalStorage", ls.Spec.Node)
		}
	}

	klog.Infof("about localstorage: %s, lsBindNodeValidate succeed", ls.Name)
	return nil
}
