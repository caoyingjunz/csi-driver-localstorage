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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
	storageutil "github.com/caoyingjunz/csi-driver-localstorage/pkg/util/storage"
)

var (
	kubeconfig         = flag.String("kubeconfig", "", "paths to a kubeconfig. Only required if out-of-cluster.")
	createLocalstorage = flag.Bool("create-localstorage", true, "Create localstorage object if not present")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	kubeClient, lsClientSet, err := util.NewClientSetsFromConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build clientSets: %v", err)
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "storage.caoyingjunz.io/node",
	})
	if err != nil {
		klog.Fatalf("Failed to get localstorage nodes: %v", err)
	}

	var nodeNames []string
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}
	if err = storageutil.CreateLocalStorages(lsClientSet, nodeNames...); err != nil {
		klog.Fatalf("Failed to create localstorage nodes: %v", err)
	}

	klog.Infof("localstorage job has been completed")
}
