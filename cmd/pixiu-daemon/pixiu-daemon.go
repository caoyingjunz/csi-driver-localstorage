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

	kubeConfig, err := controller.BuildKubeConfig()
	if err != nil {
		klog.Fatalf("Build kube config failed: %v", err)
	}

	clientSet, err := clientset.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Error building imageset clientset: %v", err)
	}

	clientBuilder := controller.SimpleControllerClientBuilder{ClientConfig: kubeConfig}
	isInformerFactory := informer.NewSharedInformerFactory(clientSet, time.Second+30)

	isc, err := imageset.NewImageSetController(
		clientSet,
		isInformerFactory.Apps().V1alpha1().ImageSets(),
		clientBuilder.ClientOrDie("shared-informers"),
	)
	if err != nil {
		klog.Fatalf("Error new ImageSetController: %v", err)
	}

	go isc.Run(5, stopCh)

	isInformerFactory.Start(stopCh)

	// always wait
	select {}
}
