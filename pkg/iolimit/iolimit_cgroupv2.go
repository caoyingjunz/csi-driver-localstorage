package iolimit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

/*
debian ARM  cgroup2
*/

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

	if err := os.WriteFile(path, []byte("+io"), prem); err != nil {
		return err
	}

	return nil
}

// 步骤：
// 1. 创建目标路径
// 2. 检查 io 相关文件是否存在
// 3. io.max 写入数据
func setIOLimit() {

}
