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
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
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
	route.POST(prioritizePrefix, handlerPrioritize)
}

func handleVersion(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	fmt.Fprint(resp, fmt.Sprint(version))
}

func handlePredicate(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {

}

func handlerPrioritize(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
}
