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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/controller/storage"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

const (
	workers = 5

	LeaseDuration = 15
	RenewDeadline = 10
	RetryPeriod   = 2

	ResourceLock      = "endpointsleases"
	ResourceName      = "localstorage-manager"
	ResourceNamespace = "kube-system"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")

	// leaderElect
	leaderElect       = flag.Bool("leader-elect", true, "Start a leader election client and gain leadership before executing the main loop. Enable this when running replicated components for high availability.")
	retryPeriod       = flag.Int("leader-elect-retry-period", RetryPeriod, "The duration the clients should wait between attempting acquisition and renewal of a leadership.")
	resourceLock      = flag.String("leader-elect-resource-lock", ResourceLock, "The type of resource object that is used for locking during leader election. Supported options are `endpoints` (default) and `configmaps`.")
	resourceName      = flag.String("leader-elect-resource-name", ResourceName, "The name of resource object that is used for locking during leader election.")
	resourceNamespace = flag.String("leader-elect-resource-namespace", ResourceNamespace, "The namespace of resource object that is used for locking during leader election.")
	leaseDuration     = flag.Int("leader-elect-lease-duration", LeaseDuration, "The duration that non-leader candidates will wait")
	renewDeadline     = flag.Int("leader-elect-renew-deadline", RenewDeadline, "The interval between attempts by the acting master to renew a leadership slot before it stops leading.")
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
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to build kube clientSet: %v", err)
	}

	run := func(ctx context.Context) {
		lsClientSet, err := versioned.NewForConfig(kubeConfig)
		if err != nil {
			klog.Fatalf("Failed to new localstorage clientSet: %v", err)
		}

		sharedInformer := externalversions.NewSharedInformerFactory(lsClientSet, time.Second)
		sc, err := storage.NewStorageController(ctx,
			sharedInformer.Storage().V1().LocalStorages(),
			lsClientSet,
			kubeClient,
		)
		if err != nil {
			klog.Fatalf("Failed to new storage controller: %s", err)
		}

		klog.Infof("Starting localstorage controller")
		go sc.Run(ctx, workers)

		sharedInformer.Start(ctx.Done())
		sharedInformer.WaitForCacheSync(ctx.Done())

		// always wait
		select {}
	}

	if !*leaderElect {
		run(ctx)
		klog.Fatalf("unreachable")
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Failed to get hostname: %v", err)
	}
	// add a uniquifier so that two processes on the same host don't accidentally both become active
	id = id + "_" + string(uuid.NewUUID())

	rl, err := resourcelock.New(
		*resourceLock,
		*resourceNamespace,
		*resourceName,
		kubeClient.CoreV1(),
		kubeClient.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: util.CreateRecorder(kubeClient),
		})
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: time.Duration(*leaseDuration) * time.Second,
		RenewDeadline: time.Duration(*renewDeadline) * time.Second,
		RetryPeriod:   time.Duration(*retryPeriod) * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leader election lost")
			},
		},
		//WatchDog: electionChecker,
		Name: "localstorage-manager",
	})

	klog.Fatalf("unreachable")
}
