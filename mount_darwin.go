package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	"bytes"
	"errors"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var cp_cache = cache.New(5*time.Minute, 30*time.Second)

type FS struct {
	fuse       *fs.Server
	conn       *fuse.Conn
	container  string
	token      string
	file       *File
	mountpoint string
}

func (f *FS) Root() (fs.Node, error) {
	node_dir := &Dir{
		fs:         f,
		mountpoint: f.mountpoint,
		path:       "",
		mimemap:    make(map[string]string),
		sizemap:    make(map[string]uint64),
	}
	//node_dir.mimemap["/"] = "inode/directory"
	//node_dir.mimemap[""] = "inode/directory"
	return node_dir, nil
}

type Dir struct {
	path       string
	mime       string
	mountpoint string
	mimemap    map[string]string
	sizemap    map[string]uint64
	fs         *FS
}

type File struct {
	name       string
	path       string
	basedir    string
	mime       string
	mountpoint string
	mu         sync.Mutex
	data       []byte
	writers    uint
	fs         *FS
	size       uint64
	dir        *Dir
	swap       bool
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	//fmt.Printf("Dir Attr %s \n", d.path)
	//Debug("Dir Attr", d.path)
	a.Mode = os.ModeDir | 0777
	return nil
}

func ListFiles(access_token string, container_name string, path string) ([]File, error) {
	cache_key := container_name + ":" + path
	var FileCollection []File
	FileCollectionCache, found := cp_cache.Get(cache_key)
	if found {
		FileCollection = FileCollectionCache.([]File)
	} else {

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
		if resp.StatusCode == 401 {
			return FileCollection, errors.New(ERROR_NOT_AUTHORIZED)
		}

		body, err := ioutil.ReadAll(resp.Body)
		jsonFiles, err := gabs.ParseJSON(body)
		//logrus.Infof("List files %+v", string(body))

		//fmt.Printf("JsonFiles %v \n", jsonFiles)
		//jsonPaths, _ := jsonFiles.Search("paths").ChildrenMap()
		//jsonTypes, _ := jsonFiles.Search("types").ChildrenMap()
		jsonPaths, _ := jsonFiles.ChildrenMap()
		for key, child := range jsonPaths {
			var jsonFile File
			jsonFile.name = string(key)
			//fmt.Printf("JsonFile %v \n", jsonFile.name)
			//fmt.Printf("Child Data %v \n", child.Path("type").Data())

			jsonFile.path = child.Path("path").Data().(string)
			jsonFile.mime = child.Path("type").Data().(string)
			jsonFile.size = uint64(child.Path("size").Data().(float64))
			//jsonFile.mime = jsonTypes[jsonFile.path].Data().(string)
			//Debug("key, value, type", key, child.Data().(string), jsonTypes[jsonFile.path])
			//Debug("key, value, type", key, child.Data().(string), jsonFile.mime)
			FileCollection = append(FileCollection, jsonFile)
			//fmt.Printf("key: %v, value: %v, type: %v\n", jsonFile.name, jsonFile.path, jsonFile.mime)

		}
		//logrus.Infof("Set Cache %v", cache_key)
		cp_cache.Set(cache_key, FileCollection, cache.DefaultExpiration)
		//for key, child := range jsonTypes {
		//	fmt.Printf("key: %v, value: %v\n", key, child.Data().(string))
		//}
		//_ = json.NewDecoder(resp.Body).Decode(&console_collection)
		//fmt.Printf("%+v\n", string(body))
		//fmt.Printf("%#v\n", console_collection.Consoles[0].Title)
	}
	return FileCollection, nil
}

func UnmountConsole(container_name string) error {
	mountpoint := GetMountsFromFile(container_name)
	if mountpoint == "" {
		fmt.Printf("A mount point for container %s doesn't exist\n", container_name)
	} else {
		err := fuse.Unmount(mountpoint)
		if err != nil {
			if strings.HasPrefix(err.Error(), "exit status 1: fusermount: entry for") {
				fmt.Printf("A mount point for container %s doesn't exist\n", container_name)
			} else {
				fmt.Printf("Error when unmounting %s %s", container_name, err.Error())
			}
			return err
		} else {
			err = os.Remove(mountpoint)
			if err != nil {
				fmt.Println("Error removing dir", err)
			}
			fmt.Printf(color("Container %s succesfully unmounted\n", "response"), container_name)
			SaveMountsToFile(container_name, "")
		}
	}
	return nil
}
func MountConsole(access_token string, container_name string, mount_dir string) error {
	var mount_point string
	var mountlink string
	var mountlabel string
	//var wg sync.WaitGroup
	_, console, _ := isValidConsole(access_token, container_name)
	if len(console.Title) > 0 {
		mountlink = console.Permalink
		mountlabel = console.Title + " (CodePicnic)"
	} else {
		mountlink = container_name
		mountlabel = container_name + " (CodePicnic)"
	}
	if mount_dir == "" {
		//mount_point = container_name
		//os.Mkdir(container_name, 0755)
		mount_point = mountlink
		os.Mkdir(mountlink, 0755)
	} else {
		//mount_point = mount_dir + "/" + container_name
		//os.Mkdir(mount_dir+"/"+container_name, 0755)
		mount_point = mount_dir + "/" + mountlink
		os.Mkdir(mount_dir+"/"+mountlink, 0755)
	}
	//Debug("MountPoint", mount_point)
	mp, err := fuse.Mount(mount_point, fuse.MaxReadahead(32*1024*1024),
		//fuse.AsyncRead(), fuse.WritebackCache())
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
	//cptab := make(map[string]string)
	var mountpoint string
	if strings.HasPrefix(mount_dir, "/") {
		mountpoint = filesys.mountpoint
	} else {
		pwd, _ := os.Getwd()
		mountpoint = pwd + "/" + filesys.mountpoint
	}
	//jsonCPTab, _ := json.Marshal(cptab)
	SaveMountsToFile(container_name, mountpoint)
	//cfg := &fs.Config{
	//	WithContext: func(ctx context.Context, req fuse.Request) context.Context {
	//		return ctx
	//	},
	//}
	//cfg.Debug = debugLog
	//srv := fs.New(mp, cfg)

	serveErr := make(chan error, 1)
	fmt.Printf("/app directory mounted on %s \n", mountpoint)
	//go func() {
	//defer wg.Done()
	//defer mp.Close()
	//serveErr <- fs.Serve(mp, filesys)
	//serveErr <- srv.Serve(filesys)

	// After setting everything up!
	// Wait for a SIGINT (perhaps triggered by user with CTRL-C)
	// Run cleanup when signal is received
	/*
		signalChan := make(chan os.Signal, 1)
		cleanupDone := make(chan bool)
		signal.Notify(signalChan, os.Interrupt)
		go func() {
			for _ = range signalChan {
				fmt.Println("\nReceived an interrupt, stopping services...\n")
				mp.Close()
				CmdUnmountConsole(container_name)
				cleanupDone <- true
			}
		}()
		<-cleanupDone
	*/

	err = fs.Serve(mp, filesys)
	closeErr := mp.Close()
	if err == nil {
		err = closeErr
	}
	serveErr <- err
	////}()
	//fmt.Printf("serve err %v", serveErr)
	//if serveErr == nil {
	//	Debug("Error", "NO")
	//}
	/*
		select {
		case <-mp.Ready:
			fmt.Printf("Ready %v\n", mp)
			if err := mp.MountError; err != nil {
				return fmt.Errorf("mount fail (delayed): %v", err)
			}
			return nil
		case err := <-serveErr:
			// Serve quit early
			if err != nil {
				return fmt.Errorf("filesystem failure: %v", err)
			}
			return errors.New("Serve exited early")
			//default:
			//	Debug("FUSE", "")
			//	return nil
		}
	*/
	<-mp.Ready
	if err := mp.MountError; err != nil {
		return err
	}
	/*err = fs.Serve(mp, filesys)
	if err != nil {
		fmt.Printf("serve err %v", err)
		return err
	}
	if err := mp.MountError; err != nil {
		fmt.Printf("serve err %v", err)
		return err
	}*/
	return err
}

var _ = fs.NodeRequestLookuper(&Dir{})

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	//logrus.Infof("Lookup d.path = %s", d.path)
	//logrus.Infof("Lookup req.Name = %s", req.Name)
	if req.Name == "CONNECTION_ERROR_CHECK_YOUR_CODEPICNIC_ACCOUNT" {
		child := &File{
			size: 0,
			name: req.Name,
		}
		return child, nil
	}
	path := req.Name
	if d.path != "" {
		path = d.path + "/" + path
	}
	//gnome tried to mount some files like autorun.info , as they not have mimetype should not be created
	//Debug("Lookup PATH", path, d.mimemap[path])
	//Debug("Lookup NAME", path, d.mimemap[req.Name])
	//if strings.HasSuffix(req.Name, ".aaaswp") {
	//	fmt.Printf("Lookup NOENT %v \n", path)
	//	return nil, fuse.ENOENT
	//}
	//if we are not doing a lookup on root
	//if d.mimemap[path] != "" {
	if d.mimemap[path] != "" {
		switch {
		case d.mimemap[path] == "inode/directory":
			//Debug("Lookup DIR", path)
			child := &Dir{
				fs:      d.fs,
				path:    path,
				mimemap: make(map[string]string),
				sizemap: make(map[string]uint64),
			}
			//logrus.Infof("Lookup d dir = %v", d)
			return child, nil
		default:
			//Debug("Lookup FILE", path)
			child := &File{
				size:       d.sizemap[path],
				name:       req.Name,
				path:       path,
				mime:       d.mimemap[path],
				basedir:    d.path,
				fs:         d.fs,
				dir:        d,
				mountpoint: d.mountpoint,
			}
			//logrus.Infof("Lookup d file = %v", d)
			return child, nil
			//}
		}
	}
	//logrus.Infof("Lookup d = %v", d)
	return nil, fuse.ENOENT
}

var _ = fs.HandleReadDirAller(&Dir{})

func CreateErrorInode() fuse.Dirent {
	var inode fuse.Dirent
	inode.Name = "CONNECTION_ERROR_CHECK_YOUR_CODEPICNIC_ACCOUNT"
	inode.Type = fuse.DT_File
	return inode
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	//logrus.Infof("ReadDirAll d.path = %s", d.path)
	var res []fuse.Dirent
	var inode fuse.Dirent
	files_list, err := ListFiles(d.fs.token, d.fs.container, d.path)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
			//Probably the token expired, try again
			logrus.Infof("Token expired, generating a new one")
			d.fs.token, err = GetTokenAccess()
			files_list, err = ListFiles(d.fs.token, d.fs.container, d.path)
		} else {
			res = append(res, CreateErrorInode())
			return res, nil
		}
	} else {

		for _, f := range files_list {
			//Debug("File List", f.name)
			//Debug("File List", f.mime)
			inode.Name = f.name
			if d.mimemap == nil {
				d.mimemap = make(map[string]string)
			}
			if d.sizemap == nil {
				d.sizemap = make(map[string]uint64)
			}
			//_, ok := d.mimemap[f.name]
			//if !ok {
			//	d.mimemap[f.name] = make([]string, "")
			//}
			path := f.name
			if d.path != "" {
				path = d.path + "/" + path
			}
			//d.mimemap[f.name] = f.mime
			d.mimemap[path] = f.mime
			d.sizemap[path] = f.size
			if f.mime == "inode/directory" {
				inode.Type = fuse.DT_Dir
			} else {
				inode.Type = fuse.DT_File
			}
			res = append(res, inode)
		}
		//fmt.Printf("End ReadDirAll \n")
	}
	return res, nil
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	//logrus.Infof("File Attr name = %s, path = %s", f.name, f.path)
	//a.Inode = 1
	//fmt.Printf("File Attr %s %s \n", f.name, f.mime)
	//Debug("File Attr", f.name)
	if f.mime == "inode/directory" {
		a.Mode = os.ModeDir | 0755
	} else {
		a.Mode = 0777
	}
	//a.Size = uint64(len(f.data))
	a.Size = f.size
	/*
		t, _ := f.ReadFile()
		f.content = []byte(t)
		a.Size = uint64(len(t))
	*/
	return nil
}

func (f *File) ReadFile() (string, error) {
	//logrus.Infof("ReadFile name = %s, path = %s", f.name, f.path)
	cp_consoles_url := site + "/api/consoles/" + f.fs.container + "/read_file?path=" + f.path
	//logrus.Infof("cp_consoles_url %v", cp_consoles_url)

	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("read_file %v", err)
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return "", errors.New(ERROR_NOT_AUTHORIZED)
	}
	body, err := ioutil.ReadAll(resp.Body)
	//Debug("ReadFile", string(body))
	return string(body), nil
}

//var _ = fs.NodeOpener(&File{})
var _ fs.NodeOpener = (*File)(nil)

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	//if !req.Flags.IsReadOnly() {
	//	return nil, fuse.Errno(syscall.EACCES)
	//}
	logrus.Infof("Open %s", f.name)
	//logrus.Infof("Open Req %v", req)
	//logrus.Infof("Open Context %v", ctx)
	//logrus.Infof("Open Attr %v", f.Attr)
	/*
		if strings.HasSuffix(f.name, ".swp") {
			fmt.Printf("Open SWP %v\n", f.name)
			resp.Flags |= fuse.OpenDirectIO
		} else {
			resp.Flags |= fuse.OpenKeepCache
		}*/
	resp.Flags |= fuse.OpenKeepCache
	//logrus.Infof("Open Resp %v", resp)
	//logrus.Infof("Open f %v", f)
	//f.writers++
	return f, nil
	//return &FileHandle{path: f.path}, nil
}

var _ fs.Handle = (*File)(nil)

var _ fs.HandleReader = (*File)(nil)

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	//t := f.content.Load().(string)
	t, err := f.ReadFile()
	if err != nil {
		if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
			//Probably the token expired, try again
			logrus.Infof("Token expired, generating a new one")
			f.fs.token, err = GetTokenAccess()
			t, err = f.ReadFile()
		}
	}
	fuseutil.HandleRead(req, resp, []byte(t))
	return nil
}

func (d *Dir) CreateDir(newdir string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/create_folder"
	cp_payload := ` { "path": "` + newdir + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	if err != nil {
		logrus.Errorf("CreateDir %v", err)
		return err
	}
	cache_key := d.fs.container + ":" + d.path
	cp_cache.Delete(cache_key)
	return nil
}

func (d *Dir) CreateFile(newfile string) (err error) {
	logrus.Infof("CreateFile %s", newfile)
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/create_file"
	cp_payload := ` { "path": "` + newfile + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	if err != nil {
		logrus.Errorf("CreateFile %v", err)
		return err
	}
	cache_key := d.fs.container + ":" + d.path
	cp_cache.Delete(cache_key)
	return nil
}

func (d *Dir) RemoveFile(file string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/exec"
	var cp_payload string
	logrus.Infof("Remove file %s", d.path+" / "+file)
	if d.path == "" {
		cp_payload = ` { "commands": "rm ` + file + `" }`
	} else {
		cp_payload = ` { "commands": "rm ` + d.path + "/" + file + `" }`
	}
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		//logrus.Errorf("RemoveFile %v", err)
		return err
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	return nil
}

func (f *File) UploadFile() (err error) {
	cp_consoles_url := site + "/api/consoles/" + f.fs.container + "/upload_file"
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	//logrus.Infof("Upload Data %s", string(f.data))
	temp_file, err := ioutil.TempFile(os.TempDir(), "cp_")
	err = ioutil.WriteFile(temp_file.Name(), f.data, 0666)
	if err != nil {
		logrus.Errorf("Writint temp %v", err)
		return err
	}
	fw, err := w.CreateFormFile("file", temp_file.Name())
	if err != nil {
		logrus.Errorf("CreateFormFile %v", err)
		return err
	}
	if _, err = io.Copy(fw, temp_file); err != nil {
		return
	}
	if fw, err = w.CreateFormField("path"); err != nil {
		return
	}
	if _, err = fw.Write([]byte("/app/" + f.basedir + "/" + f.name)); err != nil {
		return
	}
	w.Close()

	//logrus.Infof("Upload url %s", cp_consoles_url)
	req, err := http.NewRequest("POST", cp_consoles_url, &b)
	if err != nil {
		logrus.Errorf("Upload Request %v \n", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.fs.token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", user_agent)

	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	//logrus.Infof("Upload %s", strconv.Itoa(res.StatusCode))
	//if resp.StatusCode != http.StatusOK {
	//	err = fmt.Errorf("bad status: %s", res.Status)
	//}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	// Delete the resources we created
	err = os.Remove(temp_file.Name())
	if err != nil {
		logrus.Errorf("Remove temp_file %v", err)
	}
	cache_key := f.dir.fs.container + ":" + f.dir.path
	cp_cache.Delete(cache_key)
	return
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	var new_file string
	logrus.Infof("Create %v %s", req.Name, d.path)
	//logrus.Infof("Create Context %v", ctx)
	//logrus.Infof("Create Flags %s", req.Flags.String())
	path := req.Name
	if d.path != "" {
		path = d.path + "/" + path
	}
	f := &File{
		name:    req.Name,
		path:    path,
		writers: 0,
		fs:      d.fs,
		dir:     d,
		basedir: d.path,
	}
	if d.mimemap == nil {
		d.mimemap = make(map[string]string)
	}
	d.mimemap[f.name] = "inode/x-empty"
	if strings.Contains(req.Flags.String(), "OpenExclusive") {
		//logrus.Infof("File %s Exclusive", req.Name)
		f.swap = true
	} else {
		if d.path == "/" {
			new_file = req.Name
			//} else if strings.HasSuffix(f.name, ".swp") {
			//	return f, f, nil
		} else {
			new_file = d.path + "/" + req.Name
		}
		err := d.CreateFile(new_file)
		if err != nil {
			if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
				//Probably the token expired, try again
				logrus.Infof("Token expired, generating a new one")
				d.fs.token, err = GetTokenAccess()
				d.CreateFile(new_file)
			}
		}
	}
	return f, f, nil
}

const maxInt = int(^uint(0) >> 1)

var _ = fs.HandleWriter(&File{})

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.writers = 1
	logrus.Infof("Write %s", f.name)
	//fmt.Printf("Req Data %v \n", req.Data)
	//fmt.Printf("Req Len %v \n", int64(len(req.Data)))
	//fmt.Printf("Req Offset %v \n", req.Offset)
	f.mu.Lock()
	defer f.mu.Unlock()

	// expand the buffer if necessary
	newLen := req.Offset + int64(len(req.Data))
	//fmt.Printf("Req NewLen %v \n", newLen)
	//fmt.Printf("Req Len File %v \n", len(f.data))
	//fmt.Printf("Req Size File %v \n", f.size)
	if newLen > int64(maxInt) {
		//fmt.Printf("Write ERROR %v \n", f.name)
		return fuse.Errno(syscall.EFBIG)
	}

	/*if newLen := int(newLen); newLen > len(f.data) {
		f.data = append(f.data, make([]byte, newLen-len(f.data))...)
	}*/
	//use file size is better than len(f.data)
	if newLen := int(newLen); newLen > int(f.size) {
		f.data = append(f.data, make([]byte, newLen-int(f.size))...)
	} else if newLen < int(f.size) {
		//if newLen is < f.size we need to shrink the slice
		fmt.Printf("Req NewLen %v \n", newLen)
		fmt.Printf("f.data %v \n", f.data)
		//f.data = append([]byte(nil), f.data[:newLen]...)
		f.data = append([]byte(nil), req.Data[:newLen]...)
	}

	n := copy(f.data[req.Offset:], req.Data)
	resp.Size = n
	f.size = uint64(n)
	//fmt.Printf("Resp Size File %v \n", n)
	return nil
}

var _ = fs.HandleFlusher(&File{})

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	//Debug("Flush", f.name, strconv.Itoa(req.Flags))
	logrus.Infof("Flush %v %v", f.name, f.writers)
	//logrus.Infof("Flush Flags %v", req.Flags)
	//Debug("Flush Writers", strconv.Itoa(int(f.writers)))

	if f.writers == 0 {
		// Read-only handles also get flushes. Make sure we don't
		// overwrite valid file contents with a nil buffer.
		//logrus.Infof("Flush Read Only")
		return nil
	}

	//logrus.Infof("Flush Write")
	isSwapFile, _ := regexp.MatchString(`^.+?\.sw.?$`, f.name)
	isBackupFile, _ := regexp.MatchString(`^.+?~$`, f.name)
	is4913, _ := regexp.MatchString(`^4913$`, f.name)
	if isSwapFile == false && isBackupFile == false && is4913 == false {
		err := f.UploadFile()
		if err != nil {
			if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
				//Probably the token expired, try again
				logrus.Infof("Token expired, generating a new one")
				f.fs.token, err = GetTokenAccess()
				f.UploadFile()
			}
		}

	} else {
	}
	//logrus.Infof("Invalidate cache %s", cache_key)
	return nil
}

var _ = fs.HandleReleaser(&File{})

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	//logrus.Infof("Release %v", f.name)
	logrus.Infof("Release %v %v", f.name, f.writers)
	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		//	return nil
	}
	f.writers = 0
	//f.UploadFile()

	return nil
}

var _ = fs.NodeMkdirer(&Dir{})

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	//logrus.Infof("Mkdir %v %v", req.Name, d.path)
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
	err := d.CreateDir(new_dir)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
			//Probably the token expired, try again
			logrus.Infof("Token expired, generating a new one")
			d.fs.token, err = GetTokenAccess()
			d.CreateDir(new_dir)
		}
	}
	n := &Dir{
		fs:   d.fs,
		path: path,
	}
	return n, nil
}

var _ = fs.NodeRemover(&Dir{})

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {

	logrus.Infof("Remove %v %v", req.Name, strconv.FormatBool(req.Dir))
	switch req.Dir {
	case true:
		return fuse.ENOENT

	case false:
		isSwapFile, _ := regexp.MatchString(`^.+?\.sw.?$`, req.Name)
		isBackupFile, _ := regexp.MatchString(`^.+?~$`, req.Name)
		is4913, _ := regexp.MatchString(`^4913$`, req.Name)
		if isSwapFile == false && isBackupFile == false && is4913 == false {
			//logrus.Infof("Remove Normal File %s", req.Name)
			err := d.RemoveFile(req.Name)
			if err != nil {
				if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
					//Probably the token expired, try again
					logrus.Infof("Token expired, generating a new one")
					d.fs.token, err = GetTokenAccess()
					d.RemoveFile(req.Name)
				}
			}
		} else {
			//logrus.Infof("Remove Swap File %s", req.Name)
			//go d.RemoveFile(req.Name)
		}
		cache_key := d.fs.container + ":" + d.path
		cp_cache.Delete(cache_key)
		delete(d.mimemap, req.Name)
		delete(d.sizemap, req.Name)
		return nil
		//return fuse.ENOENT
	}
	return nil
}

//var _ = fs.NodeFsyncer(&Dir{})

func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	//logrus.Infof("Fsync %v %s", req, f.name)
	return nil
}

//var _ = fs.NodeGetattrer(&File{})

func (f *File) GetAttr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	//logrus.Infof("GetAttr %v", req)
	//logrus.Infof("GetAttr Attr %v", f.Attr)
	return f.Attr(ctx, &resp.Attr)
}
