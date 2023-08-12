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

package util

import (
	"fmt"
	"path/filepath"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
)

const (
	LocalstorageManagerUserAgent = "localstorage-manager"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

func BuildClientConfig(configFile string) (*restclient.Config, error) {
	if len(configFile) != 0 {
		klog.Infof("kubeconfig specified. building kube config from that")
		return clientcmd.BuildConfigFromFlags("", configFile)
	}

	kubeConfig, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err == nil {
		klog.Infof("kubeconfig not specified. try to building kube config from ~/.kube/config")
		return kubeConfig, nil
	}

	klog.Infof("Building kube configs for running in cluster...")
	return restclient.InClusterConfig()
}

func NewClientSets(kubeConfig *restclient.Config) (kubernetes.Interface, versioned.Interface, error) {
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}
	lsClientSet, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		return nil, nil, err
	}

	return kubeClient, lsClientSet, nil
}

func NewClientSetsFromConfig(configFile string) (kubernetes.Interface, versioned.Interface, error) {
	kubeConfig, err := BuildClientConfig(configFile)
	if err != nil {
		return nil, nil, err
	}

	return NewClientSets(kubeConfig)
}

func CreateRecorder(kubeClient kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: LocalstorageManagerUserAgent})
}

func BytesToQuantity(bytes int64) resource.Quantity {
	quantity := resource.NewQuantity(bytes, resource.BinarySI)
	return *quantity
}

func WaitUntilPersistentVolumeClaimIsCreated(pvcLister corelisters.PersistentVolumeClaimLister, namespace, name string, timeout time.Duration) (*v1.PersistentVolumeClaim, error) {
	// Wait until pvc is created
	pvc, err := pvcLister.PersistentVolumeClaims(namespace).Get(name)
	if err == nil {
		klog.V(2).Infof("waited for pvc %s/%s to be created", namespace, name)
		return pvc, nil
	}

	pollingPeriod := 200 * time.Millisecond
	startTime := time.Now()
	for startTime.Add(timeout).After(time.Now()) {
		time.Sleep(pollingPeriod)
		pvc, err := pvcLister.PersistentVolumeClaims(namespace).Get(name)
		if err == nil {
			klog.V(2).Infof("waited for pvc %s/%s to be created when retry", namespace, name)
			return pvc, nil
		}
	}
	return nil, fmt.Errorf("timed out after %v when waiting for pvc %v/%v to created", timeout, namespace, name)
}
