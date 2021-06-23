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

// Package app implements a Server object for running the pixiu.
package app

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	cacheddiscovery "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/advanceddeployment"
	dClientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	dInformers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions"
)

const workers = 5

// ControllerContext defines the context obj for pixiu
type ControllerContext struct {

	// ClientBuilder will provide a client for this controller to use
	ClientBuilder controller.ControllerClientBuilder

	AdClient dClientset.Interface

	// InformerFactory gives access to informers for the controller.
	InformerFactory informers.SharedInformerFactory

	ObjectOrMetadataInformerFactory controller.InformerFactory

	PixiuInformerFactory dInformers.SharedInformerFactory

	// Stop is the stop channel
	Stop <-chan struct{}

	// ResyncPeriod generates a duration each time it is invoked; this is so that
	// multiple controllers don't get into lock-step and all hammer the apiserver
	// with list requests simultaneously.
	ResyncPeriod func() time.Duration
}

func CreateControllerContext(clientBuilder controller.ControllerClientBuilder, kubeConfig *rest.Config, stop <-chan struct{}) (ControllerContext, error) {
	versionedClient := clientBuilder.ClientOrDie("shared-informers")
	sharedInformers := informers.NewSharedInformerFactory(versionedClient, time.Minute)

	metadataClient := metadata.NewForConfigOrDie(clientBuilder.ConfigOrDie("metadata-informers"))
	metadataInformers := metadatainformer.NewSharedInformerFactory(metadataClient, time.Minute)

	adClient, err := dClientset.NewForConfig(kubeConfig)
	if err != nil {
		return ControllerContext{}, err
	}

	// If APIServer is not runnint we should wait for some time unless failed
	if err := WaitForAPIServer(versionedClient, time.Second*8); err != nil {
		return ControllerContext{}, err
	}

	discoveryClient := clientBuilder.ClientOrDie("controller-discovery")
	cachedClient := cacheddiscovery.NewMemCacheClient(discoveryClient.Discovery())
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedClient)
	go wait.Until(func() {
		restMapper.Reset()
	}, 30*time.Second, stop)

	ctx := ControllerContext{
		ClientBuilder:                   clientBuilder,
		AdClient:                        adClient,
		InformerFactory:                 sharedInformers,
		ObjectOrMetadataInformerFactory: controller.NewInformerFactory(sharedInformers, metadataInformers),
		PixiuInformerFactory:            dInformers.NewSharedInformerFactory(adClient, time.Second+30),
		Stop:                            stop,
	}
	return ctx, nil
}

// StartControllers starts a set of controllers with a specified ControllerContext
func StartControllers(ctx ControllerContext, controllers map[string]InitFunc) error {
	for controllerName, intFn := range controllers {
		klog.V(0).Infof("Starting %q", controllerName)

		started, err := intFn(ctx)
		if err != nil {
			klog.Errorf("Starting %q failed", controllerName)
			return err
		}
		if !started {
			klog.Warningf("Skipping %q", controllerName)
			continue
		}
	}

	return nil
}

type InitFunc func(ctx ControllerContext) (enabled bool, err error)

// NewControllerInitializers is a public map of named controller groups
func NewControllerInitializers() map[string]InitFunc {
	controllers := map[string]InitFunc{}
	controllers["advancedDeployment"] = startPixiuController
	controllers["autoscaler"] = startAutoscalerController

	return controllers
}

func startPixiuController(ctx ControllerContext) (bool, error) {
	pc, err := advanceddeployment.NewPixiuController(
		ctx.AdClient,
		ctx.PixiuInformerFactory.Apps().V1alpha1().AdvancedDeployments(),
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ClientBuilder.ClientOrDie("shared-informers"),
	)
	if err != nil {
		return true, fmt.Errorf("New pixiu controller failed %v", err)
	}

	go pc.Run(workers, ctx.Stop)
	return true, nil
}

func startAutoscalerController(ctx ControllerContext) (bool, error) {

	return true, nil
}
