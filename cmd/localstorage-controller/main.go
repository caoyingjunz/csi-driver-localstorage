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
	"time"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/controller/storage"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	kubeConfig, err := util.BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}
	clientSet, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to build localstorage clientSet: %v", err)
	}

	sharedInformer := externalversions.NewSharedInformerFactory(clientSet, time.Second)
	lsLister := sharedInformer.Storage().V1().LocalStorages().Lister()

	sc, err := storage.NewStorageController(ctx, clientSet, lsLister)
	if err != nil {
		klog.Fatalf("Failed to new storage controller: %s", err)
	}
	sharedInformer.Start(ctx.Done())
	sharedInformer.WaitForCacheSync(ctx.Done())

	klog.Infof("Starting localstorage controller")
	if err = sc.Run(ctx, 4); err != nil {
		klog.Fatalf("failed to start localstorage controller: %v", err)
	}
}
