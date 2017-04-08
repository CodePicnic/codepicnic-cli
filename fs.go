package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"fmt"
	"github.com/CodePicnic/codepicnic-go"
	"github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"io"
	"os"
	//"os/signal"
	//"syscall"
	"strings"
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
	state      string
	WaitList   []Operation
}

type Operation struct {
	node        fs.Node
	name        string
	source      string
	destination string
}

func (fs *FS) Root() (fs.Node, error) {
	logrus.Debug("FS.Root %v\n", fs)
	node_dir := &Dir{
		fs:   fs,
		name: "",
		//NodeMap: make(map[string]fs.Node),
	}
	return node_dir, nil
}

func (f *FS) StateDown() {
	switch f.state {
	case "offline", "offline-soft":
		f.state = "offline"
	case "online", "online-soft":
		f.state = "offline-soft"
	}
	logrus.Debug("Ping FAIL ", f.state)
}

func (f *FS) StateUp() {
	switch f.state {
	case "offline", "offline-soft":
		f.state = "online-soft"
	case "online", "online-soft":
		f.state = "online"
	}
	logrus.Debug("Ping OK ", f.state)
}

func MountConsole(access_token string, container_name string, mount_dir string) error {
	var mount_point string
	var mountlink string
	var mountlabel string
	console, err := codepicnic.GetConsole(container_name)
	if err != nil {
		fmt.Printf("console error %v \n", err)
		return err
	}
	//_, console, _ := isValidConsole(access_token, container_name)
	if len(console.Title()) > 0 {
		mountlink = console.Permalink()
		mountlabel = console.Title() + " (CodePicnic)"
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
		state:      "online",
	}
	logrus.Debug("Start Dispatcher 100")
	StartDispatcher(50)
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

	//ping_ticker := time.NewTicker(time.Millisecond * 60000)
	ping_ticker := time.NewTicker(time.Millisecond * 30000)
	go func() {
		for _ = range ping_ticker.C {
			logrus.Debug("Ping codepicnic.com/api ")
			_, err := console.Status()
			if err != nil {
				switch err.Error() {
				case codepicnic.ERROR_CONNECTION_REFUSED, codepicnic.ERROR_TCP_TIMEOUT, codepicnic.ERROR_CLIENT_TIMEOUT, codepicnic.ERROR_TLS_TIMEOUT, codepicnic.ERROR_NETWORK_UNREACHABLE:
					filesys.StateDown()
					//Check again after 10 seconds if state is offline-soft
					if filesys.state == "offline-soft" {
						time.Sleep(10000 * time.Millisecond)
						_, err := console.Status()
						if err != nil {
							switch err.Error() {
							case codepicnic.ERROR_CONNECTION_REFUSED, codepicnic.ERROR_TCP_TIMEOUT, codepicnic.ERROR_CLIENT_TIMEOUT, codepicnic.ERROR_TLS_TIMEOUT, codepicnic.ERROR_NETWORK_UNREACHABLE:
								filesys.StateDown()
							}
						}
					}
				default:
					logrus.Debugf("Ping %v", err)
				}
			} else {
				filesys.StateUp()
				//Check again after 10 seconds if state is online-soft
				if filesys.state == "online-soft" {
					time.Sleep(10000 * time.Millisecond)
					_, err := console.Status()
					if err != nil {
						switch err.Error() {
						case codepicnic.ERROR_CONNECTION_REFUSED, codepicnic.ERROR_TCP_TIMEOUT, codepicnic.ERROR_CLIENT_TIMEOUT, codepicnic.ERROR_TLS_TIMEOUT, codepicnic.ERROR_NETWORK_UNREACHABLE:
							filesys.StateDown()
						}
					} else {
						filesys.StateUp()
					}
				}
			}
			if filesys.state == "online" && len(filesys.WaitList) > 0 {
				filesys.OnlineSync()
			}
		}
	}()
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
			} else if strings.HasSuffix(err.Error(), "invalid argument") {
				isEmpty, _ := IsEmptyDir(mountpoint)
				if isEmpty {
					err = os.Remove(mountpoint)
					RemoveMountFromFile(container_name)
					fmt.Printf(color("Console %s succesfully cleaned\n", "response"), container_name)
					return nil
				} else {
					fmt.Printf(color("Can't remove %s. Please remove the directory and try again\n", "error"), mountpoint)
					return err
				}
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

func (filesys *FS) OnlineSync() error {

	logrus.Debugf("Reading WaitList %+v", filesys.WaitList)
	i := 0
	for _, op := range filesys.WaitList {
		switch op.name {
		case "mkdir":
			logrus.Debugf("Mkdir %s", op.source)
			switch dir := op.node.(type) {
			case *Dir:
				err := dir.AsyncCreateDir(op.source)
				if err != nil {
					logrus.Debugf("Mkdir error %s", op.source)
					filesys.WaitList[i] = op
					i++
				}
			}
		case "rmdir":
			logrus.Debugf("Rmdir %s", op.source)
			switch dir := op.node.(type) {
			case *Dir:
				err := dir.RemoveDir(op.source)
				if err != nil {
					logrus.Debugf("rmdir error %s", op.source)
					filesys.WaitList[i] = op
					i++
				}
			}
		case "rm":
			logrus.Debugf("rm %s", op.source)
			switch dir := op.node.(type) {
			case *Dir:
				err := dir.RemoveFile(op.source)
				if err != nil {
					logrus.Debugf("rm error %s", op.source)
					filesys.WaitList[i] = op
					i++
				}
			}
		case "touch":
			logrus.Debugf("touch %s", op.source)
			switch f := op.node.(type) {
			case *File:
				err := f.dir.TouchFile(op.source)
				if err != nil {
					logrus.Debugf("touch error %s", op.source)
					filesys.WaitList[i] = op
					i++
				}
			}
		case "upload":
			logrus.Debugf("upload %s", op.source)
			switch f := op.node.(type) {
			case *File:
				err := f.AsyncUploadFile()
				if err != nil {
					logrus.Debugf("upload error %s", op.source)
					filesys.WaitList[i] = op
					i++
				}
			}

		default:
			logrus.Debugf("Operation %s", op.name)

		}
	}
	filesys.WaitList = filesys.WaitList[:i]
	if len(filesys.WaitList) == 0 {
		NotifyDesktop("Your files were succesfully synced")
	}
	return nil
}

func (fs *FS) OfflineSync(d *Dir, ctx context.Context) error {
	logrus.Debug("OfflineSync parent: ", d.name)
	for _, node := range d.NodeMap {
		switch nh := node.(type) {
		case *Dir:
			logrus.Debug("OfflineSync child:", nh.name)
			nh.ReadDirAll(ctx)
			if nh.read == false {
				fs.OfflineSync(nh, ctx)
				nh.read = true
			}
		case *File:
			logrus.Debug("OfflineSync child: ", nh.name, nh.mime)
			if strings.HasPrefix(nh.mime, "application") || strings.HasPrefix(nh.mime, "image") {
				logrus.Debug("OfflineSync excluded: ", nh.name)
			} else {
				req_read := &fuse.ReadRequest{
					FileFlags: fuse.OpenReadOnly,
				}
				resp_read := &fuse.ReadResponse{}
				nh.Read(ctx, req_read, resp_read)
			}
		}
	}
	return nil
}

func IsEmptyDir(dir string) (bool, error) {
	d, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer d.Close()

	_, err = d.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}
