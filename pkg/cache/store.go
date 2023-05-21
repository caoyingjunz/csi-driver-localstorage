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

package cache

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Volume struct {
	VolName  string
	VolID    string
	VolPath  string
	VolSize  int64
	NodeID   string
	Attached bool
}

type Cache interface {
	// GetVolumeByID retrieves a volume by its unique ID or returns
	// an error including that ID when not found.
	GetVolumeByID(volID string) (Volume, error)

	// GetVolumeByName retrieves a volume by its name or returns
	// an error including that name when not found.
	GetVolumeByName(volName string) (Volume, error)

	// GetVolumes returns all currently existing volumes.
	GetVolumes() []Volume

	// SetVolume set the existing volume,
	// identified by its volume ID, or adds it if it does
	// not exist yet.
	SetVolume(volume Volume) error

	// DeleteVolume deletes the volume with the given
	// volume ID. It is not an error when such a volume
	// does not exist.
	DeleteVolume(volID string) error
}

type cache struct {
	Volumes map[string]Volume

	storeFile string
	lock      sync.Mutex
}

var _ Cache = &cache{}

func New(storeFile string) (Cache, error) {
	c := &cache{
		storeFile: storeFile,
	}

	return c, c.restore()
}

func (c *cache) dump() error {
	data, err := json.Marshal(c.Volumes)
	if err != nil {
		return status.Errorf(codes.Internal, "error encoding volumes: %v", err)
	}

	if err := os.WriteFile(c.storeFile, data, 0660); err != nil {
		return status.Errorf(codes.Internal, "error writing store file: %v", err)
	}

	return nil
}

func (c *cache) restore() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.Volumes = nil

	data, err := os.ReadFile(c.storeFile)
	if err != nil {
		// 首次启动，无数据记录
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return status.Errorf(codes.Internal, "error reading state file: %v", err)
	}

	if err := json.Unmarshal(data, &c.Volumes); err != nil {
		return status.Errorf(codes.Internal, "error encoding volumes and snapshots from store file %q: %v", c.storeFile, err)
	}

	return nil
}

func (c *cache) GetVolumeByID(volID string) (Volume, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	vol, exist := c.Volumes[volID]
	if !exist {
		return Volume{}, status.Errorf(codes.NotFound, "volume id %s does not exist in the volumes list", volID)
	}

	return vol, nil
}

func (c *cache) GetVolumeByName(volName string) (Volume, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, vol := range c.Volumes {
		if vol.VolName == volName {
			return vol, nil
		}
	}

	return Volume{}, status.Errorf(codes.NotFound, "volume name %s does not exist in the volumes list", volName)
}

func (c *cache) GetVolumes() []Volume {
	c.lock.Lock()
	defer c.lock.Unlock()

	var volumes []Volume
	for _, vol := range c.Volumes {
		volumes = append(volumes, vol)
	}

	return volumes
}

func (c *cache) SetVolume(volume Volume) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.Volumes == nil {
		c.Volumes = make(map[string]Volume)
	}
	c.Volumes[volume.VolID] = volume

	return c.dump()
}

func (c *cache) DeleteVolume(volID string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, exist := c.Volumes[volID]; !exist {
		return nil
	}
	delete(c.Volumes, volID)

	return c.dump()
}
