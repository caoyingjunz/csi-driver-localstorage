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
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	storageutil "github.com/caoyingjunz/csi-driver-localstorage/pkg/util/storage"
)

type Prioritize struct {
	lsLister  localstorage.LocalStorageLister
	pvcLister corelisters.PersistentVolumeClaimLister
	scLister  storagelisters.StorageClassLister
}

func NewPrioritize(lsLister localstorage.LocalStorageLister, pvcLister corelisters.PersistentVolumeClaimLister, scLister storagelisters.StorageClassLister) *Prioritize {
	return &Prioritize{lsLister: lsLister, pvcLister: pvcLister, scLister: scLister}
}

func (p *Prioritize) Score(args extenderv1.ExtenderArgs) *extenderv1.HostPriorityList {
	pod := args.Pod
	if pod == nil {
		klog.Errorf("pod is nil")
		return nil
	}
	used, err := storageutil.PodIsUseLocalStorage(pod, p.pvcLister, p.scLister)
	if err != nil || !used {
		klog.Errorf("pod not use localstorage or err")
		return nil
	}

	nodeNames := *args.NodeNames
	klog.Infof("scoring nodes %v", nodeNames)

	hostPriorityList := make(extenderv1.HostPriorityList, len(nodeNames))
	lsMap, err := storageutil.GetLocalStorageMap(p.lsLister)
	if err != nil {
		klog.Errorf("failed to get localstorage node map: $v", err)
		return nil
	}

	for i, nodeName := range nodeNames {
		var score int64
		ls, found := lsMap[nodeName]
		if found {
			score = p.score(ls)
			klog.Infof("scoring node(%s) with score(%d)", nodeName, score)
		}

		hostPriorityList[i] = extenderv1.HostPriority{
			Host:  nodeName,
			Score: score,
		}
	}

	klog.Infof("score localstorage pods on nodes: %v", hostPriorityList)
	return &hostPriorityList
}

func (p *Prioritize) score(ls *localstoragev1.LocalStorage) int64 {
	localstorage := ls.DeepCopy()

	allocatable := localstorage.Status.Allocatable
	capacity := localstorage.Status.Capacity

	// TODO optimise score algorithm
	// 临时处理，后续优化
	score := 100 * allocatable.AsApproximateFloat64() / capacity.AsApproximateFloat64()
	klog.V(2).Infof("ls %s get %v score", ls.Name, score)
	return int64(score)
}
