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
	"k8s.io/klog/v2"
)

const (
	DefaultDriverName = "localstorage.csi.pixiu.io"
)

type localStorage struct {
	config Config
}

type Config struct {
	DriverName    string
	Endpoint      string
	VendorVersion string
}

func NewLocalStorage(cfg Config) (*localStorage, error) {
	klog.V(2).Infof("Driver: %v version: %v", cfg.DriverName, cfg.VendorVersion)

	ls := &localStorage{
		config: cfg,
	}
	return ls, nil
}

func (ls *localStorage) Run() error {
	s := NewNonBlockingGRPCServer()

	s.Start(ls.config.Endpoint, ls, ls, ls)
	s.Wait()

	return nil
}
