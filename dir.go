package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	//"bazil.org/fuse/fuseutil"
	//"fmt"
	"github.com/Sirupsen/logrus"
	//"github.com/patrickmn/go-cache"
	"golang.org/x/net/context"
	"os"
	"path"
	//"runtime"
	"strings"
	//"sync"
	//"syscall"
	"time"
)

type Dir struct {
	fs      *FS
	name    string
	NodeMap map[string]fs.Node
	parent  *Dir
}

var _ = fs.HandleReadDirAller(&Dir{})
var _ = fs.NodeRequestLookuper(&Dir{})
var _ = fs.NodeMkdirer(&Dir{})
var _ = fs.NodeCreater(&Dir{})
var _ = fs.NodeRemover(&Dir{})
var _ fs.NodeRenamer = (*Dir)(nil)

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = 1
	a.Mode = os.ModeDir | 0777
	a.Valid = 5 * time.Minute
	a.Uid = mount_uid
	a.Gid = mount_gid
	return nil
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

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	logrus.Debug("ReadDirAll ", d, ctx)
	var res []fuse.Dirent
	files_list, err := d.ListFiles()
	if err != nil {
		if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
			d.fs.token, err = GetTokenAccess()
			files_list, err = d.ListFiles()
		} else {
			res = append(res, CreateErrorInode())
			return res, nil
		}
	} else {
		for _, f := range files_list {
			var n fs.Node
			if f.mime == "inode/directory" {
				inode := fuse.Dirent{
					Name: f.name,
					Type: fuse.DT_Dir,
				}
				res = append(res, inode)
				//Copy the nodemap from child node, if child/nodemap previously exists
				dir_nodemap := make(map[string]fs.Node)
				if d.NodeMap != nil {
					node_dir := d.GetNode(f.name)
					if node_dir != nil {
						dir_nodemap = node_dir.(*Dir).NodeMap
					}
				}
				n = &Dir{
					fs:      d.fs,
					name:    f.name,
					NodeMap: dir_nodemap,
					//NodeMap: make(map[string]fs.Node),
					parent: d,
				}

			} else {
				inode := fuse.Dirent{
					Name: f.name,
					Type: fuse.DT_Dir,
				}
				res = append(res, inode)
				n = &File{
					name:    f.name,
					dir:     d,
					offline: false,
					size:    f.size,
				}
			}
			d.AddNode(f.name, n)
		}
	}
	//List all offline files from nodemap
	for _, ln := range d.NodeMap {
		switch node_file := ln.(type) {
		case *File:
			if node_file.offline == true {
				inode := fuse.Dirent{
					Name: node_file.name,
					Type: fuse.DT_File,
				}
				res = append(res, inode)
			}
		}
	}
	//d.SaveNodemapToCache()
	return res, nil
}

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	logrus.Debugf("Lookup %s / %s", d.name, req.Name)
	/*
	   if req.Name == "CONNECTION_ERROR_CHECK_YOUR_CODEPICNIC_ACCOUNT" {
	       child := &File{
	           size: 0,
	           name: req.Name,
	       }
	       return child, nil
	   }*/
	//d.GetFullFilePath(req.Name)
	//d.GetNodeMap()

	if node := d.GetNode(req.Name); node != nil {
		return node, nil
	}
	logrus.Debug("Lookup NOENT \n")
	return nil, fuse.ENOENT
}

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	logrus.Debugf("Mkdir %s / %s", d.name, req.Name)
	new_dir := d.GetFullFilePath(req.Name)
	err := d.AsyncCreateDir(new_dir)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
			//Probably the token expired, try again
			d.fs.token, err = GetTokenAccess()
			d.AsyncCreateDir(new_dir)
		} else {
			return nil, fuse.EPERM
		}
	}
	n := &Dir{
		fs:      d.fs,
		name:    req.Name,
		NodeMap: make(map[string]fs.Node),
		parent:  d,
	}
	d.AddNode(req.Name, n)
	//d.SaveNodemapToCache()
	return n, nil
}

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	logrus.Debugf("Create %s / %s", d.name, req.Name)
	/*
		path := req.Name
		if d.path != "" {
			path = d.path + "/" + path
		}*/
	//path := d.GetFullFilePath()
	f := &File{
		name: req.Name,
		//path:    path,
		writers: 0,
		dir:     d,
		new:     true,
	}
	if IsOffline(req.Name) == true {
		f.offline = true
	} else {
		f.offline = false
	}
	d.NodeMap[req.Name] = f
	//d.SaveNodemapToCache()
	return f, f, nil
}

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	logrus.Debugf("Remove %s / %s", d.name, req.Name)
	var err error
	switch req.Dir {
	case true:
		err = d.RemoveDir(req.Name)
		if err != nil {
			if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
				//Probably the token expired, try again
				d.fs.token, err = GetTokenAccess()
				err = d.RemoveDir(req.Name)
			}
		}

	case false:
		if IsOffline(req.Name) == true {
		} else {
			err = d.RemoveFile(req.Name)
			if err != nil {
				if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
					//Probably the token expired, try again
					d.fs.token, err = GetTokenAccess()
					err = d.RemoveFile(req.Name)
				}
			}
		}
		//d.DeleteDataFromCache(req.Name)
	}
	d.RemoveNode(req.Name)
	//d.SaveNodemapToCache()
	return nil
}

func (d *Dir) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	logrus.Debug("Rename ", req, newDir)
	oldPath := d.GetFullFilePath(req.OldName)
	newPath := newDir.(*Dir).GetFullFilePath(req.NewName)
	err := d.MoveFile(oldPath, newPath)
	if err != nil {
		return err
	}
	if node := d.GetNode(req.OldName); node != nil {
		if node_file, ok := node.(*File); ok {
			node_file.name = req.NewName
		} else if node_dir, ok := node.(*Dir); ok {
			node_dir.name = req.NewName
		}
		d.RemoveNode(req.OldName)
		newDir.(*Dir).AddNode(req.NewName, node)
	}
	/*logrus.Debug("Rename ", req, newDir)
	req_create := &fuse.CreateRequest{
		Name:  req.NewName,
		Flags: fuse.OpenWriteOnly + fuse.OpenCreate + fuse.OpenNonblock,
		Mode:  0775,
	}
	resp_create := &fuse.CreateResponse{}
	_, fh, _ := d.Create(ctx, req_create, resp_create)
	switch t := fh.(type) {
	case *File:
		logrus.Debug("Rename FILE")
		f := t
		if d.nodemap[f.name].offline == true {
		} else {
			logrus.Debug("Rename ", f.name)
			d.MoveFile(req.OldName, req.NewName)
			file_handle := *d.nodemap[req.OldName].file
			file_handle.name = req.NewName
			f.new = false
		}
	case *Dir:
		logrus.Debug("Rename DIR")
	default:
		logrus.Debug("Rename NONE")
	}*/
	/*resp_write := &fuse.WriteResponse{}
	  f.Write(ctx, req_write, resp_write)*/
	/*req_remove := &fuse.RemoveRequest{
		Name: req.OldName,
		Dir:  false,
	}
	d.Remove(ctx, req_remove)*/
	return nil
}

func (d *Dir) AddNode(name string, node fs.Node) {

	if d.NodeMap == nil {
		d.NodeMap = make(map[string]fs.Node)
	}

	d.NodeMap[name] = node
}

func (d *Dir) RemoveNode(name string) {

	if d.NodeMap != nil {
		delete(d.NodeMap, name)
	}

}

func (d *Dir) GetNode(name string) fs.Node {

	if d.NodeMap == nil {
		d.NodeMap = make(map[string]fs.Node)
		return nil
	}
	return d.NodeMap[name]
}
