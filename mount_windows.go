package main

import (
	"github.com/keybase/kbfs/dokan"
)

func UnmountConsole(container_name string) error {

	return nil
}
func MountConsole(access_token string, container_name string, mount_dir string) error {
	var myFileSystem FileSystem // Should be the real filesystem implementation
	mp, err := Mount(&Config{FileSystem: myFileSystem, Path: `Q:`})
	if err != nil {
		log.Fatal("Mount failed:", err)
	}
	err = mp.BlockTillDone()
	if err != nil {
		log.Println("Filesystem exit:", err)
	}
	return nil
}
