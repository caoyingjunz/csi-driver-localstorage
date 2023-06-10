package iolimit

import "os"

func exists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, false
	}
	return info, true
}

func FileExists(filepath string) bool {
	info, present := exists(filepath)
	return present && info.Mode().IsRegular()
}

func DirExists(path string) bool {
	info, present := exists(path)
	return present && info.IsDir()
}

func FilePerm(path string) (os.FileMode, bool) {
	info, present := exists(path)
	if present {
		return info.Mode().Perm(), present
	}
	return 0, present
}
