package main

import (
	//"context"

	"unicode/utf8"

	"golang.org/x/net/context"

	"fmt"
	"os"

	codepicnic "github.com/CodePicnic/codepicnic-go"
	"github.com/Sirupsen/logrus"
	"github.com/keybase/kbfs/dokan"
	"github.com/keybase/kbfs/libdokan"
)

type FS struct {
	container  string
	token      string
	mountpoint string
	state      string
	//WaitList   []Operation
}

func (fs FS) CreateFile(ctx context.Context, fi *dokan.FileInfo, data *dokan.CreateData) (file dokan.File, isDirectory bool, err error) {
	fmt.Printf("FielInfo %+v \n", fi)
	fmt.Printf("Data %+v \n", data)
	return EmptyFile{}, false, nil
}

//func (fs FS) GetVolumeInformation(ctx context.Context) (dokan.VolumeInformation, error) {
//	return dokan.VolumeInformation{}, nil
//}

func (fs FS) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, nil
}

func (fs FS) GetDiskFreeSpace(ctx context.Context) (dokan.FreeSpace, error) {
	return dokan.FreeSpace{}, nil
}

func (fs FS) MoveFile(ctx context.Context, source *dokan.FileInfo, targetPath string, replaceExisting bool) error {
	return nil
}

func (fs FS) ErrorPrint(error) {
	return
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
	logrus.Info("Mount data:", mount_point, mountlabel)
	//var filesys FS // Should be the real filesystem implementation
	var filesys dokan.FileSystem
	mount_drive, _ := utf8.DecodeRuneInString(mount_dir)

	mount_driveletter := byte(mount_drive)
	mp, err := dokan.Mount(&dokan.Config{
		FileSystem: filesys,
		Path:       string([]byte{mount_driveletter, ':'}),
		MountFlags: libdokan.DefaultMountFlags})
	if err != nil {
		logrus.Fatal("Mount failed:", err)
	}
	err = mp.BlockTillDone()
	if err != nil {
		logrus.Fatal("Filesystem exit:", err)
	}

	/*mp, err := fuse.Mount(mount_point, fuse.MaxReadahead(32*1024*1024),
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
	}*/
	logrus.Debug("Start Dispatcher 100")
	StartDispatcher(50)
	logrus.Infof("Serve %v", filesys)

	/*
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
	*/
	/*err = fs.Serve(mp, filesys)
	closeErr := mp.Close()
	if err == nil {
		err = closeErr
	}
	serveErr <- err
	<-mp.Ready
	if err := mp.MountError; err != nil {
		return err
	}*/
	return err
}

func UnmountConsole(container_name string) error {

	return nil
}
