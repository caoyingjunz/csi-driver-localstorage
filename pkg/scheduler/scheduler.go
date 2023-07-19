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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	coreinformers "k8s.io/client-go/informers/core/v1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"

	v1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/informers/externalversions/localstorage/v1"
	localstorage "github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/scheduler/extender"
)

const (
	version     = "v1.0.1"
	versionPath = "/version"

	apiPrefix        = "/localstorage-scheduler"
	predicatePrefix  = apiPrefix + "/filter"
	prioritizePrefix = apiPrefix + "/prioritize"
)

type ScheduleExtender struct {
	http *httprouter.Router

	predicate  *extender.Predicate
	prioritize *extender.Prioritize

	// lsLister can list/get localstorage from the shared informer's store
	lsLister localstorage.LocalStorageLister
	// pvcLister can list/get pvc from the shared informer's store
	pvcLister corelisters.PersistentVolumeClaimLister
	// scLister can list/get pvc from the shared informer's store
	scLister storagelisters.StorageClassLister

	// lsListerSynced returns true if the localstorage store has been synced at least once.
	lsListerSynced cache.InformerSynced
	// pvcListerSynced returns true if the pvc store has been synced at least once.
	pvcListerSynced cache.InformerSynced
	// scListerSynced returns true if the sc store has been synced at least once.
	scListerSynced cache.InformerSynced
}

func NewScheduleExtender(lsInformer v1.LocalStorageInformer, pvcInformer coreinformers.PersistentVolumeClaimInformer, scInformer storageinformers.StorageClassInformer) (*ScheduleExtender, error) {
	s := &ScheduleExtender{
		http: httprouter.New(),
	}

	// register scheduler extender http router
	s.http.GET(versionPath, s.version)
	s.http.POST(predicatePrefix, s.Predicate)
	s.http.POST(prioritizePrefix, s.Prioritize)

	s.lsLister = lsInformer.Lister()
	s.pvcLister = pvcInformer.Lister()
	s.scLister = scInformer.Lister()
	s.lsListerSynced = lsInformer.Informer().HasSynced
	s.pvcListerSynced = pvcInformer.Informer().HasSynced
	s.scListerSynced = scInformer.Informer().HasSynced

	s.predicate = extender.NewPredicate(s.lsLister, s.pvcLister, s.scLister)
	s.prioritize = extender.NewPrioritize(s.lsLister, s.pvcLister, s.scLister)
	return s, nil
}

func (s *ScheduleExtender) Run(ctx context.Context, addr string) error {
	if !cache.WaitForNamedCacheSync("scheduler-extender", ctx.Done(), s.lsListerSynced, s.pvcListerSynced, s.scListerSynced) {
		klog.Fatalf("failed to WaitForNamedCacheSync")
	}

	klog.Infof("starting localstorage scheduler extender server on %s", addr)
	return http.ListenAndServe(addr, s.http)
}

func (s *ScheduleExtender) version(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	fmt.Fprint(resp, fmt.Sprint(version))
}

func (s *ScheduleExtender) Predicate(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	klog.Infof("filtering for localstorage pod")
	var (
		buf                  bytes.Buffer
		extenderArgs         extenderv1.ExtenderArgs
		extenderFilterResult *extenderv1.ExtenderFilterResult
	)

	body := io.TeeReader(req.Body, &buf)
	if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
		extenderFilterResult = &extenderv1.ExtenderFilterResult{Error: err.Error()}
	} else {
		extenderFilterResult = s.predicate.Filter(extenderArgs)
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

func (s *ScheduleExtender) Prioritize(resp http.ResponseWriter, req *http.Request, params httprouter.Params) {
	klog.Infof("Scoring for localstorage pod")
	var (
		buf              bytes.Buffer
		extenderArgs     extenderv1.ExtenderArgs
		hostPriorityList *extenderv1.HostPriorityList
	)

	body := io.TeeReader(req.Body, &buf)
	if err := json.NewDecoder(body).Decode(&extenderArgs); err != nil {
		hostPriorityList = &extenderv1.HostPriorityList{}
	} else {
		hostPriorityList = s.prioritize.Score(extenderArgs)
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
