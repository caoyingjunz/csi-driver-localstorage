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

package localstorage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	localstoragev1 "github.com/caoyingjunz/csi-driver-localstorage/pkg/apis/localstorage/v1"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/util"
)

type Operation string

const (
	AddOperation Operation = "add"
	SubOperation Operation = "sub"
)

// CreateVolume create a volume
func (ls *localStorage) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume name must be provided")
	}

	caps := req.GetVolumeCapabilities()
	if caps == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume Capabilities missing in request")
	}

	ls.lock.Lock()
	defer ls.lock.Unlock()

	localstorage, err := ls.getLocalStorageByNode(ls.GetNode())
	if err != nil {
		return nil, err
	}

	volumeID := uuid.New().String()

	// TODO: 临时实现，后续修改
	path := ls.parseVolumePath(volumeID)
	if err := os.MkdirAll(path, 0750); err != nil {
		return nil, err
	}

	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	s := localstorage.DeepCopy()

	volSize := req.GetCapacityRange().GetRequiredBytes()
	util.AddVolume(s, localstoragev1.Volume{
		VolID:   volumeID,
		VolName: req.GetName(),
		VolPath: path,
		VolSize: volSize,
	})
	s.Status.Allocatable = ls.calculateAllocatedSize(s.Status.Allocatable, volSize, SubOperation)

	// Update the changes immediately
	if _, err = ls.client.StorageV1().LocalStorages().Update(ctx, s, metav1.UpdateOptions{}); err != nil {
		return nil, err
	}

	volumeContext := req.GetParameters()
	if volumeContext == nil {
		volumeContext = make(map[string]string)
	}
	volumeContext["localPath.caoyingjunz.io"] = path

	klog.Infof("pvc %v volume %v successfully deleted", name, volumeID)
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:           volumeID,
			CapacityBytes:      req.GetCapacityRange().GetRequiredBytes(),
			VolumeContext:      volumeContext,
			ContentSource:      req.GetVolumeContentSource(),
			AccessibleTopology: []*csi.Topology{},
		},
	}, nil
}

// DeleteVolume delete a volume
func (ls *localStorage) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID missing in request")
	}

	ls.lock.Lock()
	defer ls.lock.Unlock()

	localstorage, err := ls.getLocalStorageByNode(ls.GetNode())
	if err != nil {
		return nil, err
	}
	volId := req.GetVolumeId()

	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	s := localstorage.DeepCopy()

	vol := util.RemoveVolume(s, volId)
	s.Status.Allocatable = ls.calculateAllocatedSize(s.Status.Allocatable, vol.VolSize, AddOperation)

	// Update the changes immediately
	if _, err = ls.client.StorageV1().LocalStorages().Update(ctx, s, metav1.UpdateOptions{}); err != nil {
		return nil, err
	}

	// TODO: 临时处理
	if err := os.RemoveAll(ls.parseVolumePath(volId)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	klog.Infof("volume %v successfully deleted", volId)

	return &csi.DeleteVolumeResponse{}, nil
}

func (ls *localStorage) calculateAllocatedSize(allocatableSize resource.Quantity, volSize int64, op Operation) resource.Quantity {
	volSizeCap := util.BytesToQuantity(volSize)
	switch op {
	case AddOperation:
		allocatableSize.Add(volSizeCap)
	case SubOperation:
		allocatableSize.Sub(volSizeCap)
	}

	return allocatableSize
}

// get localstorage object by nodeName, error when not found
func (ls *localStorage) getLocalStorageByNode(nodeName string) (*localstoragev1.LocalStorage, error) {
	lsNodes, err := ls.lsLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var lsNode *localstoragev1.LocalStorage
	for _, l := range lsNodes {
		if l.Spec.Node == nodeName {
			lsNode = l
		}
	}
	if lsNode == nil {
		return nil, fmt.Errorf("failed to found localstorage with node %s", nodeName)

	}

	return lsNode, nil
}

// parseVolumePath returns the canonical path for volume
func (ls *localStorage) parseVolumePath(volID string) string {
	return filepath.Join(ls.config.VolumeDir, volID)
}

func (ls *localStorage) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	volumes := &csi.ListVolumesResponse{
		Entries: []*csi.ListVolumesResponse_Entry{},
	}

	ls.lock.Lock()
	defer ls.lock.Unlock()

	for _, vol := range ls.cache.GetVolumes() {
		volumes.Entries = append(volumes.Entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.VolID,
				CapacityBytes: vol.VolSize,
			},
			Status: &csi.ListVolumesResponse_VolumeStatus{
				PublishedNodeIds: []string{vol.NodeID},
				VolumeCondition:  &csi.VolumeCondition{},
			},
		})
	}

	klog.Infof("Localstorage volumes are: %+v", volumes)
	return volumes, nil
}

func (ls *localStorage) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: ls.getControllerServiceCapabilities(),
	}, nil
}

func (ls *localStorage) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

func (ls *localStorage) getControllerServiceCapabilities() []*csi.ControllerServiceCapability {
	cl := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_VOLUME_CONDITION,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
	}

	var csc []*csi.ControllerServiceCapability
	for _, cap := range cl {
		csc = append(csc, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}

	return csc
}
