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
	"fmt"

	"k8s.io/klog/v2"

	extenderv1 "k8s.io/kube-scheduler/extender/v1"
)

type Predicate struct{}

func NewPredicate() *Predicate {
	return &Predicate{}
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
