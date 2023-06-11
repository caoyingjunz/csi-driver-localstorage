package iolimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
)

/*
	ubuntu 20.04 TLS ARM  cgroup1
*/

type IOLimit struct {
	// 使用 VolName 作为子文件夹目录
	Vol cache.Volume
	// 需要限速的进程 id
	Pid        int
	Path       string
	IOInfo     *IOInfo
	DeviceInfo *DeviceInfo
}

func NewIOLimit(vol *cache.Volume, pid int, ioInfo *IOInfo, dInfo *DeviceInfo) (*IOLimit, error) {
	// 检查环境中是否有 cgroup 路径
	if exist := DirExists(baseCgroupPath); !exist {
		return nil, errors.New("check cgroup path error")
	}

	// 检查 cgroup 下是否有 blkio 文件夹
	blkioPath := filepath.Join(baseCgroupPath, blkioPath)
	if exist := DirExists(blkioPath); !exist {
		return nil, errors.New("check blkio path error")
	}

	// 创建 iolimit 文件夹
	path := filepath.Join(blkioPath, vol.VolName)
	if err := os.Mkdir(path, 0755); err != nil {
		return nil, errors.New("create iolimit file failed")
	}

	if pid == 0 {
		return nil, errors.New("pid can't be 0")
	}

	return &IOLimit{
		Vol:        *vol,
		Pid:        pid,
		Path:       path,
		IOInfo:     ioInfo,
		DeviceInfo: dInfo,
	}, nil
}

func (i *IOLimit) SetIOLimit() error {
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
	if err := i.setTasks(); err != nil {
		return err
	}

	return nil
}

func (i *IOLimit) setRbps() error {
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

func (i *IOLimit) setRiops() error {
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

func (i *IOLimit) setWbps() error {
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

func (i *IOLimit) setWiops() error {
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

func (i *IOLimit) setTasks() error {
	filePath := filepath.Join(i.Path, taskFile)
	prem, exist := FilePerm(filePath)
	if !exist {
		return errors.New("miss tasks file")
	}

	if err := os.WriteFile(filePath, []byte(fmt.Sprint(i.Pid)), prem); err != nil {
		return err
	}

	return nil
}
