package iolimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type IOLimitV2 struct {
	CGVersion CGroupVersion
	*IOLimit
}

func NewIOLimitV2(version CGroupVersion, podUid string, ioInfo *IOInfo, deviceName string) (*IOLimitV2, error) {
	if version != CGroupV2 {
		return nil, errors.New("CGroupVersion error")
	}

	// 确保 cgroup.subtree_control 文件中有 io，代表开启 io 控制器
	if err := makeSureMainSubtreeFileExist(); err != nil {
		return nil, err
	}

	dInfo, err := GetDeviceNumber(deviceName)
	if err != nil {
		return nil, err
	}

	if _, err := uuid.Parse(podUid); err != nil {
		return nil, errors.New("uncorrect uuid")
	}

	podCGPath, err := getPodCGPathForV2(podUid)
	if err != nil {
		return nil, err
	}

	return &IOLimitV2{
		CGVersion: version,
		IOLimit: &IOLimit{
			PodUid:     podUid,
			Path:       podCGPath,
			IOInfo:     ioInfo,
			DeviceInfo: dInfo,
		},
	}, nil
}

func (i *IOLimitV2) SetIOLimit() error {
	str := i.getIOLImitStr()

	// 检查 io.max 文件存在
	path := filepath.Join(i.Path, ioMaxFile)
	if exist := FileExists(path); !exist {
		return errors.New("miss io.max file")
	}

	prem, _ := FilePerm(path)
	if err := os.WriteFile(path, []byte(str), prem); err != nil {
		return err
	}

	return nil
}

func (i *IOLimitV2) getIOLImitStr() string {
	writeInfo := fmt.Sprintf("%d:%d", i.DeviceInfo.Major, i.DeviceInfo.Minor)
	if i.IOInfo.Rbps != 0 {
		writeInfo += " rbps=" + fmt.Sprint(i.IOInfo.Rbps)
	} else {
		writeInfo += " rbps=max"
	}

	if i.IOInfo.Riops != 0 {
		writeInfo += " riops=" + fmt.Sprint(i.IOInfo.Riops)
	} else {
		writeInfo += " riops=max"
	}

	if i.IOInfo.Wbps != 0 {
		writeInfo += " wbps=" + fmt.Sprint(i.IOInfo.Wbps)
	} else {
		writeInfo += " wbps=max"
	}

	if i.IOInfo.Wiops != 0 {
		writeInfo += " wiops=" + fmt.Sprint(i.IOInfo.Wiops)
	} else {
		writeInfo += " wiops=max"
	}

	return writeInfo
}

// 确保 cgroup.subtree_control 文件中有 io， 代表开启 io 控制器
func makeSureMainSubtreeFileExist() error {
	// 检查环境中是否有 cgroup 路径
	if exist := DirExists(baseCgroupPath); !exist {
		return errors.New("check cgroup path error")
	}

	path := filepath.Join(baseCgroupPath, mainSubTreeFile)
	byteData, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.Contains(string(byteData), "io") {
		return nil
	} else {
		if err := addIOControll(path); err != nil {
			return err
		}
		return nil
	}
}

// 将 io 控制器写入到控制器管理文件
func addIOControll(path string) error {
	prem, exist := FilePerm(path)
	if !exist {
		return errors.New("path error")
	}

	if err := os.WriteFile(path, []byte("+io +memory"), prem); err != nil {
		return err
	}

	return nil
}

// 获取 pod 的 cgroup path
// /sys/fs/cgroup/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-poda093bb10_355b_4a3c_9fec_4ff947f8b4ed.slice
// /sys/fs/cgroup/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod192f4dc5_b502_4c60_a49c_3b3b8ca0ce30.slice
// /sys/fs/cgroup/kubepods.slice/kubepods-pod192f4dc5_b502_4c60_a49c_3b3b8ca0ce30.slice
func getPodCGPathForV2(podUid string) (string, error) {
	podPathSuffix := GetPodCGPathSuffix(podUid)
	podCGPath := filepath.Join(baseCgroupPath, kubePodsPath) + "kubepods-" + podPathSuffix + ".slice"
	if DirExists(podCGPath) {
		return podCGPath, nil
	}

	podCGPath = filepath.Join(baseCgroupPath, kubePodsPath) + "/kubepods-besteffort.slice/kubepods-besteffort-" + podPathSuffix + ".slice"
	if DirExists(podCGPath) {
		return podCGPath, nil
	}

	podCGPath = filepath.Join(baseCgroupPath, kubePodsPath) + "/kubepods-burstable.slice/kubepods-burstable-" + podPathSuffix + ".slice"
	if DirExists(podCGPath) {
		return podCGPath, nil
	}

	return "", errors.New("get pod cgroup path failed, pod's uid is: " + podUid)
}
