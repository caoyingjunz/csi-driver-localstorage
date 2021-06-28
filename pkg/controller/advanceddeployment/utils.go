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

package advanceddeployment

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	podutil "github.com/caoyingjunz/pixiu/pkg/api/v1/pod"
	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
	appsClient "github.com/caoyingjunz/pixiu/pkg/client/clientset/versioned/typed/apps/v1alpha1"
)

func calculateStatus(ad *appsv1alpha1.AdvancedDeployment, pods []*v1.Pod, manageAdErr error) appsv1alpha1.AdvancedDeploymentStatus {
	newStatus := ad.Status

	Replicas := *ad.Spec.Replicas

	var readyReplicas, availableReplicas int32
	for _, pod := range pods {
		if podutil.IsPodReady(pod) {
			readyReplicas++
		}
		if podutil.IsPodAvailable(pod, ad.Spec.MinReadySeconds, metav1.Now()) {
			availableReplicas++
		}

	}
	newStatus.Replicas = Replicas
	newStatus.ReadyReplicas = readyReplicas
	newStatus.AvailableReplicas = availableReplicas
	newStatus.UnavailableReplicas = Replicas - availableReplicas

	return newStatus
}

// updateReplicaSetStatus attempts to update the Status.Replicas of the given ReplicaSet, with a single GET/PUT retry.
func updateAdvancedDeploymentStatus(c appsClient.AdvancedDeploymentInterface, ad *appsv1alpha1.AdvancedDeployment, newStatus appsv1alpha1.AdvancedDeploymentStatus) (*appsv1alpha1.AdvancedDeployment, error) {
	// This is the steady state. It happens when the ReplicaSet doesn't have any expectations, since
	// we do a periodic relist every 30s. If the generations differ but the replicas are
	// the same, a caller might've resized to the same replica count.
	if ad.Status.Replicas == newStatus.Replicas &&
		ad.Status.ReadyReplicas == newStatus.ReadyReplicas &&
		ad.Status.AvailableReplicas == newStatus.AvailableReplicas &&
		ad.Generation == ad.Status.ObservedGeneration &&
		reflect.DeepEqual(ad.Status.Conditions, newStatus.Conditions) {
		return ad, nil
	}

	// Save the generation number we acted on, otherwise we might wrongfully indicate
	// that we've seen a spec update when we retry.
	// TODO: This can clobber an update if we allow multiple agents to write to the
	// same status.
	newStatus.ObservedGeneration = ad.Generation

	var getErr, updateErr error
	var updatedAD *appsv1alpha1.AdvancedDeployment
	for i, ad := 0, ad; ; i++ {
		klog.V(4).Infof(fmt.Sprintf("Updating status for %v: %s/%s, ", ad.Kind, ad.Namespace, ad.Name) +
			fmt.Sprintf("replicas %d->%d (need %d), ", ad.Status.Replicas, newStatus.Replicas, *(ad.Spec.Replicas)) +
			fmt.Sprintf("readyReplicas %d->%d, ", ad.Status.ReadyReplicas, newStatus.ReadyReplicas) +
			fmt.Sprintf("availableReplicas %d->%d, ", ad.Status.AvailableReplicas, newStatus.AvailableReplicas) +
			fmt.Sprintf("sequence No: %v->%v", ad.Status.ObservedGeneration, newStatus.ObservedGeneration))

		ad.Status = newStatus
		updatedAD, updateErr = c.UpdateStatus(context.TODO(), ad, metav1.UpdateOptions{})
		if updateErr == nil {
			return updatedAD, nil
		}
		// Stop retrying if we exceed statusUpdateRetries - the replicaSet will be requeued with a rate limit.
		if i >= statusUpdateRetries {
			break
		}
		// Update the ReplicaSet with the latest resource version for the next poll
		if ad, getErr = c.Get(context.TODO(), ad.Name, metav1.GetOptions{}); getErr != nil {
			// If the GET fails we can't trust status.Replicas anymore. This error
			// is bound to be more interesting than the update failure.
			return nil, getErr
		}
	}

	return nil, updateErr
}
