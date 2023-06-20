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
	"encoding/json"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

type LocalstorageMutate struct {
	Client  client.Client
	decoder *admission.Decoder
}

var _ admission.Handler = &LocalstorageMutate{}
var _ admission.DecoderInjector = &LocalstorageMutate{}

type SetFunc func(ls *localstoragev1.LocalStorage, op admissionv1.Operation)

func (s *LocalstorageMutate) Handle(ctx context.Context, req admission.Request) admission.Response {
	ls := &localstoragev1.LocalStorage{}
	if err := s.decoder.Decode(req, ls); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.Infof("Mutating localstorage %s for %s", ls.Name, req.Operation)

	// add finalizer into localstorage if necessary
	if ls.DeletionTimestamp.IsZero() {
		s.SetFinalizer(ls)
	}

	// set localstorage default values
	s.Default(ls, req.Operation, s.SetStatus, s.SetDisks, s.SetVolumes)

	// PatchResponseFromRaw
	data, err := json.Marshal(ls)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	klog.Infof("Mutated localstorage %+v for %s", ls, req.Operation)
	return admission.PatchResponseFromRaw(req.Object.Raw, data)
}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (s *LocalstorageMutate) Default(ls *localstoragev1.LocalStorage, op admissionv1.Operation, fn ...SetFunc) {
	for _, f := range fn {
		f(ls, op)
	}
}

func (s *LocalstorageMutate) SetStatus(ls *localstoragev1.LocalStorage, op admissionv1.Operation) {
	if op == admissionv1.Create {
		if len(ls.Status.Phase) == 0 {
			ls.Status.Phase = localstoragev1.LocalStoragePending
		}
	}
}

// SetDisks set the identifier to empty if provider when Created
func (s *LocalstorageMutate) SetDisks(ls *localstoragev1.LocalStorage, op admissionv1.Operation) {
	if op == admissionv1.Create {
		disks := ls.Spec.Disks
		if disks == nil {
			return
		}

		var newDisk []localstoragev1.DiskSpec
		for _, disk := range disks {
			if len(disk.Name) == 0 {
				continue
			}
			newDisk = append(newDisk, localstoragev1.DiskSpec{Name: disk.Name})
		}

		ls.Spec.Disks = newDisk
	}
}

func (s *LocalstorageMutate) SetVolumes(ls *localstoragev1.LocalStorage, op admissionv1.Operation) {
	if op == admissionv1.Create {
		if len(ls.Status.Volumes) != 0 {
			ls.Status.Volumes = nil
		}
	}
}

// SetFinalizer add finalizer if necessary
func (s *LocalstorageMutate) SetFinalizer(ls *localstoragev1.LocalStorage) {
	if !util.ContainsFinalizer(ls, util.LsProtectionFinalizer) {
		util.AddFinalizer(ls, util.LsProtectionFinalizer)
	}
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected by InjectDecoderInto.
func (s *LocalstorageMutate) InjectDecoder(d *admission.Decoder) error {
	s.decoder = d
	return nil
}
