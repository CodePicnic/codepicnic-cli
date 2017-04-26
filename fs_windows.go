// Copyright 2016 Keybase Inc. All rights reserved.
// Use of this source code is governed by a BSD
// license that can be found in the LICENSE file.

// +build windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	codepicnic "github.com/CodePicnic/codepicnic-go"
	"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"github.com/keybase/kbfs/dokan"
	"github.com/keybase/kbfs/libdokan"

	"github.com/keybase/kbfs/dokan/winacl"
	"golang.org/x/net/context"
)

func UnmountConsole(container_name string) error {
	dokan.Unmount(container_name)
	return nil
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

	s0 := fsTableStore(emptyFS{}, nil)
	defer fsTableFree(s0)
	fs := newFS(container_name, access_token)
	mount_drive, _ := utf8.DecodeRuneInString(mount_dir)

	mount_driveletter := byte(mount_drive)
	mnt, err := dokan.Mount(&dokan.Config{
		FileSystem: fs,
		Path:       string([]byte{mount_driveletter, ':'}),
		MountFlags: libdokan.DefaultMountFlags,
	})
	if err != nil {
		fmt.Println("Mount failed:", err)
	}
	StartDispatcher(50)
	err = mnt.BlockTillDone()
	if err != nil {
		logrus.Fatal("Filesystem exit:", err)
	}
	defer mnt.Close()
	return nil
}

var _ dokan.FileSystem = emptyFS{}

type emptyFS struct{}
type emptyFile struct{}

type FS struct {
	emptyFS
	container  string
	token      string
	mountpoint string
	state      string
	DirMap     map[string]bool
	SizeMap    map[string]int64
	NodeMap    map[string]dokan.File
	//WaitList   []Operation
}

type File struct {
	dir     *Dir
	name    string
	mime    string
	mu      sync.Mutex
	data    []byte
	writers uint
	new     bool
	size    uint64
	Offline bool
}

type Dir struct {
	emptyFile
	fs            *FS
	name          string
	NodeMap       map[string]Node
	parent        *Dir
	lock          sync.Mutex
	creationTime  time.Time
	lastReadTime  time.Time
	lastWriteTime time.Time
	IsDir         bool
}

type NodeFile struct {
	emptyFile

	name          string
	dir           *Dir
	mime          string
	fs            *FS
	lock          sync.Mutex
	creationTime  time.Time
	lastReadTime  time.Time
	lastWriteTime time.Time
	IsDir         bool
	data          []byte
	writers       uint
	new           bool
	size          uint64
	Offline       bool
}

type Node interface {
	GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error)
}

func (t emptyFile) GetFileSecurity(ctx context.Context, fi *dokan.FileInfo, si winacl.SecurityInformation, sd *winacl.SecurityDescriptor) error {
	return nil
}
func (t emptyFile) SetFileSecurity(ctx context.Context, fi *dokan.FileInfo, si winacl.SecurityInformation, sd *winacl.SecurityDescriptor) error {

	return nil
}
func (t emptyFile) Cleanup(ctx context.Context, fi *dokan.FileInfo) {
	logrus.Info("Cleanup :", fi.Path())
}

func (t emptyFile) CloseFile(ctx context.Context, fi *dokan.FileInfo) {
	logrus.Info("CloseFile :", fi.Path())
}

func (t emptyFS) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, nil
}

func (t emptyFS) GetVolumeInformation(ctx context.Context) (dokan.VolumeInformation, error) {

	return dokan.VolumeInformation{}, nil
}

func (t emptyFS) GetDiskFreeSpace(ctx context.Context) (dokan.FreeSpace, error) {

	return dokan.FreeSpace{}, nil
}

func (t emptyFS) ErrorPrint(err error) {

}

func (t emptyFS) CreateFile(ctx context.Context, fi *dokan.FileInfo, cd *dokan.CreateData) (dokan.File, bool, error) {

	return emptyFile{}, true, nil
}
func (t emptyFile) CanDeleteFile(ctx context.Context, fi *dokan.FileInfo) error {
	return dokan.ErrAccessDenied
}
func (t emptyFile) CanDeleteDirectory(ctx context.Context, fi *dokan.FileInfo) error {
	return dokan.ErrAccessDenied
}
func (t emptyFile) SetEndOfFile(ctx context.Context, fi *dokan.FileInfo, length int64) error {

	return nil
}
func (t emptyFile) SetAllocationSize(ctx context.Context, fi *dokan.FileInfo, length int64) error {

	return nil
}
func (t emptyFS) MoveFile(ctx context.Context, source *dokan.FileInfo, targetPath string, replaceExisting bool) error {

	return nil
}
func (t emptyFile) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	return len(bs), nil
}
func (t emptyFile) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	return len(bs), nil
}
func (t emptyFile) FlushFileBuffers(ctx context.Context, fi *dokan.FileInfo) error {

	return nil
}

func (t emptyFile) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	var st dokan.Stat
	st.FileAttributes = dokan.FileAttributeNormal
	return &st, nil
}
func (t emptyFile) FindFiles(context.Context, *dokan.FileInfo, string, func(*dokan.NamedStat) error) error {
	return nil
}
func (t emptyFile) SetFileTime(context.Context, *dokan.FileInfo, time.Time, time.Time, time.Time) error {
	return nil
}
func (t emptyFile) SetFileAttributes(ctx context.Context, fi *dokan.FileInfo, fileAttributes dokan.FileAttribute) error {
	return nil
}

func (t emptyFile) LockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {
	return nil
}
func (t emptyFile) UnlockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {
	return nil
}

func newFS(container_name string, access_token string) *FS {
	var t FS
	t.container = container_name
	t.token = access_token
	t.DirMap = make(map[string]bool)
	t.SizeMap = make(map[string]int64)
	t.NodeMap = make(map[string]dokan.File)
	var n dokan.File
	n = &Dir{
		fs:   &t,
		name: "",
	}
	t.NodeMap["\\"] = n
	t.DirMap["\\"] = true
	t.SizeMap["\\"] = 4096
	//t.Node = newNode()
	return &t
}

func (fs *FS) CreateFile(ctx context.Context, fi *dokan.FileInfo, cd *dokan.CreateData) (dokan.File, bool, error) {

	path := fi.Path()
	logrus.Info("CreateFile: ", path)
	logrus.Info("CreateFile: ", cd.CreateDisposition)
	switch cd.CreateDisposition {
	case dokan.FileCreate:

	case dokan.FileOpen, dokan.FileOverwriteIf:
		// FileOpen        = CreateDisposition(1) If the file already exists, open it
		//instead of creating a new file. If it does not, fail the request and do
		//not create a new file
		logrus.Info("CreateDisposition: FileOpen", path)

		if node := fs.GetNode(path); node != nil {

			if fs.DirMap[path] {

				if cd.CreateOptions&dokan.FileNonDirectoryFile != 0 {
					return nil, true, dokan.ErrFileIsADirectory
				}
				return node, true, nil

			} else {

				return node, false, nil
			}

		}

	}
	return nil, false, dokan.ErrObjectNameNotFound
}

func (fs *FS) GetNode(name string) dokan.File {

	if fs.NodeMap == nil {
		fs.NodeMap = make(map[string]dokan.File)
		return nil
	}
	return fs.NodeMap[name]
}
func (t *FS) GetDiskFreeSpace(ctx context.Context) (dokan.FreeSpace, error) {
	return dokan.FreeSpace{
		FreeBytesAvailable:     testFreeAvail,
		TotalNumberOfBytes:     testTotalBytes,
		TotalNumberOfFreeBytes: testTotalFree,
	}, nil
}

const (
	// Windows mangles the last bytes of GetDiskFreeSpaceEx
	// because of GetDiskFreeSpace and sectors...
	testFreeAvail  = 0xA234567887654000
	testTotalBytes = 0xB234567887654000
	testTotalFree  = 0xC234567887654000
)

const helloStr = "hello world\r\n"

func (d *Dir) FindFiles(ctx context.Context, fi *dokan.FileInfo, p string, cb func(*dokan.NamedStat) error) error {

	file_list, _ := d.ListFiles(fi.Path())
	for _, f := range file_list {
		logrus.Info("FindFiles: ", f.name)
		var n dokan.File
		st := dokan.NamedStat{}
		st.Name = f.name
		file_attr := dokan.FileAttributeNormal
		var path string
		if fi.Path() == "\\" {
			path = ""
		} else {
			path = fi.Path()
		}
		if f.mime == "inode/directory" {
			file_attr = dokan.FileAttributeDirectory
			d.fs.DirMap[path+"\\"+f.name] = true
			dir_nodemap := make(map[string]Node)
			if d.fs.NodeMap != nil {
				node_dir := d.fs.GetNode(f.name)
				if node_dir != nil {
					dir_nodemap = node_dir.(*Dir).NodeMap
				}
			}
			n = &Dir{
				fs:      d.fs,
				name:    f.name,
				NodeMap: dir_nodemap,
				parent:  d,
			}
		} else {
			d.fs.DirMap[path+"\\"+f.name] = false
			n = &NodeFile{
				name:    f.name,
				dir:     d,
				Offline: false,
				size:    f.size,
				fs:      d.fs,
				mime:    f.mime,
				//data:    []byte(f.name),
			}
		}

		st.Stat = dokan.Stat{
			FileSize:       int64(f.size),
			FileAttributes: file_attr,
		}
		d.fs.SizeMap[path+"\\"+f.name] = st.Stat.FileSize
		d.fs.NodeMap[path+"\\"+f.name] = n
		err := cb(&st)
		if err != nil {
			return err
		}

	}
	logrus.Infof("Dirmap: %+v", d.fs.DirMap)
	logrus.Infof("SizeMap: %+v", d.fs.SizeMap)
	logrus.Infof("NodeMap: %+v", d.fs.NodeMap)
	return nil
}

func GetFullDirPath(path string) string {
	path = strings.TrimPrefix(path, "\\")
	return strings.Replace(path, "\\", "/", -1)

}

func GetFullFilePath(name string) string {
	path := GetFullDirPath(name)
	//if path != "" {
	//	path = path + "/"
	//}
	return path
}

func (d *Dir) ListFiles(path string) ([]File, error) {
	var FileCollection []File
	//cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/files?path=" + d.GetFullDirPath()

	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/files?path=" + GetFullDirPath(path)
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("List files %v", err)
		return FileCollection, errors.New(ERROR_NOT_CONNECTED)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return FileCollection, errors.New(ERROR_NOT_AUTHORIZED)
	}

	body, err := ioutil.ReadAll(resp.Body)
	jsonFiles, err := gabs.ParseJSON(body)
	jsonPaths, _ := jsonFiles.ChildrenMap()
	for key, child := range jsonPaths {
		var jsonFile File
		jsonFile.name = string(key)
		//jsonFile.name = child.Path("path").Data().(string)
		jsonFile.mime = child.Path("type").Data().(string)
		jsonFile.size = uint64(child.Path("size").Data().(float64))
		FileCollection = append(FileCollection, jsonFile)

	}
	return FileCollection, nil
}

func (d *Dir) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	return &dokan.Stat{
		FileAttributes: dokan.FileAttributeDirectory,
	}, nil
}

func (f *NodeFile) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	logrus.Info("GetFileInformation :", fi.Path())
	return &dokan.Stat{
		FileSize: int64(f.size),
	}, nil
}
func (f *NodeFile) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	logrus.Info("ReadFile :", fi.Path())
	logrus.Infof("ReadFile : %+v", f)

	var content string
	if len(f.data) == 0 {
		content, _ = f.ReadFileFromApi(fi.Path())
		newLen := len(content)
		switch {
		case newLen > len(f.data):
			f.data = append(f.data, make([]byte, newLen-len(f.data))...)
		case newLen < len(f.data):
			f.data = f.data[:newLen]
		}
		f.data = []byte(content)

	} else {
		content = string(f.data)

	}
	rd := strings.NewReader(content)
	logrus.Info("ReadFile length: ", len(f.data))
	return rd.ReadAt(bs, offset)
}

func (f NodeFile) ReadFileFromApi(path string) (string, error) {
	cp_consoles_url := site + "/api/consoles/" + f.fs.container + "/read_file?path=" + GetFullDirPath(path)

	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			return "", errors.New(ERROR_DNS_LOOKUP)
		} else {
			return "", err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return "", errors.New(ERROR_NOT_AUTHORIZED)
	}
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), nil
}

/*func newNode() *Node {
	var r Node
	r.creationTime = time.Now()
	r.lastReadTime = r.creationTime
	r.lastWriteTime = r.creationTime
	return &r
}*/
/*
func (r *Node) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	logrus.Info("GetFileInformation :", fi.Path())
	r.lock.Lock()
	defer r.lock.Unlock()
	return &dokan.Stat{
		FileSize:   int64(len(r.data)),
		LastAccess: r.lastReadTime,
		LastWrite:  r.lastWriteTime,
		Creation:   r.creationTime,
	}, nil
}

func (r *Node) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	logrus.Info("ReadFile :", fi.Path())
	r.lock.Lock()
	defer r.lock.Unlock()
	r.lastReadTime = time.Now()
	rd := bytes.NewReader(r.data)
	return rd.ReadAt(bs, offset)
}
*/
func (f *NodeFile) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	logrus.Info("WriteFile :", fi.Path())
	logrus.Info("WriteFile bs: ", string(bs))
	logrus.Info("WriteFile f.data: ", string(f.data))
	f.lock.Lock()
	defer f.lock.Unlock()
	f.lastWriteTime = time.Now()
	maxl := len(f.data)
	if int(offset)+len(bs) > maxl {
		maxl = int(offset) + len(bs)
		f.data = append(f.data, make([]byte, maxl-len(f.data))...)
	} else {
		newLen := offset + int64(len(bs))
		f.data = append([]byte(nil), bs[:newLen]...)

	}
	logrus.Info("WriteFile f.data: ", string(f.data))
	n := copy(f.data[int(offset):], bs)
	logrus.Info("WriteFile :", n)
	f.AsyncUploadFile(fi.Path())
	return n, nil
}

func (f *NodeFile) AsyncUploadFile(path string) error {
	logrus.Info("AsyncUploadFile: ", path)
	cp_consoles_url := site + "/api/consoles/" + f.fs.container + "/upload_file"
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
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
		return err
	}
	if fw, err = w.CreateFormField("path"); err != nil {
		return err
	}
	upload_file := GetFullFilePath(path)
	if _, err = fw.Write([]byte("/app/" + upload_file)); err != nil {
		return err
	}
	w.Close()
	req, err := http.NewRequest("POST", cp_consoles_url, &b)
	if err != nil {
		logrus.Errorf("Upload Request %v \n", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.fs.token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", user_agent)

	Collector(req)
	//where is the file removed ??
	if err != nil {
		logrus.Errorf("Remove temp_file %v", err)
	}
	return nil
}

func (f *NodeFile) SetFileTime(ctx context.Context, fi *dokan.FileInfo, creationTime time.Time, lastReadTime time.Time, lastWriteTime time.Time) error {
	logrus.Info("SetFileTime :", fi.Path())
	f.lock.Lock()
	defer f.lock.Unlock()
	if !lastWriteTime.IsZero() {
		f.lastWriteTime = lastWriteTime
	}
	return nil
}
func (f *NodeFile) SetEndOfFile(ctx context.Context, fi *dokan.FileInfo, length int64) error {
	logrus.Info("SetEndofFile :", fi.Path())
	f.lock.Lock()
	defer f.lock.Unlock()
	f.lastWriteTime = time.Now()
	switch {
	case int(length) < len(f.data):
		f.data = f.data[:int(length)]
	case int(length) > len(f.data):
		f.data = append(f.data, make([]byte, int(length)-len(f.data))...)
	}
	return nil
}
func (f *NodeFile) SetAllocationSize(ctx context.Context, fi *dokan.FileInfo, length int64) error {
	logrus.Info("SetAllocationSize :", fi.Path())
	f.lock.Lock()
	defer f.lock.Unlock()
	f.lastWriteTime = time.Now()
	switch {
	case int(length) < len(f.data):
		f.data = f.data[:int(length)]
	}
	return nil
}
