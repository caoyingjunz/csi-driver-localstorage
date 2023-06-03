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
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	localstoragewebhook "github.com/caoyingjunz/csi-driver-localstorage/pkg/webhook"
)

var (
	certDir  = flag.String("cert-dir", "", "certDir is the directory that contains the server key and certificate")
	certName = flag.String("cert-name", "tls.crt", "certName is the server certificate name. Defaults to tls.crt")
	keyName  = flag.String("key-name", "tls.key", "keyName is the server key name. Defaults to tls.key.")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	if len(*certDir) == 0 {
		klog.Fatalf("cert-dir is the directory that contains the server key and certificate, it must be provide")
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		klog.Fatalf("unable to set up overall controller manager: %v", err)
	}

	mgr.GetWebhookServer().Register("/mutate-v1-localstorage", &webhook.Admission{Handler: &localstoragewebhook.LocalstorageMutate{Client: mgr.GetClient()}})

	// Set up the cert options
	mgr.GetWebhookServer().CertDir = *certDir
	mgr.GetWebhookServer().CertName = *certName
	mgr.GetWebhookServer().KeyName = *keyName

	klog.Infof("Starting localstorage webhook server")
	ctx := signals.SetupSignalHandler()
	if err = mgr.Start(ctx); err != nil {
		klog.Fatalf("failed start localstorage webhook: %v", err)
	}
}
