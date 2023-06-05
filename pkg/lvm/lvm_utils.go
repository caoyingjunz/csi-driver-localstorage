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

type LogicalVolume struct {
	Path   string
	Name   string
	VGName string
	Size   string
	ID     string
}

// 根据 CreateVolumeRequest 生成 LV struct
func NewLogicalVolumeForCreate(pathPrefix string, id string, req *csi.CreateVolumeRequest) (*LogicalVolume, error) {
	name := req.GetName()
	if len(name) == 0 {
		return nil, errors.New("CreateVolumeRequest miss name")
	}

	if len(pathPrefix) == 0 {
		return nil, errors.New("pathPrefix error")
	}

	// sc 中 spec.parameters 字段需要有 vg 的信息
	vgname, ok := req.GetParameters()["vgname"]
	if !ok {
		klog.Info("create volume request sholud contain para of vgname")
		return nil, errors.New("sc paras miss vgname")
	}

	size := req.GetCapacityRange().GetRequiredBytes()
	path := filepath.Join(pathPrefix, vgname, name)

	return &LogicalVolume{
		Path:   path,
		Name:   name,
		VGName: vgname,
		Size:   fmt.Sprint(size),
		ID:     id,
	}, nil
}

// 根据 DeleteVolumeRequest 生成 LV struct
func NewLogicalVolumeForDelete(cache cache.Cache, req *csi.DeleteVolumeRequest) (*LogicalVolume, error) {
	volID := req.GetVolumeId()
	volume, err := cache.GetVolumeByID(volID)
	if err != nil {
		return nil, err
	}

	return &LogicalVolume{
		Path: volume.VolPath,
		Name: volume.VolName,
		Size: fmt.Sprint(volume.VolSize),
		ID:   volume.VolID,
	}, nil
}

// lvcreate -n test -L 5Gi lvmvg
func CreateLogicalVolume(lv *LogicalVolume) error {
	// 构造 lvcreate 的命令
	var createLVArg []string

	if len(lv.Name) == 0 || len(lv.VGName) == 0 {
		klog.Info("lvname and vgname can't be empty")
		return errors.New("miss lvname or vgname")
	}

	// TODO: 优化 size 的校验方式
	if len(lv.Size) == 0 {
		klog.Info("lvsize can't be empty")
		return errors.New("miss lvsize")
	}

	// lv 是否存在检查
	exist, err := CheckVolumeExists(lv)
	if err != nil {
		return err
	}

	if exist {
		return errors.New("lv already exists")
	}

	createLVArg = append(createLVArg, "-n", lv.Name)
	createLVArg = append(createLVArg, "-L", lv.Size)
	createLVArg = append(createLVArg, lv.VGName)

	exec := exec.New()
	out, err := exec.Command(lvCreate, createLVArg...).CombinedOutput()
	if err != nil {
		klog.Infof("lvcreate failed, lvname: %s, vgname: %s, size: %v\n", lv.Name, lv.VGName, lv.Size)
		return errors.New("lvcreate failed")
	}

	klog.Info(string(out))
	return nil
}

func CheckVolumeExists(lv *LogicalVolume) (bool, error) {
	if _, err := os.Stat(lv.Path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// lvremove /dev/lvmvg/test -f
func RemoveLogicalVolume(lv *LogicalVolume) error {
	// 构造 lvremove 的命令
	var removeLVArg []string

	// 删除前预检查
	// TODO: 检查 lv 是否还是被 mount 的，可能存在误删除的情况，这里进行维护
	if len(lv.Name) == 0 {
		klog.Info("lvname can't be empty")
		return errors.New("miss lvname")
	}

	// lv 是否存在检查
	exist, err := CheckVolumeExists(lv)
	if err != nil {
		return err
	}

	if !exist {
		return errors.New("lv doesn't exists")
	}

	removeLVArg = append(removeLVArg, lv.Path)
	removeLVArg = append(removeLVArg, "-f")

	exec := exec.New()
	out, err := exec.Command(lvRemove, removeLVArg...).CombinedOutput()
	if err != nil {
		klog.Infof("lvremove failed, lvname: %s, vgname: %s, size: %v\n", lv.Name, lv.VGName, lv.Size)
		return errors.New("lvremove failed")
	}

	klog.Info(string(out))
	return nil
}
