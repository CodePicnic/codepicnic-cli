package main

import (
	"github.com/keybase/kbfs/dokan"
)

func UnmountConsole(container_name string) error {

	return nil
}
func MountConsole(access_token string, container_name string, mount_dir string) error {
	return nil
}

func defaultDirectoryInformation() (*dokan.Stat, error) {
	var st dokan.Stat
	st.FileAttributes = dokan.FileAttributeDirectory
	return &st, nil
}
