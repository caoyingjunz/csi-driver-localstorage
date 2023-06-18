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
		t.Fail()
	}

	ioInfo := &IOInfo{
		Rbps: 1048576,
	}

	switch version {
	case CGroupV1:
	case CGroupV2:
		for _, testValue := range tests {
			iolimit, _ := NewIOLimitV2(CGroupV2, testValue.uid, ioInfo, testValue.deviceName)
			fmt.Println(iolimit.PodUid, iolimit.Path, iolimit.DeviceInfo.Major, iolimit.DeviceInfo.Minor)
			if err := iolimit.SetIOLimit(); err != nil {
				t.Fail()
			}
		}

	default:
		t.Error("unsupport cgroup version")
	}
}
