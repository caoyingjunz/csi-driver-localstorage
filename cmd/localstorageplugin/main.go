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
	"flag"
	"k8s.io/klog/v2"
)

const (
	DefaultDriverName = "localstorage.csi.pixiu.io"
)

var (
	endpoint   = flag.String("endpoint", "unix://tmp/csi.sock", "CSI endpoint")
	driverName = flag.String("drivername", DefaultDriverName, "name of the driver")
)

func init() {
	_ = flag.Set("logtostderr", "true")
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Info("starting")
	klog.V(2).Info("v2 starting")
}
