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

package extender

import (
	"fmt"

	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"

	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"
)

type Predicate struct {
	lsLister  localstorage.LocalStorageLister
	pvcLister corelisters.PersistentVolumeClaimLister
	scLister  storagelisters.StorageClassLister
}

func NewPredicate(
	lsLister localstorage.LocalStorageLister,
	pvcLister corelisters.PersistentVolumeClaimLister,
	scLister storagelisters.StorageClassLister) *Predicate {
	return &Predicate{
		lsLister:  lsLister,
		pvcLister: pvcLister,
		scLister:  scLister,
	}
}

func (p *Predicate) Handler(args extenderv1.ExtenderArgs) *extenderv1.ExtenderFilterResult {
	pod := args.Pod
	if pod == nil {
		return &extenderv1.ExtenderFilterResult{Error: fmt.Sprintf("localstorage pod is nil")}
	}

	klog.Infof("TODO: ExtenderFilterResult: %+v", pod)
	return &extenderv1.ExtenderFilterResult{
		NodeNames:   args.NodeNames,
		Nodes:       args.Nodes,
		FailedNodes: make(map[string]string),
		Error:       "",
	}
}
