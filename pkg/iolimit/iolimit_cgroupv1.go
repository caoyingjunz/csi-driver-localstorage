package iolimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

type IOLimitV1 struct {
	CGVersion CGroupVersion
	*IOLimit
}

func NewIOLimitV1(version CGroupVersion, podUid string, ioInfo *IOInfo, deviceName string) (*IOLimitV1, error) {
	if version != CGroupV1 {
		return nil, errors.New("CGroupVersion error")
	}

	// 检查环境中是否有 cgroup 路径
	if exist := DirExists(baseCgroupPath); !exist {
		return nil, errors.New("check cgroup path error")
	}

	dInfo, err := GetDeviceNumber(deviceName)
	if err != nil {
		return nil, err
	}

	if _, err := uuid.Parse(podUid); err != nil {
		return nil, errors.New("uncorrect uuid")
	}

	podCGPath, err := getPodCGPathForV1(podUid)
	if err != nil {
		return nil, err
	}

	return &IOLimitV1{
		CGVersion: version,
		IOLimit: &IOLimit{
			PodUid:     podUid,
			Path:       podCGPath,
			IOInfo:     ioInfo,
			DeviceInfo: dInfo,
		},
	}, nil
}

func (i *IOLimitV1) SetIOLimit() error {
	if err := i.setRbps(); err != nil {
		return err
	}
	if err := i.setRiops(); err != nil {
		return err
	}
	if err := i.setWbps(); err != nil {
		return err
	}
	if err := i.setWiops(); err != nil {
		return err
	}

	return nil
}

func (i *IOLimitV1) setRbps() error {
	if i.IOInfo.Rbps == 0 {
		return nil
	}

	filePath := filepath.Join(i.Path, rbpsFile)
	prem, exist := FilePerm(filePath)
	if !exist {
		return errors.New("miss rbps file")
	}

	writeInfo := fmt.Sprintf("%d:%d", i.DeviceInfo.Major, i.DeviceInfo.Minor) + " " + fmt.Sprint(i.IOInfo.Rbps)
	if err := os.WriteFile(filePath, []byte(writeInfo), prem); err != nil {
		return err
	}

	return nil
}

func (i *IOLimitV1) setRiops() error {
	if i.IOInfo.Riops == 0 {
		return nil
	}

	filePath := filepath.Join(i.Path, riopsFile)
	prem, exist := FilePerm(filePath)
	if !exist {
		return errors.New("miss riops file")
	}

	writeInfo := fmt.Sprintf("%d:%d", i.DeviceInfo.Major, i.DeviceInfo.Minor) + " " + fmt.Sprint(i.IOInfo.Riops)
	if err := os.WriteFile(filePath, []byte(writeInfo), prem); err != nil {
		return err
	}

	return nil
}

func (i *IOLimitV1) setWbps() error {
	if i.IOInfo.Wbps == 0 {
		return nil
	}

	filePath := filepath.Join(i.Path, wbpsFile)
	prem, exist := FilePerm(filePath)
	if !exist {
		return errors.New("miss wbps file")
	}

	writeInfo := fmt.Sprintf("%d:%d", i.DeviceInfo.Major, i.DeviceInfo.Minor) + " " + fmt.Sprint(i.IOInfo.Wbps)
	if err := os.WriteFile(filePath, []byte(writeInfo), prem); err != nil {
		return err
	}

	return nil
}

func (i *IOLimitV1) setWiops() error {
	if i.IOInfo.Wiops == 0 {
		return nil
	}

	filePath := filepath.Join(i.Path, wiopsFile)
	prem, exist := FilePerm(filePath)
	if !exist {
		return errors.New("miss wiops file")
	}

	writeInfo := fmt.Sprintf("%d:%d", i.DeviceInfo.Major, i.DeviceInfo.Minor) + " " + fmt.Sprint(i.IOInfo.Wiops)
	if err := os.WriteFile(filePath, []byte(writeInfo), prem); err != nil {
		return err
	}

	return nil
}

// 获取 pod 的 cgroup path
// /sys/fs/cgroup/blkio/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod4555a76d_f3d2_4bf9_a9b9_478126ae10aa.slice
// /sys/fs/cgroup/blkio/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-poda7879371_91e0_4660_a2a3_3ed9cef99a69.slice
func getPodCGPathForV1(podUid string) (string, error) {
	podPathSuffix := GetPodCGPathSuffix(podUid)
	podCGPath := filepath.Join(baseCgroupPath, blkioPath, kubePodsPath) + "kubepods-" + podPathSuffix + ".slice"
	if DirExists(podCGPath) {
		return podCGPath, nil
	}

	podCGPath = filepath.Join(baseCgroupPath, blkioPath, kubePodsPath) + "/kubepods-besteffort.slice/kubepods-besteffort-" + podPathSuffix + ".slice"
	if DirExists(podCGPath) {
		return podCGPath, nil
	}

	podCGPath = filepath.Join(baseCgroupPath, blkioPath, kubePodsPath) + "/kubepods-burstable.slice/kubepods-burstable-" + podPathSuffix + ".slice"
	if DirExists(podCGPath) {
		return podCGPath, nil
	}

	return "", errors.New("get pod cgroup path failed, pod's uid is: " + podUid)
}
