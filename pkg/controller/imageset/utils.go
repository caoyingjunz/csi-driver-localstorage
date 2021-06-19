/*
Copyright 2021 The Pixiu Authors.

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

package imageset

import (
	"fmt"
	"os"
	"strings"
)

// GetHostname returns env's hostname if 'hostnameOverride' is empty; otherwise, return 'hostnameOverride'.
func GetHostName(hostnameOverride string) (string, error) {
	hostName := hostnameOverride

	if len(hostName) == 0 {
		hostName = os.Getenv("NODE_NAME")
	}

	// Trim whitespaces first to avoid getting an empty hostname
	// For linux, the hostname is read from file /proc/sys/kernel/hostname directly
	hostName = strings.TrimSpace(hostName)
	if len(hostName) == 0 {
		return "", fmt.Errorf("empty hostname is invaild")
	}

	return strings.ToLower(hostName), nil
}

const (
	dockerSocket = "/var/run/docker.sock"
	dockerHost   = "unix://" + dockerSocket

	containerdSocket = "/run/containerd/containerd.sock"
)

// isExistingSocket checks if path exists and is domain socket
func isExistingSocket(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	return fileInfo.Mode()&os.ModeSocket != 0
}
