package iolimit

import (
	"flag"
	"fmt"
	"testing"

	"github.com/caoyingjunz/csi-driver-localstorage/pkg/cache"
)

var (
	pid     = flag.Int("pid", 0, "")
	volName = flag.String("volname", "", "")
)

func TestE2E(t *testing.T) {
	flag.Parse()
	vol := &cache.Volume{
		VolName: *volName,
	}
	ioInfo := &IOInfo{
		Rbps: 1048576,
	}
	dInfo := &DeviceInfo{
		Major: 8,
		Minor: 0,
	}
	iolimit, err := NewIOLimit(vol, *pid, ioInfo, dInfo)
	if err != nil {
		fmt.Println(err)
		t.Fail()
	}
	iolimit.SetIOLimit()
}
