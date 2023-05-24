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
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
)

func main() {
	klog.Infof("Starting localstorage controller")

	kubeConfig, err := BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}

	clientSet, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to build localstorage client: %v", err)
	}

	ls, err := clientSet.StorageV1().LocalStorages().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Fatalf("Failed to list localstorages: %v", err)
	}

	fmt.Println(ls.Items)
}

func BuildClientConfig(configFile string) (*restclient.Config, error) {
	if len(configFile) == 0 {
		configFile = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	return clientcmd.BuildConfigFromFlags("", configFile)
}
