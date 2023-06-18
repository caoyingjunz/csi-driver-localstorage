package iolimit

import (
	"errors"
	"os"
	"strings"
	"syscall"

	"github.com/caoyingjunz/pixiulib/exec"
)

func exists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, false
	}
	return info, true
}

func FileExists(filepath string) bool {
	info, present := exists(filepath)
	return present && info.Mode().IsRegular()
}

func DirExists(path string) bool {
	info, present := exists(path)
	return present && info.IsDir()
}

func FilePerm(path string) (os.FileMode, bool) {
	info, present := exists(path)
	if present {
		return info.Mode().Perm(), present
	}
	return 0, present
}

// 检查系统 cgroup 的版本
// stat -fc %T /sys/fs/cgroup/
func GetCGroupVersion() (CGroupVersion, error) {
	var getVersionArgs []string

	getVersionArgs = append(getVersionArgs, "-fc", "%T", baseCgroupPath)

	exec := exec.New()
	out, err := exec.Command("stat", getVersionArgs...).CombinedOutput()
	if err != nil {
		return "", err
	}

	if strings.Contains(string(out), "tmpfs") {
		return CGroupV1, nil
	} else if strings.Contains(string(out), "cgroup2fs") {
		return CGroupV2, nil
	} else {
		return "", errors.New("error cgroup fomart")
	}
}

// deviceName 格式：/dev/vgname/lvname
func GetDeviceNumber(deviceName string) (*DeviceInfo, error) {
	stat := syscall.Stat_t{}
	if err := syscall.Stat(deviceName, &stat); err != nil {
		return nil, err
	}
	return &DeviceInfo{
		Major: uint(stat.Rdev / 256),
		Minor: uint(stat.Rdev % 256),
	}, nil
}

func GetPodCGPathSuffix(podUid string) string {
	return "pod" + strings.ReplaceAll(podUid, "-", "_")
}
