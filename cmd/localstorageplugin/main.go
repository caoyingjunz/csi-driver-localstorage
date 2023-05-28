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
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/signals"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"time"

	// import pprof for performance diagnosed
	_ "net/http/pprof"

	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/localstorage"
)

var (
	endpoint   = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", localstorage.DefaultDriverName, "name of the driver")
	nodeId     = flag.String("nodeid", "", "node id")
	// Deprecated： 临时使用，后续删除
	volumeDir = flag.String("volume-dir", "/tmp", "directory for storing state information across driver volumes")

	enablePprof = flag.Bool("enable-pprof", false, "Start pprof and gain leadership before executing the main loop")
	pprofPort   = flag.String("pprof-port", "6060", "The port of pprof to listen on")

	kubeconfig = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Needs to be set if the plugin is being run out of cluster.")
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

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()

	kubeConfig, err := util.BuildClientConfig(*kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build kube config: %v", err)
	}
	kubeConfig.QPS = 30000
	kubeConfig.Burst = 30000

	kubeClient, lsClientSet, err := util.NewClientSets(kubeConfig)
	if err != nil {
		klog.Fatal("failed to build clientSets: %v", err)
	}

	sharedInformer := externalversions.NewSharedInformerFactory(lsClientSet, 300*time.Second)

	cfg := localstorage.Config{
		DriverName:    *driverName,
		Endpoint:      *endpoint,
		VendorVersion: version,
		NodeId:        *nodeId,
		VolumeDir:     *volumeDir,
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

	driver, err := localstorage.NewLocalStorage(cfg)
	if err != nil {
		klog.Fatalf("Failed to initialize localstorage driver :%v", err)
	}

	if err = driver.Run(); err != nil {
		klog.Fatalf("Failed to run localstorage driver :%v", err)
	}
}
