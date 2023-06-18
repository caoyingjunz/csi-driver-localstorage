package iolimit

type CGroupVersion string

const (
	CGroupV1 CGroupVersion = "cgroup1"
	CGroupV2 CGroupVersion = "cgroup2"

	baseCgroupPath = "/sys/fs/cgroup"

	// CGroupV1 使用
	blkioPath = "/blkio"
	rbpsFile  = "blkio.throttle.read_bps_device"
	riopsFile = "blkio.throttle.read_iops_device"
	wbpsFile  = "blkio.throttle.write_bps_device"
	wiopsFile = "blkio.throttle.write_iops_device"

	// CGroupV2 使用
	mainSubTreeFile = "cgroup.subtree_control"
	kubePodsPath    = "/kubepods.slice"
	besteffortPath  = "/kubepods-besteffort.slice"
	burstablePath   = "/kubepods-burstable.slice"
	ioMaxFile       = "io.max"
)

type IOLimit struct {
	// 需要限速的 pod uid
	PodUid string
	// pod CG path
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
