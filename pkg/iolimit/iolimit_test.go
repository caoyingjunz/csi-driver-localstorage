package iolimit

import (
	"flag"
	"fmt"
	"testing"
)

var (
	deviceName = flag.String("devicename", "", "")
)

func TestGetDeviceNum(t *testing.T) {
	flag.Parse()

	dInfo, err := GetDeviceNumber(*deviceName)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(dInfo)
}

func TestV2E2E(t *testing.T) {
	// 定义 cases
	tests := map[string]struct {
		uid        string
		deviceName string
	}{
		"besteffort pod": {uid: "429a978d-11aa-4c51-be07-3b64169a762e", deviceName: "/dev/lvmvg/test"},
		"burstable pod":  {uid: "192f4dc5-b502-4c60-a49c-3b3b8ca0ce30", deviceName: "/dev/lvmvg/test1"},
	}

	version, err := GetCGroupVersion()
	if err != nil {
		fmt.Println(err)
	}

	ioInfo := &IOInfo{
		Rbps: 1048576,
	}

	switch version {
	case CGroupV1:
	case CGroupV2:
		for _, testValue := range tests {
			iolimit, _ := NewIOLimitV2(CGroupV2, testValue.uid, ioInfo, testValue.deviceName)
			// fmt.Println(iolimit.PodUid, iolimit.Path, iolimit.DeviceInfo.Major, iolimit.DeviceInfo.Minor)
			if err := iolimit.SetIOLimit(); err != nil {
				fmt.Println(err)
			}
		}

	default:
		fmt.Println("unsupport cgroup version")
	}
}

func TestV1E2E(t *testing.T) {
	// 定义 cases
	tests := map[string]struct {
		uid        string
		deviceName string
	}{
		"besteffort pod": {uid: "4555a76d-f3d2-4bf9-a9b9-478126ae10aa", deviceName: "/dev/ubuntu-vg/test"},
		"burstable pod":  {uid: "ebb0be89-14b0-4468-9468-f8b480697106", deviceName: "/dev/ubuntu-vg/test1"},
	}

	version, err := GetCGroupVersion()
	if err != nil {
		fmt.Println(err)
	}

	ioInfo := &IOInfo{
		Rbps: 1048576,
	}

	switch version {
	case CGroupV1:
		for _, testValue := range tests {
			iolimit, _ := NewIOLimitV1(CGroupV1, testValue.uid, ioInfo, testValue.deviceName)
			// fmt.Println(iolimit.PodUid, iolimit.Path, iolimit.DeviceInfo.Major, iolimit.DeviceInfo.Minor)
			if err := iolimit.SetIOLimit(); err != nil {
				fmt.Println(err)
			}
		}
	case CGroupV2:
	default:
		fmt.Println("unsupport cgroup version")
	}
}
