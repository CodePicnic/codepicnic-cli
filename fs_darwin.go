package main

import (
	//"context"

	"errors"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/net/context"

	"fmt"
	"os"

	codepicnic "github.com/CodePicnic/codepicnic-go"
	"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"github.com/keybase/kbfs/dokan"
	"github.com/keybase/kbfs/dokan/winacl"
	"github.com/keybase/kbfs/libdokan"
)

type FS struct {
	container  string
	token      string
	mountpoint string
	state      string
	DirMap     map[string]bool
	SizeMap    map[string]int64
	//WaitList   []Operation
}

// EmptyFile is the noop interface for files and directories.
type Node struct {
	IsDir bool
	fs    *FS
	data  []byte
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
	offline bool
}

type Dir struct {
	fs   *FS
	name string
	//NodeMap map[string]fs.Node
	parent *Dir
	read   bool
}

func (d *Dir) GetFullDirPath() string {
	if d.parent == nil {
		return ""
	} else {
		return path.Join(d.parent.GetFullDirPath(), d.name)
	}
}

func (d *Dir) GetFullFilePath(name string) string {
	path := d.GetFullDirPath()
	if path != "" {
		path = path + "/"
	}
	return path + name
}

//GetFileSecurity gets specified information about the security of a file or directory.
func (n *Node) GetFileSecurity(ctx context.Context, fi *dokan.FileInfo, si winacl.SecurityInformation, sd *winacl.SecurityDescriptor) error {
	logrus.Info("GetFileecurity: ", fi.Path())
	return nil
}

//SetFileSecurity sets the security of a file or directory object.
func (n *Node) SetFileSecurity(ctx context.Context, fi *dokan.FileInfo, si winacl.SecurityInformation, sd *winacl.SecurityDescriptor) error {
	logrus.Info("SetFileSecurity: ", fi.Path())
	return nil
}

// Cleanup is called after the last handle from userspace is closed.
// Cleanup must perform actual deletions marked from CanDelete*
// by checking FileInfo.IsDeleteOnClose if the filesystem supports
// deletions.
func (n *Node) Cleanup(ctx context.Context, fi *dokan.FileInfo) {
	logrus.Info("Cleanup: ", fi.Path())
}

// CloseFile is called when closing a handle to the file.
func (n *Node) CloseFile(ctx context.Context, fi *dokan.FileInfo) {
	logrus.Info("CloseFile: ", fi.Path())
}

// CanDeleteFile and CanDeleteDirectory should check whether the file/directory
// can be deleted. The actual deletion should be done by checking
// FileInfo.IsDeleteOnClose in Cleanup.
func (n *Node) CanDeleteFile(ctx context.Context, fi *dokan.FileInfo) error {
	logrus.Info("CanDeleteFile: ", fi.Path())
	return dokan.ErrAccessDenied
}

// CanDeleteDirectory should check whether the file/directory
// can be deleted. The actual deletion should be done by checking
// FileInfo.IsDeleteOnClose in Cleanup.
func (n *Node) CanDeleteDirectory(ctx context.Context, fi *dokan.FileInfo) error {
	logrus.Info("CanDeleteDir: ", fi.Path())
	return dokan.ErrAccessDenied
}

// SetEndOfFile truncates the file.c May be used to extend a file with zeros.
func (n *Node) SetEndOfFile(ctx context.Context, fi *dokan.FileInfo, length int64) error {
	logrus.Info("SetEndofFile: ", fi.Path())
	return nil
}

// SetAllocationSize see FILE_ALLOCATION_INFORMATION on MSDN.
// For simple semantics if length > filesize then ignore else truncate(length).
func (n *Node) SetAllocationSize(ctx context.Context, fi *dokan.FileInfo, length int64) error {
	logrus.Info("SetAllocationSize: ", fi.Path())
	return nil
}

// ReadFile implements read for dokan.
func (n *Node) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	logrus.Info("ReadFile: ", fi.Path())
	logrus.Info("ReadFile length: ", len(bs))
	logrus.Info("ReadFile offset: ", offset)
	len_bs := len(bs)
	stat, _ := n.GetFileInformation(ctx, fi)
	logrus.Infof("ReadFile stat: %+v ", stat)
	content, _ := n.ReadFileFromApi(fi.Path())
	n.data = []byte(content)

	copy(bs, n.data)
	bs = bs[:len(content)]
	//bs = append([]byte(nil), bs[:len(content)]...)
	logrus.Info("ReadFile bs: ", string(n.data))
	logrus.Info("ReadFile length: ", len(content))
	return len_bs, nil
}

func (f *Node) ReadFileFromApi(path string) (string, error) {
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

// WriteFile implements write for dokan.
func (n *Node) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	logrus.Info("WriteFile: ", fi.Path())
	return len(bs), nil
}

// FlushFileBuffers corresponds to fsync.
func (n *Node) FlushFileBuffers(ctx context.Context, fi *dokan.FileInfo) error {
	logrus.Info("FlushFileInformation: ", fi.Path())
	return nil
}

// GetFileInformation - corresponds to stat.
func (n *Node) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	logrus.Info("GetFileInformation: ", fi.Path())
	st := &dokan.Stat{
		//Creation:           time.Now(),                // Timestamps for the file
		//LastAccess:         time.Now(),                // Timestamps for the file
		//LastWrite:          time.Now(),                // Timestamps for the file
		FileSize:           n.fs.SizeMap[fi.Path()],   // FileSize is the size of the file in bytes
		FileIndex:          1000,                      // FileIndex is a 64 bit (nearly) unique ID of the file
		FileAttributes:     dokan.FileAttributeNormal, // FileAttributes bitmask holds the file attributes
		VolumeSerialNumber: 0,                         // VolumeSerialNumber is the serial number of the volume (0 is fine)
		NumberOfLinks:      1,                         // NumberOfLinks can be omitted, if zero set to 1.
		//ReparsePointTag:    0,                         // ReparsePointTag is for WIN32_FIND_DATA dwReserved0 for reparse point tags, typically it can be omitted.
	}
	if n.IsDir {
		st.FileAttributes = dokan.FileAttributeDirectory
	}
	//st.FileAttributes = dokan.FileAttributeNormal
	logrus.Info("GetFileInformation: ", st.FileSize)
	return st, nil
}

// FindFiles is the readdir. The function is a callback that should be called
// with each file. The same NamedStat may be reused for subsequent calls.
//
// Pattern will be an empty string unless UseFindFilesWithPattern is enabled - then
// it may be a pattern like `*.png` to match. All implementations must be prepared
// to handle empty strings as patterns.
func (n *Node) FindFiles(ctx context.Context, fi *dokan.FileInfo, pattern string, fn func(*dokan.NamedStat) error) error {
	logrus.Info("FindFiles Path ", fi.Path())
	var ns dokan.NamedStat
	ns.FileAttributes = dokan.FileAttributeReadonly
	//files := []string{"a", "b", "c", "d"}
	files, _ := n.ListFiles(fi.Path())
	for _, file := range files {

		ns.Name = file.name
		file_attr := dokan.FileAttributeNormal
		var path string
		if fi.Path() == "\\" {
			path = ""
		} else {
			path = fi.Path()
		}
		if file.mime == "inode/directory" {
			file_attr = dokan.FileAttributeDirectory
			n.fs.DirMap[path+"\\"+file.name] = true
		} else {
			n.fs.DirMap[path+"\\"+file.name] = false
		}
		ns.Stat = dokan.Stat{
			FileSize:       int64(file.size),
			FileAttributes: file_attr,
		}
		n.fs.SizeMap[path+"\\"+file.name] = ns.Stat.FileSize
		err := fn(&ns)
		if err != nil {
			return err
		}

	}
	logrus.Infof("Dirmap: %+v", n.fs.DirMap)
	logrus.Infof("SizeMap: %+v", n.fs.SizeMap)
	return nil
}

func GetFullDirPath(path string) string {
	return strings.TrimPrefix(path, "\\")

}

func (d *Node) ListFiles(path string) ([]File, error) {
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

// SetFileTime sets the file time. Test times with .IsZero
// whether they should be set.
func (n *Node) SetFileTime(context.Context, *dokan.FileInfo, time.Time, time.Time, time.Time) error {
	return nil
}

// SetFileAttributes is for setting file attributes.
func (n *Node) SetFileAttributes(ctx context.Context, fi *dokan.FileInfo, fileAttributes dokan.FileAttribute) error {
	return nil
}

// LockFile at a specific offset and data length. This is only used if \ref DOKAN_OPTION_FILELOCK_USER_MODE is enabled.
func (n *Node) LockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {

	return nil
}

//UnlockFile at a specific offset and data lengthh. This is only used if \ref DOKAN_OPTION_FILELOCK_USER_MODE is enabled.
func (n *Node) UnlockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {

	return nil
}

func (fs FS) CreateFile(ctx context.Context, fi *dokan.FileInfo, data *dokan.CreateData) (file dokan.File, isDirectory bool, err error) {
	logrus.Info("CreateFile: ", fi.Path())
	logrus.Infof("CreateFile: %+v", data.FileAttributes)
	switch data.CreateDisposition {
	case dokan.FileSupersede:
		// FileSupersede   = CreateDisposition(0) If the file already exists, replace
		//it with the given file. If it does not, create the given file.

	case dokan.FileOpen:
		// FileOpen        = CreateDisposition(1) If the file already exists, open it
		//instead of creating a new file. If it does not, fail the request and do
		//not create a new file
		logrus.Info("CreateDisposition: FileOpen")
		n := &Node{
			IsDir: false,
			fs:    &fs,
		}
		if fi.Path() == "\\" {
			n.IsDir = true
		}
		//if fs.DirMap[strings.TrimPrefix(fi.Path(), "\\")] {
		if fs.DirMap[fi.Path()] {
			n.IsDir = true
		}
		return n, n.IsDir, nil
	case dokan.FileCreate:
		logrus.Info("CreateDisposition: FileCreate")
		// FileCreate      = CreateDisposition(2) If the file already exists, fail
		//the request and do not create or open the given file. If it does not,
		//create the given file.

	case dokan.FileOpenIf:
		logrus.Info("CreateDisposition: FileOpenIf")
		// FileOpenIf      = CreateDisposition(3) If the file already exists, open
		//it. If it does not, create the given file.

	case dokan.FileOverwrite:
		logrus.Info("CreateDisposition: FileOverwrite")
		// FileOverwrite   = CreateDisposition(4) If the file already exists, open
		//it and overwrite it. If it does not, fail the request.

	case dokan.FileOverwriteIf:
		logrus.Info("CreateDisposition: FileOverwriteIf")
		// FileOverwriteIf = CreateDisposition(5) If the file already exists, open
		//it and overwrite it. If it does not, create the given file.
		var f *Node
		f.IsDir = false
		return f, f.IsDir, nil
	}
	return nil, false, dokan.ErrNotSupported
	//return EmptyFile{}, false, nil
}

func (fs FS) GetVolumeInformation(ctx context.Context) (dokan.VolumeInformation, error) {
	return dokan.VolumeInformation{
		//Maximum file name component length, in bytes, supported by the specified file system. A file name component is that portion of a file name between backslashes.
		MaximumComponentLength: 0xFF, // This can be changed.
		FileSystemFlags: dokan.FileCasePreservedNames | //The file system preserves the case of file names when it places a name on disk.
			dokan.FileCaseSensitiveSearch | //The file system supports case-sensitive file names.
			dokan.FileUnicodeOnDisk | //The file system supports Unicode in file names.
			dokan.FileSupportsReparsePoints | //The file system supports reparse points.
			dokan.FileSupportsRemoteStorage, //The file system supports remote storage.
		FileSystemName: fs.container,
		VolumeName:     fs.container,
	}, nil
}

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
	var filesys FS // Should be the real filesystem implementation
	filesys.container = container_name
	filesys.token = access_token
	filesys.DirMap = make(map[string]bool)
	filesys.SizeMap = make(map[string]int64)
	//var filesys dokan.FileSystem
	mount_drive, _ := utf8.DecodeRuneInString(mount_dir)

	mount_driveletter := byte(mount_drive)
	//logrus.Info("Mount DriveLetter:", string(mount_driveletter))
	mp, err := dokan.Mount(&dokan.Config{
		FileSystem: filesys,
		//Path:       `T:\`,
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
	dokan.Unmount(container_name)
	return nil
}
