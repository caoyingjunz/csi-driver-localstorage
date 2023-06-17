package iolimit

import (
	"flag"
	"fmt"
	"testing"
)

var (
	podUid     = flag.String("poduid", "8cdcf7c3-3595-4dd6-8792-16efcfa36a79", "")
	deviceName = flag.String("devicename", "", "")
)

func TestE2E(t *testing.T) {
	flag.Parse()

	ioInfo := &IOInfo{
		Rbps: 1048576,
	}
	iolimit, err := NewIOLimitV1(CGroupV1, *podUid, ioInfo, "/dev/sda")
	if err != nil {
		fmt.Println(err)
		t.Fail()
	}
	iolimit.SetIOLimit()
}

func TestV2E2E(t *testing.T) {
	flag.Parse()

	version, err := GetCGroupVersion()
	if err != nil {
		t.Fail()
	}

	switch version {
	case CGroupV1:
	case CGroupV2:
		ioInfo := &IOInfo{
			Rbps: 1048576,
		}
		_, _ = NewIOLimitV2(CGroupV1, *podUid, ioInfo, "/dev/sda")

	default:
		t.Error("unsupport cgroup version")
	}
}

func TestGetDeviceNum(t *testing.T) {
	flag.Parse()

	dInfo, err := GetDeviceNumber(*deviceName)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(dInfo)
}
