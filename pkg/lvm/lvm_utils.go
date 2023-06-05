package lvm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
	"github.com/caoyingjunz/pixiulib/exec"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/klog/v2"
)

const (
	lvCreate string = "lvcreate"
	lvRemove string = "lvremove"
)

func NewVolumeForCreate(pathPrefix string, id string, req *csi.CreateVolumeRequest) (*cache.Volume, string, error) {
	name := req.GetName()
	// sc 中 spec.parameters 字段需要有 vg 的信息
	vgname, ok := req.GetParameters()["vgname"]
	if !ok {
		klog.Info("create volume request sholud contain para of vgname")
		return nil, "", errors.New("sc paras miss vgname")
	}

	if len(name) == 0 || len(pathPrefix) == 0 || len(id) == 0 || len(vgname) == 0 {
		return nil, "", errors.New("name, path prefix, id or vgname error")
	}

	size := req.GetCapacityRange().GetRequiredBytes()
	path := filepath.Join(pathPrefix, vgname, name)

	return &cache.Volume{
		VolName: name,
		VolID:   id,
		VolPath: path,
		VolSize: size,
	}, vgname, nil
}

func NewVolumeForDelete(cache cache.Cache, req *csi.DeleteVolumeRequest) (*cache.Volume, error) {
	volID := req.GetVolumeId()
	if len(volID) == 0 {
		return nil, errors.New("volume id error")
	}

	volume, err := cache.GetVolumeByID(volID)
	if err != nil {
		return nil, err
	}

	return &volume, nil
}

// lvcreate -n test -L 5Gi lvmvg
func CreateLogicalVolume(volume *cache.Volume, vgname string) error {
	// 构造 lvcreate 的命令
	var createLVArg []string

	if len(vgname) == 0 {
		return errors.New("vgname error")
	}

	// volume 是否存在检查
	exist, err := CheckVolumeExists(volume)
	if err != nil {
		return err
	}

	if exist {
		return errors.New("lv already exists")
	}

	createLVArg = append(createLVArg, "-n", volume.VolName)
	createLVArg = append(createLVArg, "-L", fmt.Sprint(volume.VolSize))
	createLVArg = append(createLVArg, vgname)

	exec := exec.New()
	out, err := exec.Command(lvCreate, createLVArg...).CombinedOutput()
	if err != nil {
		klog.Infof("lvcreate failed, lvname: %s, vgname: %s, size: %v\n", volume.VolName, vgname, volume.VolSize)
		return errors.New("lvcreate failed")
	}

	klog.Info(string(out))
	return nil
}

func CheckVolumeExists(vol *cache.Volume) (bool, error) {
	if _, err := os.Stat(vol.VolPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// lvremove /dev/lvmvg/test -f
func RemoveLogicalVolume(volume *cache.Volume) error {
	// 构造 lvremove 的命令
	var removeLVArg []string

	// 删除前预检查
	// TODO: 检查 lv 是否还是被 mount 的，可能存在误删除的情况，这里进行维护

	// lv 是否存在检查
	exist, err := CheckVolumeExists(volume)
	if err != nil {
		return err
	}

	if !exist {
		return errors.New("lv doesn't exists")
	}

	removeLVArg = append(removeLVArg, volume.VolPath)
	removeLVArg = append(removeLVArg, "-f")

	exec := exec.New()
	out, err := exec.Command(lvRemove, removeLVArg...).CombinedOutput()
	if err != nil {
		klog.Infof("lvremove failed, lvname: %s, size: %v\n", volume.VolName, volume.VolSize)
		return errors.New("lvremove failed")
	}

	klog.Info(string(out))
	return nil
}
