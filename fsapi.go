package main

import (
	"bytes"
	"errors"
	"github.com/Jeffail/gabs"
	"github.com/Sirupsen/logrus"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strings"
)

func (f *File) ReadFile() (string, error) {
	cp_consoles_url := site + "/api/consoles/" + f.dir.fs.container + "/read_file?path=" + f.dir.GetFullFilePath(f.name)

	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.dir.fs.token)
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

//Need to change this to Dir.ListFiles
func (d *Dir) ListFiles() ([]File, error) {
	var FileCollection []File
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/files?path=" + d.GetFullDirPath()
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

func IsOffline(file string) bool {
	var is_offline bool
	//Users may see what appear to be random, zero-byte files appear in their home directory, named 4913, 5036, 5159, 5282 (increasing at increments of 123.)

	offline_regexp := []string{`^.+?\.sw.+$`, `^.+?~$`, `^4913$`, `^\._.+?$`}
	for _, reg := range offline_regexp {
		is_offline, _ = regexp.MatchString(reg, file)
		if is_offline == true {
			return true
		}
	}
	return false
}

func (d *Dir) TouchFile(file string) (err error) {
	new_file := d.GetFullFilePath(file)

	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/exec"
	var cp_payload string
	cp_payload = ` { "commands": "touch ` + new_file + `" }`
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
	return nil
}

//need to merge RemoveDir and RemoveFile
func (d *Dir) RemoveFile(file string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/exec"
	var cp_payload string
	rm_path := d.GetFullFilePath(file)
	cp_payload = ` { "commands": "rm ` + rm_path + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		logrus.Errorf("RemoveFile %v", err)
		return err
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	return nil
}

func (d *Dir) RemoveDir(dir string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/exec"
	var cp_payload string
	rm_dir := d.GetFullFilePath(dir)
	if dir == "" {
		//Avoid remove base directory
		return nil
	} else {
		cp_payload = ` { "commands": "rm -rf /app/` + rm_dir + `" }`
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
		logrus.Errorf("RemoveDir %v", err)
		return err
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	return nil
}

func (d *Dir) MoveFile(src_file string, dst_file string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/exec"
	cp_payload := ` { "commands": "mv ` + src_file + " " + dst_file + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()
	if err != nil {
		logrus.Errorf("RemoveFile %v", err)
		return err
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_NOT_AUTHORIZED)
	}
	return nil
}

func (d *Dir) AsyncCreateDir(dir string) (err error) {
	newdir := d.GetFullFilePath(dir)
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/create_folder"
	cp_payload := ` { "path": "` + newdir + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	req.Header.Set("User-Agent", user_agent)
	Collector(req)
	return nil
}
func (f *File) AsyncUploadFile() error {
	cp_consoles_url := site + "/api/consoles/" + f.dir.fs.container + "/upload_file"
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
	upload_file := f.dir.GetFullFilePath(f.name)
	if _, err = fw.Write([]byte("/app/" + upload_file)); err != nil {
		return err
	}
	w.Close()
	req, err := http.NewRequest("POST", cp_consoles_url, &b)
	if err != nil {
		logrus.Errorf("Upload Request %v \n", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.dir.fs.token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", user_agent)

	Collector(req)
	//where is the file removed ??
	if err != nil {
		logrus.Errorf("Remove temp_file %v", err)
	}
	return nil
}
