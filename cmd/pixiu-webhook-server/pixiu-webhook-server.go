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

package main

import (
	"flag"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app"
	"github.com/caoyingjunz/pixiu/pkg/controller/webhook"
)

const (
	HealthzHost = "127.0.0.1"
	HealthzPort = "10259"

	mutateURL   = "/mutate"
	validateURL = "/validate"

	Port     = 8443
	CertFile = "/run/secrets/tls/tls.crt"
	KeyFile  = "/run/secrets/tls/tls.key"

)

func main() {

	klog.InitFlags(nil)
	flag.Parse()

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc(mutateURL, webhook.HandlerMutate)
	mux.HandleFunc(validateURL, webhook.HandlerValidate)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}

	// Start Heathz Check
	go app.StartHealthzServer(healthzHost, healthzPort)

	klog.Infof("Starting Webhook Server...")
	// TODO: use flag to pass the certFile and keyFile
	klog.Fatal(server.ListenAndServeTLS(CertFile, KeyFile))
}

var (
	healthzHost string // The host of Healthz
	healthzPort string // The port of Healthz to listen on
	port int           // The port of Webhook to listen on
	certFile string    // path to the x509 certificate for https
	keyFile string     // path to the x509 private key matching `CertFile`
)

func init() {
	flag.StringVar(&healthzHost, "healthz-host", HealthzHost, "The host of Healthz.")
	flag.StringVar(&healthzPort, "healthz-port", HealthzPort, "The port of Healthz to listen on.")
	flag.IntVar(&port, "port", Port, "Webhook server port.")
	flag.StringVar(&certFile, "tlsCertFile", CertFile, "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&keyFile, "tlsKeyFile", KeyFile, "File containing the x509 private key to --tlsCertFile.")
}
