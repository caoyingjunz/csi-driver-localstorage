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
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util/router"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "paths to a kubeconfig. Only required if out-of-cluster.")

	port = flag.Int("port", 8090, "port is the port that the scheduler server serves at")
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
	_, lsClientSet, err := util.NewClientSets(kubeConfig)
	if err != nil {
		klog.Fatal("failed to build clientSets: %v", err)
	}

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	lsInformer := externalversions.NewSharedInformerFactory(lsClientSet, 300*time.Second)
	go func() {
		// new http router
		scheduleRoute := httprouter.New()

		// Install scheduler extender http router
		router.InstallHttpRouteWithInformer(ctx, scheduleRoute, lsInformer.Storage().V1().LocalStorages())

		klog.Infof("starting localstorage scheduler extender server")
		if err = http.ListenAndServe(":"+strconv.Itoa(*port), scheduleRoute); err != nil {
			klog.Fatalf("failed to start localstorage scheduler extender server: %v", err)
		}
	}()

	// Start ls informers.
	lsInformer.Start(ctx.Done())
	// Wait for ls caches to sync.
	lsInformer.WaitForCacheSync(ctx.Done())

	<-ctx.Done()
}
