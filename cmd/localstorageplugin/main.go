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
	"k8s.io/klog/v2"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/localstorage"
)

var (
	endpoint   = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", localstorage.DefaultDriverName, "name of the driver")
	nodeId     = flag.String("nodeid", "", "node id")

	// Deprecated： 临时使用，后续删除
	volumeDir = flag.String("volume-dir", "/tmp", "directory for storing state information across driver volumes")
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

	driver, err := localstorage.NewLocalStorage(cfg)
	if err != nil {
		klog.Fatalf("Failed to initialize localstorage driver :%v", err)
	}

	if err = driver.Run(); err != nil {
		klog.Fatalf("Failed to run localstorage driver :%v", err)
	}
}
