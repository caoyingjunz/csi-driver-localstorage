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
	v1 "k8s.io/api/core/v1"
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

	return admission.Allowed("")
}

func (v *LocalstorageValidator) ValidateCreate(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	klog.V(2).Infof("validate create", "name", ls.Name)

	if err := v.validateLocalStorageNode(ctx, ls); err != nil {
		return err
	}

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

// validate localstorage node
// 1. localstorage node can't be empty
// 2. localstorage node must be in kubernetes
// 3. only the one node to binding to
func (v *LocalstorageValidator) validateLocalStorageNode(ctx context.Context, ls *localstoragev1.LocalStorage) error {
	if len(ls.Spec.Node) == 0 {
		return fmt.Errorf("localstraoge (%s) binding node may not be empty", ls.Name)
	}

	nodes := &v1.NodeList{}
	if err := v.Client.List(ctx, nodes); err != nil {
		return fmt.Errorf("failed to list kube node objects: %v", err)
	}
	klog.V(2).Infof("found kube nodes %+v", nodes.Items)
	var found bool
	for _, node := range nodes.Items {
		if node.Name == ls.Spec.Node {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("localstorage node (%s) not found in kubernetes", ls.Spec.Node)
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

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected by InjectDecoderInto.
func (v *LocalstorageValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
