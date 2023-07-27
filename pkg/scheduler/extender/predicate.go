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

	v1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	storageutil "github.com/caoyingjunz/csi-driver-localstorage/pkg/util/storage"
)

type Predicate struct {
	lsLister  localstorage.LocalStorageLister
	pvcLister corelisters.PersistentVolumeClaimLister
	scLister  storagelisters.StorageClassLister
}

func NewPredicate(lsLister localstorage.LocalStorageLister, pvcLister corelisters.PersistentVolumeClaimLister, scLister storagelisters.StorageClassLister) *Predicate {
	return &Predicate{lsLister: lsLister, pvcLister: pvcLister, scLister: scLister}
}

func (p *Predicate) Filter(args extenderv1.ExtenderArgs) *extenderv1.ExtenderFilterResult {
	pod := args.Pod
	if pod == nil {
		return &extenderv1.ExtenderFilterResult{Error: fmt.Sprintf("pod is nil")}
	}

	pvc, err := storageutil.GetLocalStoragePersistentVolumeClaimFromPod(pod, p.pvcLister, p.scLister)
	if err != nil {
		return &extenderv1.ExtenderFilterResult{Error: err.Error()}
	}
	if pvc == nil {
		klog.Infof("ignore filter namespace(%s) name(%s)", pod.Namespace, pod.Name)
		return &extenderv1.ExtenderFilterResult{
			NodeNames: args.NodeNames,
			Nodes:     args.Nodes,
		}
	}
	request, exists := pvc.Spec.Resources.Requests[v1.ResourceStorage]
	if !exists {
		return &extenderv1.ExtenderFilterResult{Error: fmt.Sprintf("failed to find pvc %s request quantity", pvc.Name)}
	}

	klog.Infof("starting filter namespace(%s) name(%s)", pod.Namespace, pod.Name)
	localstorageMap, err := storageutil.GetLocalStorageMap(p.lsLister)
	if err != nil {
		return &extenderv1.ExtenderFilterResult{Error: err.Error()}
	}

	scheduleNodes := make([]string, 0)
	failedNodes := make(map[string]string)
	for _, nodeName := range *args.NodeNames {
		ls, found := localstorageMap[nodeName]
		if !found {
			failedNodes[nodeName] = fmt.Sprintf("node(%s) has no localstorage", nodeName)
			continue
		}
		if ls.Status.Phase != localstoragev1.LocalStorageReady {
			failedNodes[nodeName] = fmt.Sprintf("node(%s) localstorage not ready", nodeName)
			continue
		}
		allocSize := ls.Status.Allocatable
		if request.Cmp(*allocSize) > 0 {
			failedNodes[nodeName] = fmt.Sprintf("node(%s) localstorage allocatable size too small", nodeName)
			continue
		}

		scheduleNodes = append(scheduleNodes, nodeName)
	}

	klog.Infof("filter localstorage pods on nodes: %v", scheduleNodes)
	return &extenderv1.ExtenderFilterResult{
		NodeNames:   &scheduleNodes,
		Nodes:       args.Nodes,
		FailedNodes: failedNodes,
	}
}
