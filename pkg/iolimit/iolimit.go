package iolimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
)

/*
	ubuntu 20.04 TLS ARM
*/

const (
	baseCgroupPath = "/sys/fs/cgroup"
	blkioPath      = "/blkio"
	rbpsFile       = "blkio.throttle.read_bps_device"
	riopsFile      = "blkio.throttle.read_iops_device"
	wbpsFile       = "blkio.throttle.write_bps_device"
	wiopsFile      = "blkio.throttle.write_iops_device"
	taskFile       = "tasks"
)

type IOLimit struct {
	// 使用 VolName 作为子文件夹目录
	Vol cache.Volume
	// 需要限速的进程 id
	Pid        int
	Path       string
	IOInfo     *IOInfo
	DeviceInfo *DeviceInfo
}

// 设备需要设置的读写速率
type IOInfo struct {
	// 按每秒读取块设备的数据量设定上限
	Rbps uint
	// 按每秒读操作次数设定上限
	Riops uint
	// 按每秒写入块设备的数据量设定上限
	Wbps uint
	// 按每秒写操作次数设定上限
	Wiops uint
}

// # ls -l /dev/sda
// brw-rw---- 1 root disk 8, 0 Jun  9 15:16 /dev/sda1
// 主子设备号，linux 系统中用来标识设备的 id
type DeviceInfo struct {
	Major uint
	Minor uint
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
