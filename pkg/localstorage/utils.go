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
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

var (
	requests = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_requests",
			Help:    "gRPC request metrics.",
			Buckets: []float64{0.001, 0.01, 0.1, 1, 10, 100, 1000},
		},
		[]string{"method", "path", "status"},
	)

	VolumeTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "volume_total",
			Help: "Total number of Node Volume",
		},
	)
)

func init() {
	prometheus.MustRegister(requests)
	prometheus.MustRegister(VolumeTotal)
}

func parseEndpoint(ep string) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(ep), "unix://") {
		s := strings.SplitN(ep, "://", 2)
		if s[1] != "" {
			return s[0], s[1], nil
		}
	}
	return "", "", fmt.Errorf("invalid endpoint: %v", ep)
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	klog.V(2).Infof("GRPC call: %s", info.FullMethod)

	resp, err := handler(ctx, req)
	if err != nil {
		klog.Errorf("GRPC error: %v", err)
	}

	requests.With(prometheus.Labels{
		"method": info.FullMethod,
		"status": strconv.Itoa(int(status.Code(err))),
	}).Observe(float64(time.Since(start).Milliseconds()))

	return resp, err
}

func makeVolumeDir(volDir string) error {
	_, err := os.Stat(volDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err = os.MkdirAll(volDir, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}
