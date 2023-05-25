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
	"context"
	"flag"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
)

func main() {
	klog.Infof("Starting localstorage manager controller")

	kubeConfig, err := util.BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}

	clientSet, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to build localstorage client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sharedInformer := externalversions.NewSharedInformerFactory(clientSet, time.Second)
	lsLister := sharedInformer.Storage().V1().LocalStorages().Lister()

	sharedInformer.Start(ctx.Done())
	sharedInformer.WaitForCacheSync(ctx.Done())

	selector := labels.Everything()

	for {
		time.Sleep(3 * time.Second)
		localstorages, err := lsLister.List(selector)
		if err != nil {
			klog.Errorf("%v", err)
		}

		for _, localstorage := range localstorages {
			fmt.Println(localstorage.Name)
		}

		obj, err := lsLister.Get(localstorages[0].Name)
		if err != nil {
			klog.Errorf("%v", err)
		}
		fmt.Println("obj", obj)

		fmt.Println()
	}
}
