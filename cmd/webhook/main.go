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
	// webhook
	host = flag.String("host", "", "host is the ip address that the webhook server binds to")
	port = flag.Int("port", 8443, "port is the port that the webhook server serves at")

	// cert
	certDir  = flag.String("cert-dir", "/tmp/webhook-server", "certDir is the directory that contains the server key and certificate")
	certName = flag.String("cert-name", "tls.crt", "certName is the server certificate name. Defaults to tls.crt")
	keyName  = flag.String("key-name", "tls.key", "keyName is the server key name. Defaults to tls.key.")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	webhookManager, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Host: *host,
		Port: *port,
	})
	if err != nil {
		klog.Fatalf("unable to set up overall controller manager: %v", err)
	}
	installCert(webhookManager.GetWebhookServer())

	kubeClient := webhookManager.GetClient()
	// Register webhook APIs
	webhookManager.GetWebhookServer().Register("/mutate-v1-localstorage", &webhook.Admission{Handler: &localstoragewebhook.LocalstorageMutate{Client: kubeClient}})
	webhookManager.GetWebhookServer().Register("/validate-v1-localstorage", &webhook.Admission{Handler: &localstoragewebhook.LocalstorageValidator{Client: kubeClient}})

	klog.Infof("Starting localstorage webhook server")
	ctx := signals.SetupSignalHandler()
	if err = webhookManager.Start(ctx); err != nil {
		klog.Fatalf("failed to start localstorage webhook server: %v", err)
	}
}

// install the cert
func installCert(s *webhook.Server) {
	s.CertDir = *certDir
	s.CertName = *certName
	s.KeyName = *keyName
}
