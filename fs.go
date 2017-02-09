package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	//"bazil.org/fuse/fuseutil"
	//"bytes"
	//"errors"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	//"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	//"io"
	"io/ioutil"
	//"mime/multipart"
	"net/http"
	"os"
	//"regexp"
	//"strconv"
	"strings"
	"sync"
	//"syscall"
	"time"
)

var mount_uid = uint32(1000)
var mount_gid = uint32(1000)

type FS struct {
	fuse       *fs.Server
	conn       *fuse.Conn
	container  string
	token      string
	file       *File
	mountpoint string
	ltree      []LDir
}

func (f *FS) Root() (fs.Node, error) {
	node_dir := &Dir{
		fs:      f,
		path:    "",
		mime:    "inode/directory",
		mimemap: make(map[string]string),
		sizemap: make(map[string]uint64),
	}
	var ltree []Ldir
	fs.ltree = ltree
	return node_dir, nil
}

type LDir struct {
	Nodes []LocalNode
	Path  string
}

type LocalNode struct {
	Name string
	Type fuse.DirentType
}

type Dir struct {
	fs      *FS
	path    string
	mime    string
	mimemap map[string]string
	sizemap map[string]uint64
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0777
	a.Valid = 5 * time.Minute
	a.Uid = mount_uid
	a.Gid = mount_gid
	return nil
}

type File struct {
	dir      *Dir
	name     string
	path     string
	basedir  string
	mime     string
	mu       sync.Mutex
	data     []byte
	writers  uint
	size     uint64
	swap     bool
	readlock bool
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	if f.mime == "inode/directory" {
		a.Mode = os.ModeDir | 0755
	} else {
		a.Mode = 0777
	}
	a.Size = f.size
	a.Uid = mount_uid
	a.Gid = mount_gid
	a.Valid = 5 * time.Minute
	return nil
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent
	var inode fuse.Dirent
	files_list, err := ListFiles(d.fs.token, d.fs.container, d.path)
	if err != nil {
		/*
			if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
				d.fs.token, err = GetTokenAccess()
				files_list, err = ListFiles(d.fs.token, d.fs.container, d.path)
			} else {
				res = append(res, CreateErrorInode())
				return res, nil
			}*/
	} else {
		for _, f := range files_list {
			/*if d.mimemap == nil {
				d.mimemap = make(map[string]string)
			}
			if d.sizemap == nil {
				d.sizemap = make(map[string]uint64)
			}*/
			path := f.name
			if d.path != "" {
				path = d.path + "/" + path
			}
			/*d.mimemap[f.name] = f.mime
			d.sizemap[f.name] = f.size
			*/
			if f.mime == "inode/directory" {
				inode.Type = fuse.DT_Dir
			} else {
				inode.Type = fuse.DT_File
			}
			inode.Name = f.name
			res = append(res, inode)
		}
	}
	for _, ln := range SetDummyLDir().Nodes {
		inode.Type = ln.Type
		inode.Name = ln.Name
		res = append(res, inode)
	}
	//cache_key := d.fs.container + ":mimemap:" + d.path
	//cp_cache.Set(cache_key, d.mimemap, cache.DefaultExpiration)
	return res, nil
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

func SetDummyLDir() LDir {

	var ld LDir
	var n LocalNode
	n.Name = ".codepicnic.tmp"
	n.Type = fuse.DT_File
	ld.Nodes = append(ld.Nodes, n)
	n.Name = "codepicnic.swp"
	n.Type = fuse.DT_File
	ld.Nodes = append(ld.Nodes, n)
	return ld

}

func ListLocalFiles(container_name string, path string) ([]File, error) {
	var FileCollection []File
	return FileCollection, nil
}

func ListFiles(access_token string, container_name string, path string) ([]File, error) {
	//cache_key := container_name + ":" + path
	var FileCollection []File
	/*FileCollectionCache, found := cp_cache.Get(cache_key)
	if found {
		FileCollection = FileCollectionCache.([]File)
	} else {*/

	cp_consoles_url := site + "/api/consoles/" + container_name + "/files?path=" + path
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("List files %v", err)
		panic(err)
	}
	defer resp.Body.Close()
	/*
		if resp.StatusCode == 401 {
			return FileCollection, errors.New(ERROR_NOT_AUTHORIZED)
		}*/

	body, err := ioutil.ReadAll(resp.Body)
	jsonFiles, err := gabs.ParseJSON(body)
	jsonPaths, _ := jsonFiles.ChildrenMap()
	for key, child := range jsonPaths {
		var jsonFile File
		jsonFile.name = string(key)

		jsonFile.path = child.Path("path").Data().(string)
		jsonFile.mime = child.Path("type").Data().(string)
		jsonFile.size = uint64(child.Path("size").Data().(float64))
		FileCollection = append(FileCollection, jsonFile)

	}
	//cp_cache.Set(cache_key, FileCollection, cache.DefaultExpiration)
	//}
	return FileCollection, nil
}
