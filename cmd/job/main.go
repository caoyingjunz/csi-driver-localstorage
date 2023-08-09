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

var (
	kubeconfig = flag.String("kubeconfig", "", "paths to a kubeconfig. Only required if out-of-cluster.")
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

}