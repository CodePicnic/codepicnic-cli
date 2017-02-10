package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	//"bytes"
	//"errors"
	"fmt"
	//"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	//"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	//"io"
	//"io/ioutil"
	//"mime/multipart"
	//"net/http"
	"os"
	//"regexp"
	//"strconv"
	"strings"
	"sync"
	"syscall"
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
	ltree      map[string][]Node
}

func (f *FS) Root() (fs.Node, error) {
	logrus.Infof("FS.Root %v\n", f)
	f.ltree = make(map[string][]Node)
	node_dir := &Dir{
		fs:      f,
		path:    "",
		nodemap: make(map[string]Node),
		//mime: "inode/directory",
		//mimemap: make(map[string]string),
		//sizemap: make(map[string]uint64),
	}
	f.ltree[""] = SetDummyLDir()
	return node_dir, nil
}

type Node struct {
	name    string
	size    uint64
	dtype   fuse.DirentType
	offline bool
}

type Dir struct {
	fs   *FS
	path string
	//mime    string
	nodemap map[string]Node
	//mimemap map[string]string
	//sizemap map[string]uint64
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
	dir  *Dir
	name string
	path string
	//basedir string
	mime    string
	mu      sync.Mutex
	data    []byte
	writers uint
	size    uint64
	//swap     bool
	//readlock bool
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
			var n Node
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
				n.size = f.size
			}
			inode.Name = f.name
			n.name = inode.Name
			n.dtype = inode.Type
			n.offline = false

			d.nodemap[f.name] = n
			res = append(res, inode)
		}
	}
	//Only offline nodes from ltree are added to the Dirent
	for _, ln := range d.fs.ltree[d.path] {
		if ln.offline == true {
			inode.Type = ln.dtype
			inode.Name = ln.name
			res = append(res, inode)
			d.nodemap[ln.name] = ln
		}
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

func SetDummyLDir() []Node {

	var n Node
	var ln []Node
	n.name = ".codepicnic"
	n.dtype = fuse.DT_File
	n.offline = true
	n.size = 21
	ln = append(ln, n)
	return ln

}

var _ = fs.NodeRequestLookuper(&Dir{})

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	/*
		if req.Name == "CONNECTION_ERROR_CHECK_YOUR_CODEPICNIC_ACCOUNT" {
			child := &File{
				size: 0,
				name: req.Name,
			}
			return child, nil
		}*/
	path := req.Name
	if d.path != "" {
		path = d.path + "/" + path
	}
	/*

		cache_key := d.fs.container + ":mimemap:" + d.path
		cache_data, _ := cp_cache.Get(cache_key)
	*/
	/*
		lookup_mimemap := make(map[string]string)
		if len(d.mimemap) == 0 && cache_data != nil {
			lookup_mimemap = cache_data.(map[string]string)
		fus} else {
			lookup_mimemap = d.mimemap
		}
		if lookup_mimemap[req.Name] != "" {
			switch {
			case lookup_mimemap[req.Name] == "inode/directory":
				child := &Dir{
					fs:      d.fs,
					path:    path,
					mimemap: make(map[string]string),
					sizemap: make(map[string]uint64),
				}
				return child, nil
			default:
				child := &File{
					size:       d.sizemap[req.Name],
					name:       req.Name,
					path:       path,
					mime:       d.mimemap[req.Name],
					basedir:    d.path,
					fs:         d.fs,
					dir:        d,
					mountpoint: d.mountpoint,
					readlock:   false,
				}
				return child, nil
			}
		}
	*/
	node := d.nodemap[req.Name]
	if (Node{}) != d.nodemap[req.Name] {
		switch {
		case node.dtype == fuse.DT_Dir:
			logrus.Infof("Lookup %v\n", node)
			child := &Dir{
				fs:      d.fs,
				path:    path,
				nodemap: make(map[string]Node),
			}
			return child, nil
		case node.dtype == fuse.DT_File:
			child := &File{
				size: node.size,
				name: req.Name,
				path: path,
				//mime:       d.mimemap[req.Name],
				//basedir:    d.path,
				//fs:  d.fs,
				dir: d,
				//mountpoint: d.mountpoint,
				//readlock:   false,
			}
			return child, nil
		default:
			return nil, fuse.ENOENT
		}
	}
	return nil, fuse.ENOENT
}

var _ fs.NodeOpener = (*File)(nil)

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	logrus.Infof("Open f.dir.nodemap %+v", f.dir.nodemap[f.name])
	resp.Flags |= fuse.OpenKeepCache
	return f, nil
}

var _ fs.Handle = (*File)(nil)

var _ fs.HandleReader = (*File)(nil)

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	var content string
	logrus.Infof("Read f.dir.nodemap %+v", f.dir.nodemap[f.name])

	if f.dir.nodemap[f.name].offline == true {
		content = string(f.data)
	} else {
		content, _ = f.ReadFile()
	}
	/*
	   if err != nil {
	       if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
	           //Probably the token expired, try again
	           //logrus.Infof("Token expired, generating a new one")
	           f.fs.token, err = GetTokenAccess()
	           t, err = f.ReadFile()
	       }
	   }*/
	fuseutil.HandleRead(req, resp, []byte(content))
	return nil
}

var _ = fs.NodeMkdirer(&Dir{})

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	var new_dir string
	path := req.Name
	if d.path != "" {
		path = d.path + "/" + path
	}
	if d.path == "/" || d.path == "" {
		new_dir = req.Name
	} else {
		new_dir = d.path + "/" + req.Name
	}
	//err := d.CreateDir(new_dir)
	d.CreateDir(new_dir)
	/*
		if err != nil {
			if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
				//Probably the token expired, try again
				//logrus.Infof("Token expired, generating a new one")
				d.fs.token, err = GetTokenAccess()
				d.CreateDir(new_dir)
			}
		}*/
	/*cache_key := d.fs.container + ":mimemap:" + d.path
	cp_cache.Set(cache_key, d.mimemap, cache.DefaultExpiration)*/
	//add new path into the ltree
	d.fs.ltree[path] = make([]Node, 0)
	//add new local Node into the nodemap
	var ln Node
	ln.name = req.Name
	ln.dtype = fuse.DT_Dir
	d.nodemap[req.Name] = ln
	n := &Dir{
		fs:      d.fs,
		path:    path,
		nodemap: make(map[string]Node),
	}
	return n, nil
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	var new_file string
	ch := make(chan error)
	path := req.Name
	if d.path != "" {
		path = d.path + "/" + path
	}
	f := &File{
		name:    req.Name,
		path:    path,
		writers: 0,
		dir:     d,
		//basedir:  d.path,
		//readlock: false,
	}
	/*cache_key := d.fs.container + ":mimemap:" + d.path
	cp_cache.Set(cache_key, d.mimemap, cache.DefaultExpiration)
	cache_key = d.fs.container + ":" + d.path
	cp_cache.Get(cache_key)*/
	var n Node
	n.name = req.Name
	n.dtype = fuse.DT_File
	if IsOffline(req.Name) == true {
		n.offline = true
		n.size = 0
		d.fs.ltree[d.path] = append(d.fs.ltree[d.path], n)
	} else {
		n.offline = false
		if d.path == "" {
			new_file = req.Name
		} else {
			new_file = d.path + "/" + req.Name
		}
		//err := d.CreateFile(new_file)
		go d.TouchFile(new_file, ch)
	}

	d.nodemap[req.Name] = n
	return f, f, nil
}

const maxInt = int(^uint(0) >> 1)

var _ = fs.HandleWriter(&File{})

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	//logrus.Infof("Write f.dir.nodemap %+v", f.dir.nodemap[f.name])
	//logrus.Infof("Write req.Offset %i ", req.Offset)
	//logrus.Infof("Write req.Data %+v %i", req.Data, len(req.Data))
	logrus.Infof("Write f.data %s", string(f.data))
	f.writers = 1
	f.mu.Lock()
	defer f.mu.Unlock()

	// expand the buffer if necessary
	newLen := req.Offset + int64(len(req.Data))
	if newLen > int64(maxInt) {
		return fuse.Errno(syscall.EFBIG)
	}

	//use file size is better than len(f.data)
	if newLen := int(newLen); newLen > len(f.data) {
		f.data = append(f.data, make([]byte, newLen-len(f.data))...)
	} else if newLen < len(f.data) {
		f.data = append([]byte(nil), req.Data[:newLen]...)
	}

	_ = copy(f.data[req.Offset:], req.Data)
	resp.Size = len(req.Data)
	f.size = uint64(len(req.Data))
	var n Node
	n.name = f.name
	n.dtype = fuse.DT_File
	n.offline = f.dir.nodemap[f.name].offline
	n.size = f.size
	f.dir.nodemap[f.name] = n
	logrus.Infof("Write f.dir.nodemap %+v", f.dir.nodemap[f.name])
	logrus.Infof("Write f.size %+v", f.size)
	return nil
}

var _ = fs.HandleFlusher(&File{})

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	if f.writers == 0 {
		// Read-only handles also get flushes. Make sure we don't
		// overwrite valid file contents with a nil buffer.
		return nil
	}

	if f.dir.nodemap[f.name].offline == true {
	} else {

		//err := f.UploadFile()
		f.UploadFile()
		/*
			if err != nil {
				if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
					//Probably the token expired, try again
					//logrus.Infof("Token expired, generating a new one")
					f.fs.token, err = GetTokenAccess()
					f.UploadFile()
				}
			}*/

	}
	return nil
}

var _ = fs.HandleReleaser(&File{})

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	//logrus.Infof("Release %v", f.name)
	//logrus.Infof("Release %v %v", f.name, f.writers)
	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		//  return nil
	}
	f.writers = 0
	//f.UploadFile()

	return nil
}
