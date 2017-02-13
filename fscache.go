package main

import (
	//"github.com/Sirupsen/logrus"
	"errors"
	"github.com/patrickmn/go-cache"
)

func (f *File) SaveDataToCache() error {
	cache_key := "file:data:" + f.dir.fs.container + ":" + f.dir.path + ":" + f.name
	cp_cache.Set(cache_key, f.data, cache.DefaultExpiration)
	return nil
}

func (d *Dir) DeleteDataFromCache(file string) error {
	cache_key := "file:data:" + d.fs.container + ":" + d.path + ":" + file
	cp_cache.Delete(cache_key)
	return nil
}

func (f *File) GetDataFromCache() ([]byte, error) {
	var file_data []byte
	var err error
	err = nil
	cache_key := "file:data:" + f.dir.fs.container + ":" + f.dir.path + ":" + f.name
	cache_data, found := cp_cache.Get(cache_key)
	if found {
		file_data = cache_data.([]byte)
	} else {
		err = errors.New(ERROR_CACHE_NOT_FOUND)
	}
	return file_data, err

}

func (d *Dir) SaveNodemapToCache() error {
	cache_key := "dir:nodemap:" + d.fs.container + ":" + d.path
	cp_cache.Set(cache_key, d.nodemap, cache.DefaultExpiration)
	return nil
}
func (d *Dir) DeleteNodemapFromCache() error {
	cache_key := "dir:nodemap:" + d.fs.container + ":" + d.path
	cp_cache.Delete(cache_key)
	return nil
}
func (d *Dir) GetNodemapFromCache() (map[string]Node, error) {
	dir_nodemap := make(map[string]Node)
	var err error
	err = nil
	cache_key := "dir:nodemap:" + d.fs.container + ":" + d.path
	cache_nodemap, found := cp_cache.Get(cache_key)
	if found {
		dir_nodemap = cache_nodemap.(map[string]Node)
	} else {
		err = errors.New(ERROR_CACHE_NOT_FOUND)
	}
	return dir_nodemap, err

}
func (f *File) GetData() error {
	var file_data []byte
	var err error
	err = nil
	if len(f.data) == 0 {
		file_data, _ = f.GetDataFromCache()
		f.data = file_data
	}
	f.size = uint64(len(f.data))
	return err

}
func (d *Dir) GetNodemap() error {
	dir_nodemap := make(map[string]Node)
	var err error
	err = nil
	if len(d.nodemap) == 0 {
		dir_nodemap, _ = d.GetNodemapFromCache()
		d.nodemap = dir_nodemap
	}
	return err
}
