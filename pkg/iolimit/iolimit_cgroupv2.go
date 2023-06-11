package iolimit

import (
	"errors"
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

func (i *IOLimitV2) SetIOLimit() {

}
