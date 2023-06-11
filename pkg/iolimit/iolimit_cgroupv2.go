package iolimit

import (
	"errors"

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

	return &IOLimitV2{
		CGVersion: version,
		IOLimit:   &IOLimit{},
	}, nil
}

// 步骤：
// 1. 创建目标路径
// 2. 检查 io 相关文件是否存在
// 3. io.max 写入数据
func (i *IOLimitV2) SetIOLimit() {

}
