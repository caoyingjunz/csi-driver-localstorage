package lvm

// func preTestCreate() error {
// 	exec := exec.New()
// 	_, err := exec.LookPath(lvCreate)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func preTestRemove() error {
// 	exec := exec.New()
// 	_, err := exec.LookPath(lvRemove)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

// func TestCreateLV(t *testing.T) {
// 	if err := preTestCreate(); err != nil {
// 		return
// 	}

// 	pathPrefix := "/dev"
// 	volumeID := "123456789"
// 	vgname := "vg1"
// 	req := &csi.CreateVolumeRequest{
// 		Name: "testLV",
// 		CapacityRange: &csi.CapacityRange{
// 			RequiredBytes: 1000,
// 		},
// 		Parameters: make(map[string]string),
// 	}
// 	req.Parameters["vgname"] = vgname

// 	volume, vgname, _ := NewVolumeForCreate(pathPrefix, volumeID, req)
// 	fmt.Println(volume, vgname)

// 	CreateLogicalVolume(volume, vgname)

// 	exist, _ := CheckVolumeExists(volume)
// 	if !exist {
// 		t.Fatal()
// 	}
// }

// func TestRemoveLV(t *testing.T) {
// 	if err := preTestRemove(); err != nil {
// 		return
// 	}

// 	volume := &cache.Volume{
// 		VolPath: "/dev/vg1/testLV",
// 	}

// 	RemoveLogicalVolume(volume)

// 	exist, _ := CheckVolumeExists(volume)
// 	if exist {
// 		t.Fatal()
// 	}
// }
