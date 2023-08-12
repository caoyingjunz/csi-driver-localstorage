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
	"os"
	"time"
	// import pprof for performance diagnosed
	_ "net/http/pprof"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/localstorage"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

var (
	endpoint   = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", localstorage.DefaultDriverName, "name of the driver")
	nodeId     = flag.String("nodeid", "", "node id")
	volumeDir  = flag.String("volume-dir", "/tmp", "directory for storing state information across driver volumes")

	kubeconfig   = flag.String("kubeconfig", "", "paths to a kubeconfig. Only required if out-of-cluster.")
	kubeAPIQPS   = flag.Int("kube-api-qps", 5, "QPS to use while communicating with the kubernetes apiserver. Defaults to 5")
	kubeAPIBurst = flag.Int("kube-api-burst", 10, "Burst to use while communicating with the kubernetes apiserver. Defaults to 10.")

	// pprof flags
	enablePprof = flag.Bool("enable-pprof", false, "Start pprof and gain leadership before executing the main loop")
	pprofPort   = flag.String("pprof-port", "6060", "The port of pprof to listen on")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

var (
	version = "v1.0.0"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	cfg := localstorage.Config{
		DriverName:    *driverName,
		Endpoint:      *endpoint,
		VendorVersion: version,
		NodeId:        *nodeId,
		VolumeDir:     *volumeDir,
	}
	if len(cfg.NodeId) == 0 {
		klog.V(2).Infof("Get node name from env")
		cfg.NodeId = os.Getenv("NODE_NAME")
	}

	// Start pprof and gain leadership before executing the main loop
	if *enablePprof {
		go func() {
			klog.Infof("Starting the pprof server on: %s", *pprofPort)
			if err := http.ListenAndServe(":"+*pprofPort, nil); err != nil {
				klog.Fatalf("Failed to start pprof server: %v", err)
			}
		}()
	}

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	kubeConfig, err := util.BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}
	kubeConfig.QPS = float32(*kubeAPIQPS)
	kubeConfig.Burst = *kubeAPIBurst

	kubeClient, lsClientSet, err := util.NewClientSets(kubeConfig)
	if err != nil {
		klog.Fatal("failed to build clientSets: %v", err)
	}

	sharedInformer := externalversions.NewSharedInformerFactory(lsClientSet, 300*time.Second)
	driver, err := localstorage.NewLocalStorage(ctx, cfg,
		sharedInformer.Storage().V1().LocalStorages(),
		lsClientSet,
		kubeClient,
	)
	if err != nil {
		klog.Fatalf("Failed to initialize localstorage driver :%v", err)
	}

	go func() {
		klog.Infof("Starting localstorage driver")
		if err = driver.Run(ctx); err != nil {
			klog.Fatalf("Failed to run localstorage driver :%v", err)
		}
	}()

	sharedInformer.Start(ctx.Done())
	sharedInformer.WaitForCacheSync(ctx.Done())

	<-ctx.Done()
}
