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

	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller/app"
	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller/app/config"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/pixiu"
	clientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	informers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions"
	"github.com/caoyingjunz/pixiu/pkg/signals"
)

const (
	workers = 5
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	kubeConfig, err := config.BuildKubeConfig()
	if err != nil {
		klog.Fatalf("Build kube config failed: %v", err)
	}

	clientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: kubeConfig,
	}

	clientSet, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Error building pixiu clientset: %s", err)
	}

	pixiuInformerFactory := informers.NewSharedInformerFactory(clientset, time.Second+30)

	controllerContext, err := app.CreateControllerContext(clientBuilder, clientBuilder, stopCh)
	if err != nil {
		klog.Fatalf("Create contoller context failed: %v", err)
	}

	pc, err := pixiu.NewPixiuController(
		controllerContext.InformerFactory.Core().V1().Pods(),
		clientBuilder.ClientOrDie("shared-informers"),
	)
	if err != nil {
		klog.Fatalf("New pixiu controller failed %v", err)
	}

	go pc.Run(workers, stopCh)

	controllerContext.InformerFactory.Start(stopCh)
	controllerContext.ObjectOrMetadataInformerFactory.Start(stopCh)
	pixiuInformerFactory.Start(stopCh)

	// always wait
	select {}
}
