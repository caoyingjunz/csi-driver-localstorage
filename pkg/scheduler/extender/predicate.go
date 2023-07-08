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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	ls "github.com/caoyingjunz/csi-driver-localstorage/pkg/localstorage"
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

func (p *Predicate) Filter(args extenderv1.ExtenderArgs) *extenderv1.ExtenderFilterResult {
	pod := args.Pod
	if pod == nil {
		return &extenderv1.ExtenderFilterResult{Error: fmt.Sprintf("pod is nil")}
	}

	used, reqStorage, err := p.getUsedLocalstorage(pod)
	if err != nil {
		return &extenderv1.ExtenderFilterResult{Error: err.Error()}
	}
	if !used {
		klog.Infof("namespace(%s) pod(%s) don't use localstorage, ignore", pod.Namespace, pod.Name)
		return &extenderv1.ExtenderFilterResult{
			NodeNames: args.NodeNames,
			Nodes:     args.Nodes,
		}
	}

	klog.Infof("namespace(%s) pod(%s) use localstorage, try to schedule it", pod.Namespace, pod.Name)
	lsNodes, err := p.lsLister.List(labels.Everything())
	if err != nil {
		return &extenderv1.ExtenderFilterResult{Error: err.Error()}
	}
	localstorageMap := make(map[string]localstoragev1.LocalStorage)
	for _, lsNode := range lsNodes {
		localstorageMap[lsNode.Spec.Node] = *lsNode
	}

	scheduleNodes := make([]string, 0)
	failedNodes := make(map[string]string)
	for _, nodeName := range *args.NodeNames {
		ls, found := localstorageMap[nodeName]
		if !found {
			failedNodes[nodeName] = fmt.Sprintf("pod(%s) can not scheduled on node(%s) because it has no localstorage", pod.Name, nodeName)
			continue
		}
		if ls.Status.Phase != localstoragev1.LocalStorageReady {
			failedNodes[nodeName] = fmt.Sprintf("pod(%s) can not scheduled on node(%s) because it localstorage not ready", pod.Name, nodeName)
			continue
		}
		allocSize := ls.Status.Allocatable
		if reqStorage.Cmp(*allocSize) > 0 {
			failedNodes[nodeName] = fmt.Sprintf("pod(%s) can not scheduled on node(%s) because it localstorage Allocatable size too small", pod.Name, nodeName)
			continue
		}

		scheduleNodes = append(scheduleNodes, nodeName)
	}

	klog.Infof("schedule localstorage pods on nodes: %v", scheduleNodes)
	return &extenderv1.ExtenderFilterResult{
		NodeNames:   &scheduleNodes,
		Nodes:       args.Nodes,
		FailedNodes: failedNodes,
	}
}

func (p *Predicate) getUsedLocalstorage(pod *v1.Pod) (bool, *resource.Quantity, error) {
	volumes := pod.Spec.Volumes
	if volumes == nil || len(volumes) == 0 {
		return false, nil, nil
	}

	for _, volume := range volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}

		claimName := volume.PersistentVolumeClaim.ClaimName
		// get pvc from indexer
		pvc, err := p.pvcLister.PersistentVolumeClaims(pod.Namespace).Get(claimName)
		if err != nil {
			return false, nil, fmt.Errorf("failed to get pvc %s from indexer: %v", claimName, err)
		}

		storageClassName := pvc.Spec.StorageClassName
		if storageClassName == nil || len(*storageClassName) == 0 {
			continue
		}
		sc, err := p.scLister.Get(*storageClassName)
		if err != nil {
			return false, nil, fmt.Errorf("failed to get sc %s from indexer: %v", *storageClassName, err)
		}

		if sc.Provisioner == ls.DefaultDriverName {
			request, found := pvc.Spec.Resources.Requests[v1.ResourceStorage]
			if !found {
				return false, nil, fmt.Errorf("failed to get pvc %s request size from indexer: %v", claimName, err)
			}

			return true, &request, nil
		}
	}

	return false, nil, nil
}
