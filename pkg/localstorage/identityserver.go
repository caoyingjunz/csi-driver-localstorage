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
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/labels"
)

type IdentityServer struct {
	Driver *localStorage
}

func (ls *localStorage) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	return &csi.GetPluginInfoResponse{
		Name:          ls.config.DriverName,
		VendorVersion: ls.config.VendorVersion,
	}, nil
}

// 从以下三个方面判断 plugin 的健康
// 1. informer synced检查
// 2. CR 资源获取
// 3. gRPC 接口可以连接
func (ls *localStorage) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	// 1. informer sync检查
	synced := ls.lsListerSynced()
	if !synced {
		return &csi.ProbeResponse{
			Ready: &wrappers.BoolValue{Value: false}}, status.Error(codes.FailedPrecondition, "plugin is not ready")
	}

	// 2. crd 资源获取
	_, err := ls.lsLister.List(labels.Everything())
	if err != nil {
		return &csi.ProbeResponse{
			Ready: &wrappers.BoolValue{Value: false}}, status.Error(codes.FailedPrecondition, "plugin is not ready")
	}

	// 3. grpc 接口可以连接
	unixPrefix := "unix://"
	endpoint := ls.config.Endpoint
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(grpc.ConnectParams{Backoff: backoff.Config{MaxDelay: time.Second}}),
		grpc.WithChainUnaryInterceptor(connection.LogGRPC),
	}

	if strings.HasPrefix(endpoint, unixPrefix) {
		_, err := grpc.DialContext(ctx, endpoint, dialOptions...)
		if err != nil {
			return &csi.ProbeResponse{
				Ready: &wrappers.BoolValue{Value: false}}, status.Error(codes.FailedPrecondition, "plugin is not ready")
		}
	}

	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}

func (ls *localStorage) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}
