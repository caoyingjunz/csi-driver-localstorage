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

package scheduler

import (
	"k8s.io/klog/v2"
	"math/rand"

	schedulerextender "k8s.io/kube-scheduler/extender/v1"
)

type Prioritize struct {
}

func (p *Prioritize) Handler(args schedulerextender.ExtenderArgs) *schedulerextender.HostPriorityList {
	nodes := args.Nodes.Items

	hostPriorityList := make(schedulerextender.HostPriorityList, len(nodes))
	for i, node := range nodes {
		score := rand.Int63n(schedulerextender.MaxExtenderPriority + 1)
		hostPriorityList[i] = schedulerextender.HostPriority{
			Host:  node.Name,
			Score: score,
		}
	}

	klog.Infof("TODO: hostPriorityList: %+v", hostPriorityList)
	return &hostPriorityList
}
