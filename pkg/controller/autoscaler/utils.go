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
	"strings"

	apps "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	utilpointer "k8s.io/utils/pointer"
)

const (
	HorizontalPodAutoscaler string = "HorizontalPodAutoscaler"
	AutoscalingAPIVersion   string = "autoscaling/v2beta2"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// AutoscalerContext is responsible for kubernetes resources stored.
type AutoscalerContext struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	APIVersion  string            `json:"api_version"`
	Kind        string            `json:"kind"`
	UID         types.UID         `json:"uid"`
	Annotations map[string]string `json:"annotations"`
}

// NewAutoscalerContext extracts contexts which we needed from kubernetes resouces.
// The resouces could be Deployment, StatefulSet for now
func NewAutoscalerContext(obj interface{}) *AutoscalerContext {
	// TODO: 后续优化，直接获取 hpa 的 Annotations
	switch o := obj.(type) {
	case *apps.Deployment:
		return &AutoscalerContext{
			Name:        o.Name,
			Namespace:   o.Namespace,
			APIVersion:  o.APIVersion,
			Kind:        "Deployment",
			UID:         o.UID,
			Annotations: o.Annotations,
		}
	case *apps.StatefulSet:
		return &AutoscalerContext{
			Name:        o.Name,
			Namespace:   o.Namespace,
			APIVersion:  o.APIVersion,
			Kind:        "StatefulSet",
			UID:         o.UID,
			Annotations: o.Annotations,
		}
	default:
		// never happens
		return nil
	}
}

func CreateHorizontalPodAutoscaler(
	name string,
	namespace string,
	uid types.UID,
	apiVersion string,
	kind string,
	annotations map[string]string) (*autoscalingv2.HorizontalPodAutoscaler, error) {

	minReplicas, err := ExtractReplicas(annotations, MinReplicas)
	if err != nil {
		klog.Errorf("Extract MinReplicas from annotations failed: %v", err)
		return nil, err
	}
	maxReplicas, err := ExtractReplicas(annotations, MaxReplicas)
	if err != nil {
		klog.Errorf("Extract maxReplicas from annotations failed: %v", err)
		return nil, err
	}
	metrics, err := parseMetrics(annotations)
	if err != nil {
		klog.Errorf("Parse metrics from annotations failed: %v", err)
		return nil, fmt.Errorf("Parse metrics from annotations failed: %v", err)
	}
	klog.Infof("Parse %d metrics from annotations for %s/%s", len(metrics), namespace, name)

	controller := true
	blockOwnerDeletion := true
	// Inject ownerReference label
	ownerReference := metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               name,
		UID:                uid,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       HorizontalPodAutoscaler,
			APIVersion: AutoscalingAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				ownerReference,
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: utilpointer.Int32Ptr(minReplicas),
			MaxReplicas: maxReplicas,
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: apiVersion,
				Kind:       kind,
				Name:       name,
			},
			Metrics: metrics,
		},
	}

	return hpa, nil
}

func parseMetrics(annotations map[string]string) ([]autoscalingv2.MetricSpec, error) {
	metrics := make([]autoscalingv2.MetricSpec, 0)

	for metricName, metricValue := range annotations {
		if !strings.Contains(metricName, "."+KubezRootPrefix) {
			continue
		}
		metricNameSlice := strings.Split(metricName, KubezSeparator)
		if len(metricNameSlice) < 2 {
			continue
		}
		metricTypeSlice := strings.Split(metricName, ".")
		if len(metricTypeSlice) < 2 {
			continue
		}

		metricType := metricTypeSlice[0]
		if metricType != cpu && metricType != memory {
			return nil, fmt.Errorf("unsupprted metric resource name: %s", metricType)
		}

		switch metricNameSlice[len(metricNameSlice)-1] {
		case targetAverageUtilization:
			averageUtilization, err := ExtractAverageUtilization(metricValue)
			if err != nil {
				return nil, err
			}

			metric := autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: utilpointer.Int32Ptr(averageUtilization),
					},
				},
			}

			// TODO: To be optimised
			switch metricType {
			case cpu:
				metric.Resource.Name = v1.ResourceCPU
			case memory:
				metric.Resource.Name = v1.ResourceMemory
			}
			metrics = append(metrics, metric)
		case targetAverageValue:
			averageValue, err := resource.ParseQuantity(metricValue)
			if err != nil {
				return nil, err
			}

			metric := autoscalingv2.MetricSpec{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Target: autoscalingv2.MetricTarget{
						Type:         autoscalingv2.AverageValueMetricType,
						AverageValue: &averageValue,
					},
				},
			}

			switch metricType {
			case cpu:
				metric.Resource.Name = v1.ResourceCPU
			case memory:
				metric.Resource.Name = v1.ResourceMemory
			}
			metrics = append(metrics, metric)
		}
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("could't parse metrics, the numbers is zero")
	}

	return metrics, nil
}

func IsOwnerReference(uid types.UID, ownerReferences []metav1.OwnerReference) bool {
	var isOwnerRef bool
	for _, ownerReferences := range ownerReferences {
		if uid == ownerReferences.UID {
			isOwnerRef = true
			break
		}
	}
	return isOwnerRef
}

func ManagerByKubezController(hpa *autoscalingv2.HorizontalPodAutoscaler) bool {
	for _, managedField := range hpa.ManagedFields {
		if managedField.APIVersion == AutoscalingAPIVersion &&
			(managedField.Manager == KubezManager ||
				// This condition used for local run
				managedField.Manager == KubezMain) {
			return true
		}
	}
	return false
}
