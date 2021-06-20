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
	"context"
	"flag"
	"os"
	"time"
	
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	dClientset "github.com/caoyingjunz/pixiu/pkg/generated/clientset/versioned"
	informers "github.com/caoyingjunz/pixiu/pkg/generated/informers/externalversions"
	"github.com/caoyingjunz/pixiu/cmd/pixiu-controller-manager/app"
	"github.com/caoyingjunz/pixiu/pkg/controller"
	"github.com/caoyingjunz/pixiu/pkg/controller/advanceddeployment"
	"github.com/caoyingjunz/pixiu/pkg/signals"
)

const (
	workers = 5

	HealthzHost = "127.0.0.1"
	HealthzPort = "10256"

	LeaseDuration                   = 15
	RenewDeadline                   = 10
	RetryPeriod                     = 2
	ResourceLock                    = "endpointsleases"
	ResourceName                    = "kubez-autoscaler-manager"
	ResourceNamespace               = "kube-system"
	LeaderElect                     = true
	PixiuControllerManagerUserAgent = "pixiu-controller-manager"
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
	clientBuilder := controller.SimpleControllerClientBuilder{
		ClientConfig: clientConfig,
	}
	clientSet, err := dClientset.NewForConfig(clientConfig)
	if err != nil {
		klog.Fatalf("Error building pixiu clientset: %s", err)
	}
	run := func(ctx context.Context) {
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
		go app.StartHealthzServer(healthzHost, healthzPort)

		// always wait
		select {}
	}

	if !LeaderElect {
		run(context.TODO())
		panic("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("get hostname failed %v", err)
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())
	leaderClient := clientBuilder.ClientOrDie("leader-client")
	eventRecorder := app.CreateRecorder(leaderClient, PixiuControllerManagerUserAgent)
	rl, err := resourcelock.New(
		resourceLock,
		resourceNamespace,
		resourceName,
		leaderClient.CoreV1(),
		leaderClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: eventRecorder,
		})
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: time.Duration(leaseDuration) * time.Second,
		RenewDeadline: time.Duration(renewDeadline) * time.Second,
		RetryPeriod:   time.Duration(retryPeriod) * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
		//WatchDog: electionChecker,
		Name: PixiuControllerManagerUserAgent,
	})
	panic("unreachable")
}

var (
	// Path to a kubeconfig. Only required if out-of-cluster
	kubeconfig        string
	healthzHost       string
	healthzPort       string
	leaderElect       bool
	leaseDuration     int
	renewDeadline     int
	retryPeriod       int
	resourceLock      string
	resourceName      string
	resourceNamespace string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&healthzHost, "healthz-host", HealthzHost, "The host of Healthz.")
	flag.StringVar(&healthzPort, "healthz-port", HealthzPort, "The port of Healthz to listen on.")
	flag.BoolVar(&leaderElect, "leader-elect", LeaderElect, "Leader election switch")
	flag.IntVar(&leaseDuration, "leader-elect-lease-duration", LeaseDuration, "Lease time.")
	flag.IntVar(&renewDeadline, "leader-elect-renew-deadline", RenewDeadline, "Renewal of lease.")
	flag.IntVar(&retryPeriod, "leader-elect-retry-period", RetryPeriod, "Non-leader node retry time.")
	flag.StringVar(&resourceLock, "leader-elect-resource-lock", ResourceLock, "The type of resource object that is used for locking during leader election. Supported options are `endpoints` (default) and `configmaps`.")
	flag.StringVar(&resourceName, "leader-elect-resource-name", ResourceName, "The name of resource object that is used for locking during leader election.")
	flag.StringVar(&resourceNamespace, "leader-elect-resource-namespace", ResourceNamespace, "The namespace of resource object that is used for locking during leader election.")
}
