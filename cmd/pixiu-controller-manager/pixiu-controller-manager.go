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
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/advanceddeployment"
	dClientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	informers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions"
	"github.com/caoyingjunz/pixiu/pkg/signals"
)

var (
	// Path to a kubeconfig. Only required if out-of-cluster
	kubeconfig string
	healthzHost string
	healthzPort string
)

const (
	workers = 5
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

	clientSet, err := dClientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Fatalf("Error building pixiu clientset: %s", err)
	}

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: clientConfig}
	pixiuInformerFactory := informers.NewSharedInformerFactory(clientSet, time.Second+30)

	controllerContext, err := app.CreateControllerContext(clientBuilder, clientBuilder, stopCh)
	if err != nil {
		klog.Fatalf("Create contoller context failed: %v", err)
	}

	pc, err := advanceddeployment.NewPixiuController(
		clientSet,
		pixiuInformerFactory.Apps().V1alpha1().AdvancedDeployments(),
		controllerContext.InformerFactory.Core().V1().Pods(),
		clientBuilder.ClientOrDie("shared-informers"),
	)
	if err != nil {
		klog.Fatalf("New pixiu controller failed %v", err)
	}

	controllerContext.InformerFactory.Start(stopCh)
	controllerContext.ObjectOrMetadataInformerFactory.Start(stopCh)
	pixiuInformerFactory.Start(stopCh)

	go pc.Run(workers, stopCh)

	// Heathz Check
	go StartHealthzServer(healthzHost,healthzPort)
	
	// always wait
	select {}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&healthzHost, "healthzHost", "", "The host of Healthz")
	flag.StringVar(&healthzPort, "healthzPort", "", "The port of Healthz to listen on")
}
