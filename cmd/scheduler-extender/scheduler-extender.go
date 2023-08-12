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
	"strconv"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/scheduler"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "paths to a kubeconfig. Only required if out-of-cluster.")
	port       = flag.Int("port", 8090, "port is the port that the scheduler server serves at")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	kubeConfig, err := util.BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}
	kubeClient, lsClientSet, err := util.NewClientSets(kubeConfig)
	if err != nil {
		klog.Fatalf("Failed to build clientSets: %v", err)
	}

	kubeInformer := informers.NewSharedInformerFactory(kubeClient, 300*time.Second)
	shareInformer := externalversions.NewSharedInformerFactory(lsClientSet, 300*time.Second)

	sched, err := scheduler.NewScheduleExtender(
		shareInformer.Storage().V1().LocalStorages(),
		kubeInformer.Core().V1().PersistentVolumeClaims(),
		kubeInformer.Storage().V1().StorageClasses(),
	)
	if err != nil {
		klog.Fatalf("Failed to new schedule extender controller: %s", err)
	}

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	// Start all informers.
	kubeInformer.Start(ctx.Done())
	shareInformer.Start(ctx.Done())
	// Wait for all caches to sync.
	shareInformer.WaitForCacheSync(ctx.Done())
	kubeInformer.WaitForCacheSync(ctx.Done())

	// Start localstorage scheduler extender server
	if err = sched.Run(ctx, ":"+strconv.Itoa(*port)); err != nil {
		klog.Fatalf("failed to start localstorage scheduler extender server: %v", err)
	}
}
