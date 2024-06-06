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

package lvm

import "sync"

const (
	cmdPhysicalVolumeCreate string = "pvcreate"
	cmdPhysicalVolumeDelete string = "pvmove"

	cmdVolumeGroupCreate string = "vgcreate"
	cmdVolumeGroupExtend string = "vgextend"
)

type Interface interface {
	EnsureVolumeGroup(vg string) (bool, error)
	DeleteVolumeGroup(vg string) error
	VolumeGroupExists(vg string) (bool, error)
}

func New() Interface {
	return &runner{}
}

// runner implements Interface in terms of exec("lvm").
type runner struct {
	mu sync.Mutex
}

func (runner *runner) EnsureVolumeGroup(vg string) (bool, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	return false, nil
}

func (runner *runner) DeleteVolumeGroup(vg string) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	return nil
}

func (runner *runner) VolumeGroupExists(vg string) (bool, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()

	return false, nil
}
