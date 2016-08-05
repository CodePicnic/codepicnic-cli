package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/go-ini/ini"
	"github.com/ryanuber/columnize"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	//"time"
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	"github.com/Jeffail/gabs"
	"syscall"
)

//const site = "https://codepicnic.com"

const site = "https://codeground.xyz"
const FragSeparator = ':'

//const swarm_host = "tcp://52.200.53.168:4000"

const swarm_host = "tcp://54.210.2.20:4000"

type Token struct {
	Access  string `json:"access_token"`
	Type    string `json:"token_type"`
	Expires string `json:"expires_in"`
	Created string `json:"created_at"`
}

type Console struct {
	Url           string `json:"url"`
	ContainerName string `json:"container_name"`
}

type ConsoleExtra struct {
	Id       int
	Title    string
	Size     string
	Type     string
	Hostname string
	Mode     string
}

type ConsoleJson struct {
	Id            int    `json:"id"`
	Content       string `json:"content"`
	Title         string `json:"title"`
	Name          string `json:"name"`
	ContainerName string `json:"container_name"`
	ContainerType string `json:"container_type"`
	CustomImage   string `json:"custom_image"`
	CreatedAt     string `json:"created_at"`
	Permalink     string `json:"permalink"`
	//Url           string `json:"url"`
	//TerminalUrl   string `json:"terminal_url"`
}

type ConsoleCollection struct {
	Consoles []ConsoleJson `json:"consoles"`
}

const CodepicnicAuthServer = "http://127.0.0.1:4001"

func GetCredentialsFromFile() (client_id string, client_secret string) {
	cfg, err := ini.Load(getHomeDir() + "/.codepicnic/credentials")
	if err != nil {
		panic(err)
	}
	client_id = cfg.Section("").Key("client_id").String()
	client_secret = cfg.Section("").Key("client_secret").String()
	return

}
func SaveCredentialsToFile(client_id string, client_secret string) {

	cfg, err := ini.Load(getHomeDir() + "/.codepicnic/credentials")
	if err != nil {
		panic(err)
	}
	cfg.Section("").Key("client_id").SetValue(client_id)
	cfg.Section("").Key("client_secret").SetValue(client_secret)
	//fmt.Println(getHomeDir() + "/.codepicnic/credentials")
	err = cfg.SaveTo(getHomeDir() + "/.codepicnic/credentials")

	if err != nil {
		panic(err)
	}
	return

}

func SaveTokenToFile(access_token string) {

	cfg, err := ini.Load(getHomeDir() + "/.codepicnic/credentials")
	if err != nil {
		panic(err)
	}
	cfg.Section("").Key("access_token").SetValue(access_token)
	//fmt.Println(getHomeDir() + "/.codepicnic/credentials")
	err = cfg.SaveTo(getHomeDir() + "/.codepicnic/credentials")

	if err != nil {
		panic(err)
	}
	return

}

func getHomeDir() string {

	user_data, err := user.Current()
	if err != nil {
		fmt.Println("error")
		panic(err)
	}
	return user_data.HomeDir

}

func GetTokenAccess() (string, error) {
	client_id, client_secret := GetCredentialsFromFile()
	access_token, err := GetTokenAccessFromCredentials(client_id, client_secret)
	return access_token, err
}

func GetTokenAccessFromCredentials(client_id string, client_secret string) (string, error) {

	cp_token_url := site + "/oauth/token"
	//client_id, client_secret = GetCredentialsFromFile()
	cp_payload := `{ "grant_type": "client_credentials","client_id": "` + client_id + `", "client_secret": "` + client_secret + `"}`
	var jsonStr = []byte(cp_payload)
	req, err := http.NewRequest("POST", cp_token_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	//fmt.Println("response Status:", resp.Status)
	//fmt.Println("response Status:", resp.StatusCode)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == 401 {
		return "", errors.New("Not Authorized")
	}
	defer resp.Body.Close()
	var token Token
	_ = json.NewDecoder(resp.Body).Decode(&token)
	return token.Access, nil
}

/*
POST https://codepicnic.com/api/consoles HTTP/1.1
Content-Type: application/json; charset=utf-8

{
  "console": {
    "container_size": "medium",
    "container_type": "bash",
    "hostname": "custom-hostname"
  }
}

*/

func CreateConsole(access_token string, console_extra ConsoleExtra) string {

	cp_consoles_url := site + "/api/consoles"

	//cp_payload := `{ "console:    { "grant_type": "client_credentials","client_id": "` + client_id + `", "client_secret": "` + client_secret + `"}`
	cp_payload := ` { "console": { "container_size": "` + console_extra.Size + `", "container_type": "` + console_extra.Type + `", "title": "` + console_extra.Title + `" , "hostname": "` + console_extra.Hostname + `", "current_mode": "` + console_extra.Mode + `" }  }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return console.ContainerName
}

func ListConsoles(access_token string) []ConsoleJson {

	cp_consoles_url := site + "/api/consoles/all"
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	//fmt.Println(time.Now().Format("20060102150405"))
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	//fmt.Println("response Status:", resp.Status)
	//fmt.Printf("%+v\n", resp)
	var console_collection ConsoleCollection
	//var console_collection []ConsoleJson
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &console_collection)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
	//_ = json.NewDecoder(resp.Body).Decode(&console_collection)
	//fmt.Printf("%+v\n", string(body))
	//fmt.Printf("%#v\n", console_collection.Consoles[0].Title)
	return console_collection.Consoles
}

func StopConsole(access_token string, container_name string) {

	cp_consoles_url := site + "/api/consoles/" + container_name + "/stop"
	req, err := http.NewRequest("POST", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return
}

func StartConsole(access_token string, container_name string) {

	cp_consoles_url := site + "/api/consoles/" + container_name + "/start"
	req, err := http.NewRequest("POST", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return
}

func RestartConsole(access_token string, container_name string) {

	cp_consoles_url := site + "/api/consoles/" + container_name + "/restart"
	req, err := http.NewRequest("POST", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return
}

func ProxyConsole(access_token string, container_name string) string {

	cp_connect_url := CodepicnicAuthServer + "/connect/" + container_name
	req, err := http.NewRequest("GET", cp_connect_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("%+v\n", err)
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body)
}

func ConnectConsole(access_token string, container_name string) {

	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient(swarm_host, "v1.22", nil, defaultHeaders)
	if err != nil {
		panic(err)
	}
	//r, err := cli.ContainerInspect(context.Background(), container_name)
	r, err := cli.ContainerExecCreate(context.Background(), container_name, types.ExecConfig{User: "", Cmd: []string{"bash"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	//fmt.Println(r.ID)

	aResp, err := cli.ContainerExecAttach(context.Background(), r.ID, types.ExecConfig{Tty: true, Detach: false})

	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	tty := true
	if err != nil {
		log.Fatalf("Couldn't attach to container: %s", err)
	}
	defer aResp.Close()
	receiveStdout := make(chan error, 1)
	if os.Stdout != nil || os.Stderr != nil {
		go func() {
			// When TTY is ON, use regular copy
			if tty && os.Stdout != nil {
				_, err = io.Copy(os.Stdout, aResp.Reader)
			} else {
				_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, aResp.Reader)
			}
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if os.Stdin != nil {
			io.Copy(aResp.Conn, os.Stdin)
		}

		if err := aResp.CloseWrite(); err != nil {
			log.Fatalf("Couldn't send EOF: %s", err)
		}
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			log.Fatalf("Error receiveStdout: %s", err)
		}
	case <-stdinDone:
		if os.Stdout != nil || os.Stderr != nil {
			if err := <-receiveStdout; err != nil {
				log.Fatalf("Error receiveStdout: %s", err)
			}
		}
	}

	return
}

type FS struct {
	container  string
	token      string
	file       *File
	mountpoint string
}

func (f *FS) Root() (fs.Node, error) {
	//fmt.Printf("FS Root \n")
	node_dir := &Dir{
		container:  f.container,
		token:      f.token,
		path:       "/",
		mountpoint: f.mountpoint,
	}
	return node_dir, nil
}

type Dir struct {
	container  string
	token      string
	path       string
	mime       string
	mountpoint string
	fs         *FS
}

type File struct {
	name       string
	path       string
	mime       string
	container  string
	token      string
	mountpoint string
	mu         sync.Mutex
	content    []byte
	data       []byte
	writers    uint
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	//fmt.Printf("Dir Attr %s \n", d.path)
	a.Mode = os.ModeDir | 0755
	return nil
}

func ListFiles(access_token string, container_name string, path string) []File {
	cp_consoles_url := site + "/api/consoles/" + container_name + "/files?path=" + path
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	jsonFiles, err := gabs.ParseJSON(body)
	jsonPaths, _ := jsonFiles.Search("paths").ChildrenMap()
	jsonTypes, _ := jsonFiles.Search("types").ChildrenMap()
	var FileCollection []File
	for key, child := range jsonPaths {
		var jsonFile File
		jsonFile.name = string(key)
		jsonFile.path = child.Data().(string)
		jsonFile.mime = jsonTypes[key].Data().(string)
		FileCollection = append(FileCollection, jsonFile)
		fmt.Printf("key: %v, value: %v, type: %v\n", key, child.Data().(string), jsonTypes[key])
		fmt.Printf("key: %v, value: %v, type: %v\n", jsonFile.name, jsonFile.path, jsonFile.mime)

	}
	//for key, child := range jsonTypes {
	//	fmt.Printf("key: %v, value: %v\n", key, child.Data().(string))
	//}
	//_ = json.NewDecoder(resp.Body).Decode(&console_collection)
	//fmt.Printf("%+v\n", string(body))
	//fmt.Printf("%#v\n", console_collection.Consoles[0].Title)
	return FileCollection
}

func MountConsole(access_token string, container_name string, mount_dir string) error {
	fmt.Println("Mount")
	mp, err := fuse.Mount(mount_dir+"/"+container_name, fuse.MaxReadahead(32*1024*1024),
		fuse.AsyncRead())
	if err != nil {
		fmt.Printf("serve err %v", err)
		return err
	}
	defer mp.Close()
	fmt.Println("Filesys")
	filesys := &FS{
		token:      access_token,
		container:  container_name,
		mountpoint: mount_dir + "/" + container_name,
	}
	fmt.Println("Serve")
	srv := fs.New(mp, &fs.Config{})

	serveErr := make(chan error, 1)

	go func() {
		defer mp.Close()
		//serveErr <- fs.Serve(mp, filesys)
		serveErr <- srv.Serve(filesys)
	}()
	fmt.Printf("serve err %v", serveErr)

	select {
	case <-mp.Ready:
		fmt.Println("Ready")
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
	}

	/*err = fs.Serve(mp, filesys)
	if err != nil {
		fmt.Printf("serve err %v", err)
		return err
	}
	if err := mp.MountError; err != nil {
		fmt.Printf("serve err %v", err)
		return err
	}
	return err*/
}

var _ = fs.NodeRequestLookuper(&Dir{})

func (d *Dir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {
	//path := req.Name
	//fmt.Printf("Lookup %v \n", req.Name)
	for _, f := range ListFiles(d.token, d.container, "") {
		if req.Name == f.name {
			//fmt.Printf("Switch %s %s \n", f.name, f.mime)
			switch {
			case f.mime == "inode/directory":
				//fmt.Printf("Case DIR %v \n", f.name)
				child := &Dir{
					container: d.container,
					token:     d.token,
					path:      req.Name,
				}
				return child, nil
			default:
				fmt.Printf("Case FILE %v \n", f.name)
				fmt.Printf("Case FILE paht %v \n", f.path)
				child := &File{
					name: f.name,
					mime: f.mime,
					//path:       f.path,
					container:  d.container,
					token:      d.token,
					mountpoint: d.mountpoint,
				}
				return child, nil
			}
		}
	}
	//child := &File{
	//	name: req.Name,
	//}
	//return child, nil
	//d.fs.file.name = req.Name
	//return d.fs.file, nil
	return nil, fuse.ENOENT
	//if d.file != nil {
	//	path = d.file.Name + path
	//}
	//for _, f := range d.archive.File {
	//	switch {
	//	case f.Name == path:
	//		child := &File{
	//			file: f,
	//		}
	//		return child, nil
	//	case f.Name[:len(f.Name)-1] == path && f.Name[len(f.Name)-1] == '/':
	//		child := &Dir{
	//			archive: d.archive,
	//			file:    f,
	//		}
	//		return child, nil
	//	}
	//}
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var dirDirs []fuse.Dirent
	var inode fuse.Dirent
	//fmt.Printf("Start ReadDirAll \n")
	for _, f := range ListFiles(d.token, d.container, "") {
		//fmt.Printf("%s \n", f.name)

		//fmt.Printf("File List %s %s \n", f.name, f.mime)
		inode.Name = f.name
		if f.mime == "inode/directory" {
			inode.Type = fuse.DT_Dir
		} else {
			inode.Type = fuse.DT_File
		}
		dirDirs = append(dirDirs, inode)
	}
	//fmt.Printf("End ReadDirAll \n")
	return dirDirs, nil
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	//a.Inode = 1
	//fmt.Printf("File Attr %s %s \n", f.name, f.mime)
	if f.mime == "inode/directory" {
		a.Mode = os.ModeDir | 0755
	} else {
		a.Mode = 0644
	}
	t, _ := f.ReadFile()
	f.content = []byte(t)
	a.Size = uint64(len(t))
	return nil
}

func (f *File) ReadFile() (string, error) {
	cp_consoles_url := site + "/api/consoles/" + f.container + "/" + f.name
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), nil
}

//var _ = fs.NodeOpener(&File{})
var _ fs.NodeOpener = (*File)(nil)

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	//if !req.Flags.IsReadOnly() {
	//	return nil, fuse.Errno(syscall.EACCES)
	//}
	fmt.Printf("Open \n")
	resp.Flags |= fuse.OpenKeepCache
	f.writers++
	return f, nil
	//return &FileHandle{path: f.path}, nil
}

var _ fs.Handle = (*File)(nil)

var _ fs.HandleReader = (*File)(nil)

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	//t := f.content.Load().(string)
	t, _ := f.ReadFile()
	fuseutil.HandleRead(req, resp, []byte(t))
	return nil
}

var _ = fs.HandleWriter(&File{})

func (d *Dir) CreateDir(newdir string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.container + "/create_folder"
	cp_payload := ` { "path": "` + newdir + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (d *Dir) CreateFile(newfile string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.container + "/create_file"
	cp_payload := ` { "path": "` + newfile + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (f *File) UploadFile() (err error) {
	cp_consoles_url := site + "/api/consoles/" + f.container + "/upload_file"
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	// Add your image file
	fmt.Printf("Upload Path %v \n", f.path)
	fmt.Printf("Upload Name %v \n", f.name)
	//fh, err := os.Open(f.mountpoint + "/" + f.path)
	//fmt.Printf("Open %v \n", f.mountpoint+"/"+f.path)
	//if err != nil {
	//	fmt.Printf("Error 1 %v \n", err)
	//	return err
	//}
	//defer fh.Close()
	temp_file, err := ioutil.TempFile(os.TempDir(), "cp_")
	err = ioutil.WriteFile(temp_file.Name(), f.data, 0644)
	if err != nil {
		fmt.Printf("Error writint temp %v", err)
		return err
	}
	fw, err := w.CreateFormFile("file", temp_file.Name())
	if err != nil {
		fmt.Printf("Error 2 %v \n", err)
		return err
	}
	if _, err = io.Copy(fw, temp_file); err != nil {
		return
	}
	// Add the other fields
	if fw, err = w.CreateFormField("path"); err != nil {
		return
	}
	if _, err = fw.Write([]byte("/app/" + f.path + "/" + f.name)); err != nil {
		return
	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", cp_consoles_url, &b)
	if err != nil {
		fmt.Printf("Error 3 %v \n", err)
		return err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	fmt.Printf("Upload %v \n", res.StatusCode)
	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	// Delete the resources we created
	err = os.Remove(temp_file.Name())
	if err != nil {
		log.Fatal(err)
	}
	return
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	fmt.Printf("Create %v %v \n", req.Name, d.path)
	f := &File{
		container: d.container,
		name:      req.Name,
		path:      d.path,
		token:     d.token,
		writers:   1,
	}
	if d.path == "/" {
		d.CreateFile(req.Name)
	} else {
		d.CreateFile(d.path + "/" + req.Name)
	}
	return f, f, nil
}

const maxInt = int(^uint(0) >> 1)

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fmt.Printf("Write \n")
	f.mu.Lock()
	defer f.mu.Unlock()

	// expand the buffer if necessary
	newLen := req.Offset + int64(len(req.Data))
	if newLen > int64(maxInt) {
		return fuse.Errno(syscall.EFBIG)
	}
	if newLen := int(newLen); newLen > len(f.data) {
		f.data = append(f.data, make([]byte, newLen-len(f.data))...)
	}

	n := copy(f.data[req.Offset:], req.Data)
	resp.Size = n
	return nil
}

var _ = fs.HandleFlusher(&File{})

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	//fmt.Printf("Flush %v \n", req.Flags)

	if f.writers == 0 {
		// Read-only handles also get flushes. Make sure we don't
		// overwrite valid file contents with a nil buffer.
		fmt.Printf("Flush Read Only \n")
		return nil
	}

	fmt.Printf("Flush Write \n")
	f.UploadFile()
	return nil
}

var _ = fs.HandleReleaser(&File{})

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fmt.Printf("Release %v \n", req.Flags)
	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		//	return nil
	}
	f.writers--
	//f.UploadFile()

	return nil
}

var _ = fs.NodeMkdirer(&Dir{})

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	fmt.Printf("Mkdir %v %v \n", req.Name, d.path)
	if d.path == "/" {
		d.CreateDir(req.Name)
	} else {
		d.CreateDir(d.path + "/" + req.Name)
	}
	n := &Dir{
		fs:        d.fs,
		path:      req.Name,
		container: d.container,
		token:     d.token,
	}
	return n, nil
}

func main() {
	app := cli.NewApp()
	app.Version = "0.1.0"
	app.Name = "codepicnic-cli"
	app.Usage = "A CLI tool to manage your CodePicnic consoles"
	var container_size, container_type, title, hostname, current_mode string

	app.Commands = []cli.Command{
		{
			Name: "create",
			//Aliases: []string{"c"},
			Usage: "create and start a new console",

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "size",
					Value:       "medium",
					Usage:       "Container Size",
					Destination: &container_size,
				},
				cli.StringFlag{
					Name:        "type",
					Value:       "bash",
					Usage:       "Container Type",
					Destination: &container_type,
				},
				cli.StringFlag{
					Name:        "title",
					Value:       "",
					Usage:       "Pick a name for your console. Make it personal!",
					Destination: &title,
				},

				cli.StringFlag{
					Name:        "hostname",
					Value:       "",
					Usage:       "Any name you'd like to be used as your console hostname.",
					Destination: &hostname,
				},

				cli.StringFlag{
					Name:        "mode",
					Value:       "draft",
					Usage:       "The mode the console is currently in.",
					Destination: &current_mode,
				},
			},

			Action: func(c *cli.Context) error {
				access_token, err := GetTokenAccess()
				if err != nil {
					fmt.Println("Error: ", err)
				}
				var console ConsoleExtra

				console.Size = container_size
				console.Type = container_type
				console.Title = title
				console.Hostname = hostname
				console.Mode = current_mode

				container_name := CreateConsole(access_token, console)
				fmt.Println(container_name)
				return nil
			},
		},
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "list consoles",
			Action: func(c *cli.Context) error {
				access_token, err := GetTokenAccess()
				SaveTokenToFile(access_token)
				if err != nil {
					fmt.Println("Error: ", err)
					panic(err)
				}

				consoles := ListConsoles(access_token)
				//fmt.Printf("%#v\n", consoles[0].Title)

				output := []string{
					"ID | TITLE | CONTAINER NAME | CONTAINER TYPE | CREATED | URL",
				}
				for i := range consoles {
					console_cols := strconv.Itoa(consoles[i].Id) + "|" + consoles[i].Title + "|" + consoles[i].ContainerName + "|" + consoles[i].ContainerType + "|" + consoles[i].CreatedAt + "|" + site + "/consoles/" + consoles[i].Permalink
					output = append(output, console_cols)
				}
				result := columnize.SimpleFormat(output)
				fmt.Println(result)
				return nil
			},
		},
		{
			Name:  "stop",
			Usage: "stop a console",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StopConsole(access_token, c.Args()[0])
				return nil
			},
		},
		{
			Name:  "start",
			Usage: "start a console",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StartConsole(access_token, c.Args()[0])
				return nil
			},
		},
		{
			Name:  "restart",
			Usage: "restart a console",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				RestartConsole(access_token, c.Args()[0])
				return nil
			},
		},
		{
			Name:  "configure",
			Usage: "save configuration",
			Action: func(c *cli.Context) error {
				reader := bufio.NewReader(os.Stdin)
				fmt.Print("Client ID: ")
				input_id, _ := reader.ReadString('\n')
				reader_secret := bufio.NewReader(os.Stdin)
				fmt.Print("Client Secret: ")
				input_secret, _ := reader_secret.ReadString('\n')
				fmt.Print("Please wait, testing credentials... ")
				client_id := strings.Trim(input_id, "\n")
				client_secret := strings.Trim(input_secret, "\n")
				access_token, err := GetTokenAccessFromCredentials(client_id, client_secret)
				if err != nil {
					fmt.Println("Error: ", err)
					panic(err)
				}
				fmt.Println("Token: ", access_token)
				fmt.Print("Please wait, saving credentials... ")
				SaveCredentialsToFile(client_id, client_secret)
				SaveTokenToFile(access_token)
				fmt.Println("Credentials saved \n")
				return nil
			},
		},
		{
			Name:  "connect",
			Usage: "connect to a console",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StartConsole(access_token, c.Args()[0])
				ConnectConsole(access_token, c.Args()[0])
				//fmt.Println(ProxyConsole(access_token, c.Args()[0]))
				return nil
			},
		},
		{
			Name:  "mount",
			Usage: "mount /app filesystem from a container",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StartConsole(access_token, c.Args()[0])
				MountConsole(access_token, c.Args()[0], c.Args()[1])
				/*if err != nil {
					fmt.Println("Error: ", err)
					panic(err)
				}*/
				return nil
			},
		},
		{
			Name:  "files",
			Usage: "list files from a container",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StartConsole(access_token, c.Args()[0])
				ListFiles(access_token, c.Args()[0], "")
				return nil
			},
		},
		{
			Name:  "cat",
			Usage: "cat contents from file",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StartConsole(access_token, c.Args()[0])
				//ReadFile(access_token, c.Args()[0], "")
				return nil
			},
		},
		{
			Name:  "put",
			Usage: "put contents to file",
			Action: func(c *cli.Context) error {
				access_token, _ := GetTokenAccess()
				StartConsole(access_token, c.Args()[0])
				//UploadFile(access_token, c.Args()[0], c.Args()[1], c.Args()[2])
				return nil
			},
		},
	}
	app.Run(os.Args)
}
