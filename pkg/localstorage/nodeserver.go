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
	"context"
	storageutil "github.com/caoyingjunz/csi-driver-localstorage/pkg/util/storage"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
	"os"
)

// NodeGetCapabilities return the capabilities of the Node plugin
func (ls *localStorage) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{Capabilities: []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
	}}, nil
}

// NodeGetInfo return info of the node on which this plugin is running
func (ls *localStorage) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: ls.config.NodeId,
	}, nil
}

// NodePublishVolume mount the volume
func (ls *localStorage) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	klog.Infof(" NodePublishVolume req %v\n", req)

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing VolumeId")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing TargetPath")
	}

	localstorage, err := storageutil.GetLocalStorageByNode(ls.lsLister, ls.GetNode())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	sourcePath := ""
	for _, volume := range localstorage.Status.Volumes {
		if req.GetVolumeId() == volume.VolID {
			sourcePath = volume.VolPath
		}
	}
	if len(sourcePath) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "can't found LocalStorage volume, id %s", req.GetVolumeId())
	}

	targetPath := req.TargetPath
	klog.V(2).Infof("mount source path [%s] to target path [%s] \n", sourcePath, targetPath)

	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(targetPath, 0750)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	mountOptions := []string{"bind"}
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	mounter := mount.New("")
	if err := mounter.Mount(sourcePath, targetPath, "", mountOptions); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmount the volume
func (ls *localStorage) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	klog.Infof(" NodeUnpublishVolume req %v\n", req)

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing VolumeId")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing TargetPath")
	}

	targetPath := req.TargetPath
	klog.Infof("umount target path [%s]\n", targetPath)

	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return nil, status.Error(codes.Internal, "target path not found")
		}
	}

	mounter := mount.New("")
	if err := mounter.Unmount(targetPath); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeStageVolume stage volume
func (ls *localStorage) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume unstage volume
func (ls *localStorage) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodeGetVolumeStats get volume stats
func (ls *localStorage) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, nil
}

// NodeExpandVolume node expand volume
func (ls *localStorage) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
