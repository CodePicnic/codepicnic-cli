package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	//"bazil.org/fuse/fuseutil"
	//"bytes"
	//"errors"
	"fmt"
	//"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	//"io"
	//"io/ioutil"
	//"mime/multipart"
	//"net/http"
	"os"
	//"regexp"
	//"strconv"
	//"runtime"
	"strings"
	//"sync"
	//"syscall"
	"time"
)

var mount_uid = uint32(1000)
var mount_gid = uint32(1000)

var cp_cache = cache.New(cache.NoExpiration, 30*time.Second)

type FS struct {
	fuse       *fs.Server
	conn       *fuse.Conn
	container  string
	token      string
	mountpoint string
}

func (f *FS) Root() (fs.Node, error) {
	logrus.Debug("FS.Root %v\n", f)
	node_dir := &Dir{
		fs:      f,
		name:    "",
		NodeMap: make(map[string]fs.Node),
	}
	return node_dir, nil
}

type Node struct {
	name    string
	size    uint64
	dtype   fuse.DirentType
	offline bool
	file    *File
	dir     *Dir
}

func MountConsole(access_token string, container_name string, mount_dir string) error {
	var mount_point string
	var mountlink string
	var mountlabel string
	_, console, _ := isValidConsole(access_token, container_name)
	if len(console.Title) > 0 {
		mountlink = console.Permalink
		mountlabel = console.Title + " (CodePicnic)"
	} else {
		mountlink = container_name
		mountlabel = container_name + " (CodePicnic)"
	}
	if mount_dir == "" {
		mount_point = mountlink
		os.Mkdir(mountlink, 0755)
	} else {
		mount_point = mount_dir + "/" + mountlink
		os.Mkdir(mount_dir+"/"+mountlink, 0755)
	}
	mp, err := fuse.Mount(mount_point, fuse.MaxReadahead(32*1024*1024),
		fuse.AsyncRead(), fuse.VolumeName(mountlabel))
	if err != nil {
		logrus.Infof("serve err %v\n", err)
		return err
	}
	defer mp.Close()
	filesys := &FS{
		token:      access_token,
		container:  container_name,
		mountpoint: mount_point,
	}
	logrus.Debug("Start Dispatcher 100")
	StartDispatcher(100)
	logrus.Infof("Serve %v", filesys)
	var mountpoint string
	if strings.HasPrefix(mount_dir, "/") {
		mountpoint = filesys.mountpoint
	} else {
		pwd, _ := os.Getwd()
		mountpoint = pwd + "/" + filesys.mountpoint
	}
	SaveMountsToFile(container_name, mountpoint)

	serveErr := make(chan error, 1)
	fmt.Printf("/app directory mounted on %s \n", mountpoint)
	err = fs.Serve(mp, filesys)
	closeErr := mp.Close()
	if err == nil {
		err = closeErr
	}
	serveErr <- err
	<-mp.Ready
	if err := mp.MountError; err != nil {
		return err
	}
	return err
}

func UnmountConsole(container_name string) error {
	mountpoint := GetMountsFromFile(container_name)
	if mountpoint == "" {
		fmt.Printf("A mount point for container %s doesn't exist\n", container_name)
	} else {
		err := fuse.Unmount(mountpoint)
		if err != nil {
			if strings.HasPrefix(err.Error(), "exit status 1: fusermount: entry for") {
				//if a mount point exists in the config but not in the OS.
				//fmt.Printf("A mount point for container %s doesn't exist\n", container_name)
				fmt.Printf(color("Console %s succesfully cleaned\n", "response"), container_name)
				RemoveMountFromFile(container_name)
			} else if strings.HasSuffix(err.Error(), "Device or resource busy") {
				fmt.Printf(color("Can't unmount. Mount point for console %s is busy.\n", "error"), container_name)
			} else {
				fmt.Printf("Error when unmounting %s %s\n", container_name, err.Error())
			}
			return err
		} else {
			err = os.Remove(mountpoint)
			if err != nil {
				fmt.Println("Error removing dir", err)
			}
			fmt.Printf(color("Console %s succesfully unmounted\n", "response"), container_name)
			RemoveMountFromFile(container_name)
		}
	}
	return nil
}

func (fsys *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	logrus.Debug("Statfs ", req)
	resp.Bavail = 1<<43 + 5
	resp.Bfree = 1<<43 + 5
	resp.Files = 1<<59 + 11
	resp.Ffree = 1<<58 + 13
	//OSX (finder) only supports some Blocks sizes
	//https://github.com/jacobsa/fuse/blob/3b8b4e55df5483817cd361a28d0a830d5acd962b/fuseops/ops.go
	resp.Bsize = 1 << 15
	resp.Namelen = 2048
	return nil

}

func CreateErrorInode() fuse.Dirent {
	var inode fuse.Dirent
	inode.Name = "CONNECTION_ERROR_CHECK_YOUR_CODEPICNIC_ACCOUNT"
	inode.Type = fuse.DT_File
	return inode
}
