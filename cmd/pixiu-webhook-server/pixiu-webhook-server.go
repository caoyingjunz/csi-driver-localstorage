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
	"net/http"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app"
	"github.com/caoyingjunz/pixiu/pkg/controller/webhook"
)

const (
	HealthzHost = "127.0.0.1"
	HealthzPort = "10259"

	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `tls.crt`
	tlsKeyFile  = `tls.key`
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", webhook.Serve)
	mux.HandleFunc("/validate", webhook.Serve)

	server := &http.Server{
		Addr:    ":8443",
		Handler: mux,
	}

	// Start Heathz Check
	go app.StartHealthzServer(healthzHost, healthzPort)

	klog.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}

var (
	healthzHost string // The host of Healthz
	healthzPort string // The port of Healthz to listen on
)

func init() {
	flag.StringVar(&healthzHost, "healthz-host", HealthzHost, "The host of Healthz.")
	flag.StringVar(&healthzPort, "healthz-port", HealthzPort, "The port of Healthz to listen on.")
}
