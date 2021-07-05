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

package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

	appsv1alpha1 "github.com/caoyingjunz/pixiu/pkg/apis/apps/v1alpha1"
)

const (
	advancedImage      = "AdvancedImage"
	advancedDeployment = "AdvancedDeployment"
	imageSet           = "ImageSet"
	Pull               = "pull"
	Remove             = "remove"
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()

	ignoredNamespaces = []string{metav1.NamespaceSystem, metav1.NamespacePublic}

	AvailableActions = map[string]bool{
		Pull:	true,
		Remove:	true,
	}
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func admissionRequired(ignoredList []string, metadata *metav1.ObjectMeta) bool {
	// skip special kubernetes system namespaces
	for _, namespace := range ignoredList {
		if metadata.Namespace == namespace {
			klog.Infof("Skip validation for %v for it's in special namespace:%v", metadata.Name, metadata.Namespace)
			return false
		}
	}

	// TODO: 自定义检查实现
	return false
}

// TODO
func doMutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	return nil
}

// Do validate Pixiu resources
func doValidate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request

	var objectMeta *metav1.ObjectMeta
	var resourceNamespace, resourceName string
	klog.V(4).Infof("Will validate for Kind=%v, Namespace=%v Name=%v ResourceName=%v UID=%v Operation=%v UserInfo=%v",
		req.Kind, req.Namespace, req.Name, resourceName, req.UID, req.Operation, req.UserInfo)

	var err error
	switch req.Kind.Kind {
	case advancedImage:
		var ai appsv1alpha1.AdvancedImage
		if err = json.Unmarshal(req.Object.Raw, &ai); err != nil {
			klog.Errorf("Could not unmarshal raw object: %v", err)
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = ai.Name, ai.Namespace, &ai.ObjectMeta
	case imageSet:
		var is appsv1alpha1.ImageSet
		if err = json.Unmarshal(req.Object.Raw, &is); err != nil {
			klog.Errorf("Could not unmarshal raw object: %v", err)
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		resourceName, resourceNamespace, objectMeta = is.Name, is.Namespace, &is.ObjectMeta

		if AvailableActions[is.Spec.Action] {
			return &v1beta1.AdmissionResponse{
				Allowed: true,
				Result: &metav1.Status{},
			}
		} else {
			return &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: "action is pull or remove",
				},
			}
		}
	default:
		// This case will not happened
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: "unsupported Pixiu Kind",
			},
		}
	}
	klog.Infof("debug resourceName: %v, resourceNamespace: %v, objectMeta: %v", resourceName, resourceNamespace, objectMeta)

	allowed := true
	var status *metav1.Status
	if objectMeta != nil {
		anno := objectMeta.GetAnnotations()
		if anno != nil {
			if _, exists := anno["allForbid"]; exists {
				allowed = false
				status = &metav1.Status{
					Reason: "Debug, allForbid is set",
					Code:   http.StatusMethodNotAllowed,
				}
			}
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: allowed,
		Result:  status,
	}
}

func verifyAndParseRequest(r *http.Request) (*v1beta1.AdmissionReview, error) {
	var body []byte
	var err error

	body, err = ioutil.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("Content-Type=%s, expect application/json", contentType)
	}

	ar := v1beta1.AdmissionReview{}
	if _, _, err = universalDeserializer.Decode(body, nil, &ar); err != nil {
		return nil, fmt.Errorf("can't decode admissionReview from body: %v", err)
	}

	return &ar, nil
}

// TODO: will complated the HandlerMutate in the future
func HandlerMutate(w http.ResponseWriter, r *http.Request) {
	klog.Infof("Do nothing for HandlerMutate for now")
	return
}

// Hander validate for Pixiu webhook server
func HandlerValidate(w http.ResponseWriter, r *http.Request) {
	ar, err := verifyAndParseRequest(r)
	if err != nil {
		klog.Errorf("verify or parse the request failed: %v", err)
		http.Error(w, fmt.Sprintf("verify or parse the request failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Do validate
	adResponse := doValidate(ar)
	adReview := v1beta1.AdmissionReview{}
	if adResponse != nil {
		adReview.Response = adResponse
		if ar.Request != nil {
			adReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(adReview)
	if err != nil {
		klog.Errorf("Can't marshal AdmissionReview: %v", err)
		http.Error(w, fmt.Sprintf("Can't marshal AdmissionReview: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err = w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
