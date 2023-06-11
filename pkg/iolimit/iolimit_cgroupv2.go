package iolimit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
)

/*
debian ARM  cgroup2
*/

type IOLimitV2 struct {
	CGVersion CGroupVersion
	*IOLimit
}

func NewIOLimitV2(version CGroupVersion, vol *cache.Volume, pid int, ioInfo *IOInfo, dInfo *DeviceInfo) (*IOLimitV2, error) {
	if version != CGroupV2 {
		return nil, errors.New("CGroupVersion error")
	}

	// 确保 cgroup.subtree_control 文件中有 io， 代表开启 io 控制器
	if err := makeSureMainSubtreeFileExist(); err != nil {
		return nil, err
	}

	// 创建目标路径
	path := filepath.Join(baseCgroupPath, vol.VolName)
	if err := os.Mkdir(path, 0755); err != nil {
		return nil, errors.New("create iolimit file failed")
	}

	if pid == 0 {
		return nil, errors.New("pid can't be 0")
	}

	return &IOLimitV2{
		CGVersion: version,
		IOLimit: &IOLimit{
			Vol:        vol,
			Pid:        pid,
			Path:       path,
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
	}
	if i.IOInfo.Riops != 0 {
		writeInfo += " riops=" + fmt.Sprint(i.IOInfo.Riops)
	}
	if i.IOInfo.Wbps != 0 {
		writeInfo += " wbps=" + fmt.Sprint(i.IOInfo.Wbps)
	}
	if i.IOInfo.Wiops != 0 {
		writeInfo += " wiops=" + fmt.Sprint(i.IOInfo.Wiops)
	}
	return writeInfo
}
