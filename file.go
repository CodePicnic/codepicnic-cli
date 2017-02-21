package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	//"bytes"
	//"errors"
	//"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	//"io"
	//"io/ioutil"
	//"mime/multipart"
	//"net/http"
	"os"
	//"regexp"
	//"strconv"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const maxInt = int(^uint(0) >> 1)

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

var _ = fs.HandleWriter(&File{})
var _ fs.NodeOpener = (*File)(nil)
var _ fs.Handle = (*File)(nil)
var _ fs.HandleReader = (*File)(nil)
var _ = fs.HandleFlusher(&File{})
var _ = fs.HandleReleaser(&File{})
var _ = fs.NodeFsyncer(&File{})
var _ = fs.NodeSetattrer(&File{})
var _ = fs.NodeSetxattrer(&File{})

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

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	logrus.Debug("Open %+v\n", req)
	//os x can't handle files truncated
	if runtime.GOOS == "darwin" {
		resp.Flags |= fuse.OpenDirectIO
	} else {
		resp.Flags |= fuse.OpenKeepCache
	}
	return f, nil
}

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	logrus.Debug(req)
	//data, err := f.GetDataFromCache()
	/*
		if err != nil {
			logrus.Debug("Read cache f.data ", string(data))
		} else {
			logrus.Debug("Read cache not found f.data ", string(data))
		}*/
	var content string
	var err error

	if f.offline == true {
		content = string(f.data)
	} else {
		content, err = f.ReadFile()
		if err != nil {
			if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
				//Probably the token expired, try again
				f.dir.fs.token, err = GetTokenAccess()
				content, err = f.ReadFile()
			} else if strings.Contains(err.Error(), ERROR_DNS_LOOKUP) {
				return fuse.EINTR
			} else {
				return fuse.EIO
			}
		}
		newLen := len(content)
		switch {
		case newLen > len(f.data):
			f.data = append(f.data, make([]byte, newLen-len(f.data))...)
		case newLen < len(f.data):
			f.data = f.data[:newLen]
		}
		f.data = []byte(content)

	}
	fuseutil.HandleRead(req, resp, []byte(content))
	//f.SaveDataToCache()
	return nil
}

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	logrus.Debug("Write %+v\n", req)
	f.writers = 1
	f.mu.Lock()
	defer f.mu.Unlock()
	//Get f.data from OS cache or from CLI Cache
	//f.GetData()
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

	//copy req.Data to f.data
	_ = copy(f.data[req.Offset:], req.Data)
	//copy f.data to cache
	//f.SaveDataToCache()
	resp.Size = len(req.Data)
	f.size = uint64(newLen)
	//var n fs.Node
	//n.name = f.name
	//n.dtype = fuse.DT_File
	//n.offline = f.dir.nodemap[f.name].offline
	//n.size = f.size
	//f.dir.nodemap[f.name] = n
	//f.dir.AddNode(f.name, f)
	//f.dir.SaveNodemapToCache()
	return nil
}

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	logrus.Debug("Flush %+v\n", req)
	if f.offline == true {
	} else {
		if f.writers == 0 {
			if f.new == true {
				new_file := f.dir.GetFullFilePath(f.name)
				f.new = false
				//err := f.dir.TouchFile(new_file)
				err := f.dir.TouchFile(new_file)
				if err != nil {
					if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
						//Probably the token expired, try again
						f.dir.fs.token, err = GetTokenAccess()
						err = f.dir.TouchFile(new_file)
					}
				}
			}
			// Read-only handles also get flushes. Make sure we don't
			// overwrite valid file contents with a nil buffer.
			return nil
		} else {
			//ch := make(chan error)
			//go f.UploadAsyncFile(ch)
			//err := f.UploadFile()
			f.AsyncUploadFile()
			f.new = false
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
	}
	return nil
}

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	logrus.Debug("Release %+v\n", req)
	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		//  return nil
	}
	f.writers = 0
	//f.UploadFile()

	return nil
}

func (f *File) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	logrus.Debug("FSync %+v\n", req)
	return nil
}

func (f *File) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	logrus.Debug("Setattr ", req)
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Valid.Size() {
		if req.Size > uint64(maxInt) {
			return fuse.Errno(syscall.EFBIG)
		}
		newLen := int(req.Size)
		switch {
		case newLen > len(f.data):
			f.data = append(f.data, make([]byte, newLen-len(f.data))...)
		case newLen < len(f.data):
			f.data = f.data[:newLen]
		}
	}
	//f.SaveDataToCache()
	return nil
}

func (f *File) Setxattr(ctx context.Context, req *fuse.SetxattrRequest) error {
	logrus.Debug("Setxattr ", req)
	return nil
}
