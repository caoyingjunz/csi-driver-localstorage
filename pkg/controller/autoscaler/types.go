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

package autoscaler

import (
	"fmt"
	"strconv"
)

const (
	KubezManager    string = "pixiu-autoscaler-controller"
	KubezMain       string = "main"
	KubezRootPrefix string = "hpa.caoyingjunz.io"
	KubezSeparator  string = "/"

	MinReplicas              string = "hpa.caoyingjunz.io/minReplicas"
	MaxReplicas              string = "hpa.caoyingjunz.io/maxReplicas"
	targetAverageUtilization string = "targetAverageUtilization"
	targetAverageValue       string = "targetAverageValue"

	// TODO: prometheus is not supported for now.
	cpu        string = "cpu"
	memory     string = "memory"
	prometheus string = "prometheus"

	cpuAverageUtilization        = "cpu." + KubezRootPrefix + KubezSeparator + targetAverageUtilization
	memoryAverageUtilization     = "memory." + KubezRootPrefix + KubezSeparator + targetAverageUtilization
	prometheusAverageUtilization = "prometheus." + KubezRootPrefix + KubezSeparator + targetAverageUtilization

	// CPU, in cores. (500m = .5 cores)
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	cpuAverageValue        = "cpu." + KubezRootPrefix + KubezSeparator + targetAverageValue
	memoryAverageValue     = "memory." + KubezRootPrefix + KubezSeparator + targetAverageValue
	prometheusAverageValue = "prometheus." + KubezRootPrefix + KubezSeparator + targetAverageValue
)

var (
	// Init SafeSet which contains the HPA Average Utilization / Value
	kset = NewSafeSet(cpuAverageUtilization, memoryAverageUtilization, cpuAverageValue, memoryAverageValue)
)

// To ensure whether we need to maintain the HPA
func IsNeedForHPAs(annotations map[string]string) bool {
	if annotations == nil || len(annotations) == 0 {
		return false
	}

	for aKey := range annotations {
		if kset.Has(aKey) {
			return true
		}
	}

	return false
}

func ExtractReplicas(annotations map[string]string, replicasType string) (int32, error) {
	var Replicas int64
	var err error
	switch replicasType {
	case MinReplicas:
		minReplicas, exists := annotations[MinReplicas]
		if exists {
			Replicas, err = strconv.ParseInt(minReplicas, 10, 32)
			if err != nil {
				return 0, err
			}
		} else {
			// Default minReplicas is 1
			Replicas = int64(1)
		}
	case MaxReplicas:
		maxReplicas, exists := annotations[MaxReplicas]
		if !exists {
			return 0, fmt.Errorf("%s is required", MaxReplicas)
		}
		Replicas, err = strconv.ParseInt(maxReplicas, 10, 32)
	}

	return int32(Replicas), err
}

func ExtractAverageUtilization(averageUtilization string) (int32, error) {
	value64, err := strconv.ParseInt(averageUtilization, 10, 32)
	if err != nil {
		return 0, err
	}
	if value64 <= 0 && value64 > 100 {
		return 0, fmt.Errorf("averageUtilization should be range 1 between 100")
	}

	return int32(value64), nil
}
