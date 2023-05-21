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
	"k8s.io/klog/v2"
	"path"
	"sync"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
)

const (
	DefaultDriverName = "localstorage.csi.caoyingjunz.io"
	StoreFile         = "localstorage.json"
)

type localStorage struct {
	config Config
	cache  cache.Cache

	lock sync.Mutex
}

type Config struct {
	DriverName    string
	Endpoint      string
	VendorVersion string
	NodeId        string
	// Deprecated: 临时使用，后续删除
	VolumeDir string
}

func NewLocalStorage(cfg Config) (*localStorage, error) {
	if cfg.DriverName == "" {
		return nil, fmt.Errorf("no driver name provided")
	}
	if len(cfg.NodeId) == 0 {
		return nil, fmt.Errorf("no node id provided")
	}
	klog.V(2).Infof("Driver: %v version: %v, nodeId: %v", cfg.DriverName, cfg.VendorVersion, cfg.NodeId)

	if err := makeVolumeDir(cfg.VolumeDir); err != nil {
		return nil, err
	}
	storeFile := path.Join(cfg.VolumeDir, StoreFile)
	klog.V(2).Infof("localstorage will be store in %s", storeFile)

	s, err := cache.New(storeFile)
	if err != nil {
		return nil, err
	}
	return &localStorage{
		config: cfg,
		cache:  s,
	}, nil
}

func (ls *localStorage) Run() error {
	s := NewNonBlockingGRPCServer()

	s.Start(ls.config.Endpoint, ls, ls, ls)
	s.Wait()

	return nil
}
