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

package storage

import (
	"context"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/clientset/versioned"
	"github.com/caoyingjunz/csi-driver-localstorage/pkg/client/listers/localstorage/v1"
)

type StorageController struct{}

// NewStorageController creates a new StorageController.
func NewStorageController(ctx context.Context, clientSet versioned.Interface, sLister v1.LocalStorageLister) (*StorageController, error) {
	sc := &StorageController{}

	return sc, nil
}

func (s *StorageController) Run(ctx context.Context, workers int) error {
	return nil
}
