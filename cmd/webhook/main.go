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

package main

import (
	"flag"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/webhook"
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		klog.Fatalf("unable to set up overall controller manager: %v", err)
	}

	if err = builder.WebhookManagedBy(mgr).For(&localstoragev1.LocalStorage{}).
		WithDefaulter(&webhook.LocalstorageMutator{}).WithValidator(&webhook.LocalstorageValidator{}).
		Complete(); err != nil {
		klog.Fatalf("failed to create localstorage webhook: %v", err)
	}

	klog.Infof("Starting localstorage webhook server")
	ctx := signals.SetupSignalHandler()
	if err = mgr.Start(ctx); err != nil {
		klog.Fatalf("failed to run localstorage webhook: %v", err)
	}
}
