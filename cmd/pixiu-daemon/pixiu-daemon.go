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
	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app"
	"time"

	"k8s.io/klog/v2"

	clientset "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned"
	informer "github.com/caoyingjunz/pixiu/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/imageset"
	"github.com/caoyingjunz/pixiu/pkg/signals"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	clientConfig, err := controller.BuildKubeConfig(kubeconfig)
	if err != nil {
		klog.Fatalf("Build kube config failed: %v", err)
	}

	clientSet, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Fatalf("Error building imageset clientset: %v", err)
	}

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: clientConfig}
	isInformerFactory := informer.NewSharedInformerFactory(clientSet, time.Second+30)

	hostName, err := imageset.GetHostName(hostnameOverride)
	if err != nil {
		klog.Fatalf("Get hostname failed: %v", err)
	}

	isc, err := imageset.NewImageSetController(
		clientSet,
		isInformerFactory.Apps().V1alpha1().ImageSets(),
		clientBuilder.ClientOrDie("shared-informers"),
		hostName,
	)
	if err != nil {
		klog.Fatalf("Error new ImageSetController: %v", err)
	}

	go isc.Run(5, stopCh)

	// notice that there is no need to run Start methods in a separate goroutine.
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	isInformerFactory.Start(stopCh)

	// Heathz Check
	go app.StartHealthzServer(healthzHost, healthzPort)
	// always wait
	select {}
}

const (
	HealthzHost = "127.0.0.1"
	HealthzPort = "10258"
)

var (
	kubeconfig       string
	hostnameOverride string
	healthzHost       string
	healthzPort       string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&hostnameOverride, "hostnameOverride", "", "The name of the host")
	flag.StringVar(&healthzHost, "healthz-host", HealthzHost, "The host of Healthz.")
	flag.StringVar(&healthzPort, "healthz-port", HealthzPort, "The port of Healthz to listen on.")
}
