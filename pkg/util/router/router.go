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

package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"k8s.io/klog/v2"
	"net/http"

	"github.com/julienschmidt/httprouter"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/scheduler"
)

const (
	version     = "v1.0.1"
	versionPath = "/version"

	apiPrefix        = "/localstorage-scheduler"
	predicatePrefix  = apiPrefix + "/filter"
	prioritizePrefix = apiPrefix + "/prioritize"
)

func InstallRouters(route *httprouter.Router) {
	route.GET(versionPath, handleVersion)
	route.POST(predicatePrefix, handlePredicate)
	route.POST(prioritizePrefix, handlePrioritize)
}

func handleVersion(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	fmt.Fprint(resp, fmt.Sprint(version))
}

func handlePredicate(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	klog.Infof("Starting handle localstorage scheduler predicate")
	var (
		buf                  bytes.Buffer
		extenderArgs         extenderv1.ExtenderArgs
		extenderFilterResult *extenderv1.ExtenderFilterResult
	)

	body := io.TeeReader(req.Body, &buf)
	if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
		extenderFilterResult = &extenderv1.ExtenderFilterResult{Error: err.Error()}
	} else {
		predicate := scheduler.NewPredicate()
		extenderFilterResult = predicate.Handler(extenderArgs)
	}

	resp.Header().Set("Content-Type", "application/json")
	result, err := json.Marshal(extenderFilterResult)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(fmt.Sprintf("{'error':'%s'}", err.Error())))
	} else {
		resp.WriteHeader(http.StatusOK)
		resp.Write(result)
	}
}

func handlePrioritize(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	klog.Infof("Starting handle localstorage scheduler prioritize")
	var (
		buf              bytes.Buffer
		extenderArgs     extenderv1.ExtenderArgs
		hostPriorityList *extenderv1.HostPriorityList
	)

	body := io.TeeReader(req.Body, &buf)
	if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
		hostPriorityList = &extenderv1.HostPriorityList{}
	} else {
		prioritize := scheduler.NewPrioritize()
		hostPriorityList = prioritize.Handler(extenderArgs)
	}

	resp.Header().Set("Content-Type", "application/json")
	result, err := json.Marshal(hostPriorityList)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		resp.Write([]byte(fmt.Sprintf("{'error':'%s'}", err.Error())))
	} else {
		resp.WriteHeader(http.StatusOK)
		resp.Write(result)
	}
}
