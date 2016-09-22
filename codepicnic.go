package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/go-ini/ini"
	//"github.com/russmack/replizer"
	"github.com/ryanuber/columnize"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"text/tabwriter"
	"text/template"
	//"os/signal"
	"os/user"
	"strconv"
	"strings"
	"sync"
	//"time"
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	"github.com/Jeffail/gabs"
	//"github.com/fatih/color"
	"github.com/kardianos/osext"
	"github.com/patrickmn/go-cache"
	"path/filepath"
	"syscall"
	"time"
)

//const site = "https://codepicnic.com"

const FragSeparator = ':'
const cfg_dir = ".codepicnic"
const cfg_file = "config"

var version string
var site string
var swarm_host string
var format string

//const site = "https://codeground.xyz"

//const swarm_host = "tcp://52.200.53.168:4000"

//const swarm_host = "tcp://54.88.32.109:4000"

var debug = true

var cp_cache = cache.New(5*time.Minute, 30*time.Second)

// https://github.com/docker/docker/blob/master/cli/command/container/cp.go
func splitContainerFromPath(arg string) (container, path string) {
	if system.IsAbs(arg) {
		return "", arg
	}

	parts := strings.SplitN(arg, ":", 2)

	if len(parts) == 1 || strings.HasPrefix(parts[0], ".") {
		return "", arg
	}

	return parts[0], parts[1]
}

func Debug(print string, values ...string) error {
	if debug {
		fmt.Printf("DEBUG %s %v \n", print, values)
	}
	return nil
}

func CreateConfigDir() {
	config_dir := getHomeDir() + string(filepath.Separator) + cfg_dir
	config_file := config_dir + string(filepath.Separator) + cfg_file
	os.Mkdir(config_dir, 0755)
	if _, err := os.Stat(config_file); os.IsNotExist(err) {
		f, err := os.Create(config_file)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
}

func GetCredentialsFromFile() (client_id string, client_secret string) {
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		return
	}
	client_id = cfg.Section("credentials").Key("client_id").String()
	client_secret = cfg.Section("credentials").Key("client_secret").String()
	return

}

func SaveCredentialsToFile(client_id string, client_secret string) {
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		panic(err)
	}
	cfg.Section("credentials").Key("client_id").SetValue(client_id)
	cfg.Section("credentials").Key("client_secret").SetValue(client_secret)
	//fmt.Println(getHomeDir() + "/.codepicnic/credentials")
	err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)

	if err != nil {
		panic(err)
	}
	return

}

func SaveMountsToFile(container string, mountpoint string) {

	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		panic(err)
	}
	cfg.Section("mounts").Key(container).SetValue(mountpoint)
	err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)

	if err != nil {
		panic(err)
	}
	return

}

func GetMountsFromFile(container string) string {
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		panic(err)
	}
	mountpoint := cfg.Section("mounts").Key(container).String()
	return mountpoint

}

func SaveTokenToFile(access_token string) {

	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		panic(err)
	}
	cfg.Section("credentials").Key("access_token").SetValue(access_token)
	err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)

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

func CreateConsole(access_token string, console_extra ConsoleExtra) (string, string) {

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
	return console.ContainerName, console.Url
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

/*
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
}*/
func ConnectConsole(access_token string, container_name string) {

	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient(swarm_host, "v1.22", nil, defaultHeaders)
	if err != nil {
		fmt.Println("e1", err)
		panic(err)
	}
	//r, err := cli.ContainerInspect(context.Background(), container_name)
	r, err := cli.ContainerExecCreate(context.Background(), container_name, types.ExecConfig{User: "", Cmd: []string{"bash"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	if err != nil {
		fmt.Println("e2", err)
		panic(err)
	}
	//fmt.Println(r.ID)

	aResp, err := cli.ContainerExecAttach(context.Background(), r.ID, types.ExecConfig{Tty: true, Detach: false})

	if err != nil {
		fmt.Println("e3", err)
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
			fmt.Printf("stdinDone\n")
		}

		if err := aResp.CloseWrite(); err != nil {
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				//Connection already closed
			} else {
				log.Fatalf("Couldn't send EOF: %s", err)
			}
		}
		//close(stdinDone)
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
	close(stdinDone)
	//stdinw := bufio.NewReader(os.Stdin)
	fmt.Printf("done\n")
	aResp.Conn.Close()
	return
}

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
	content    []byte
	data       []byte
	writers    uint
	fs         *FS
	size       uint64
	dir        *Dir
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	//fmt.Printf("Dir Attr %s \n", d.path)
	//Debug("Dir Attr", d.path)
	a.Mode = os.ModeDir | 0777
	return nil
}

func ListFiles(access_token string, container_name string, path string) []File {
	cache_key := container_name + ":" + path
	var FileCollection []File
	FileCollectionCache, found := cp_cache.Get(cache_key)
	if found {
		FileCollection = FileCollectionCache.([]File)
	} else {
		cp_consoles_url := site + "/api/consoles/" + container_name + "/files?path=" + path
		Debug("list files", cp_consoles_url)
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
		Debug("Set Cache", cache_key)
		cp_cache.Set(cache_key, FileCollection, cache.DefaultExpiration)
		//for key, child := range jsonTypes {
		//	fmt.Printf("key: %v, value: %v\n", key, child.Data().(string))
		//}
		//_ = json.NewDecoder(resp.Body).Decode(&console_collection)
		//fmt.Printf("%+v\n", string(body))
		//fmt.Printf("%#v\n", console_collection.Consoles[0].Title)
	}
	return FileCollection
}

//func UnmountConsole(access_token string, container_name string, mount_dir string) error {
func UnmountConsole(access_token string, container_name string) error {
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

			fmt.Printf(color("Container %s succesfully unmounted\n", "response"), container_name)
			SaveMountsToFile(container_name, "")
		}
	}
	return nil
}
func debugLog(msg interface{}) {
	fmt.Printf("%s", msg)
}
func MountConsole(access_token string, container_name string, mount_dir string) error {
	var mount_point string
	//var wg sync.WaitGroup

	if mount_dir == "" {
		mount_point = container_name
		os.Mkdir(container_name, 0755)
	} else {
		mount_point = mount_dir + "/" + container_name
		os.Mkdir(mount_dir+"/"+container_name, 0755)
	}
	//Debug("MountPoint", mount_point)
	mp, err := fuse.Mount(mount_point, fuse.MaxReadahead(32*1024*1024),
		//fuse.AsyncRead(), fuse.WritebackCache())
		fuse.AsyncRead())
	if err != nil {
		fmt.Printf("serve err %v", err)
		return err
	}
	defer mp.Close()
	filesys := &FS{
		token:      access_token,
		container:  container_name,
		mountpoint: mount_point,
	}
	Debug("Serve", "")
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
	cfg := &fs.Config{
		WithContext: func(ctx context.Context, req fuse.Request) context.Context {
			return ctx
		},
	}
	cfg.Debug = debugLog
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
			return child, nil
			//}
		}
	}
	return nil, fuse.ENOENT
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent
	var inode fuse.Dirent
	for _, f := range ListFiles(d.fs.token, d.fs.container, d.path) {
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
	return res, nil
}

var _ fs.Node = (*File)(nil)

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	//a.Inode = 1
	//fmt.Printf("File Attr %s %s \n", f.name, f.mime)
	//Debug("File Attr", f.name)
	if f.mime == "inode/directory" {
		a.Mode = os.ModeDir | 0755
	} else {
		a.Mode = 0777
	}
	a.Size = f.size
	/*
		t, _ := f.ReadFile()
		f.content = []byte(t)
		a.Size = uint64(len(t))
	*/
	return nil
}

func (f *File) ReadFile() (string, error) {
	Debug("ReadFile", f.name, f.path)
	cp_consoles_url := site + "/api/consoles/" + f.fs.container + "/" + f.path
	Debug("cp_consoles_url", cp_consoles_url)

	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.fs.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//Debug("ReadFile", string(body))
	return string(body), nil
}

func DownloadFileFromConsole(token string, console_id string, src string, dst string) error {
	if dst == "" {
		dst = src
	}
	cp_consoles_url := site + "/api/consoles/" + console_id + "/" + src

	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	ioutil.WriteFile(dst, body, 0644)
	return nil
}

//var _ = fs.NodeOpener(&File{})
var _ fs.NodeOpener = (*File)(nil)

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	//if !req.Flags.IsReadOnly() {
	//	return nil, fuse.Errno(syscall.EACCES)
	//}
	Debug("Open", f.name)
	/*
		if strings.HasSuffix(f.name, ".swp") {
			fmt.Printf("Open SWP %v\n", f.name)
			resp.Flags |= fuse.OpenDirectIO
		} else {
			resp.Flags |= fuse.OpenKeepCache
		}*/
	resp.Flags |= fuse.OpenKeepCache
	//f.writers++
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

func (d *Dir) CreateDir(newdir string) (err error) {
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/create_folder"
	cp_payload := ` { "path": "` + newdir + `" }`
	var jsonStr = []byte(cp_payload)

	Debug("Create Dir", newdir)
	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	cache_key := d.fs.container + ":" + d.path
	cp_cache.Delete(cache_key)
	defer resp.Body.Close()
	return nil
}

func (d *Dir) CreateFile(newfile string) (err error) {
	Debug("CreateFile", newfile)
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/create_file"
	cp_payload := ` { "path": "` + newfile + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	cache_key := d.fs.container + ":" + d.path
	cp_cache.Delete(cache_key)
	defer resp.Body.Close()
	return nil
}

func (d *Dir) RemoveFile(file string) (err error) {
	Debug("RemoveFile", file)
	cp_consoles_url := site + "/api/consoles/" + d.fs.container + "/exec"
	cp_payload := ` { "commands": "rm ` + file + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.fs.token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (f *File) UploadFile() (err error) {
	cp_consoles_url := site + "/api/consoles/" + f.fs.container + "/upload_file"
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	Debug("Upload Path", f.basedir)
	Debug("Upload Name", f.name)
	Debug("Upload Data", string(f.data))
	temp_file, err := ioutil.TempFile(os.TempDir(), "cp_")
	err = ioutil.WriteFile(temp_file.Name(), f.data, 0666)
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
	if fw, err = w.CreateFormField("path"); err != nil {
		return
	}
	if _, err = fw.Write([]byte("/app/" + f.basedir + "/" + f.name)); err != nil {
		return
	}
	w.Close()

	Debug("Upload url", cp_consoles_url)
	req, err := http.NewRequest("POST", cp_consoles_url, &b)
	if err != nil {
		fmt.Printf("Error 3 %v \n", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+f.fs.token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	Debug("Upload", strconv.Itoa(res.StatusCode))
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

func UploadFileToConsole(token string, console_id string, dst string, src string) (err error) {
	if dst == "" {
		dst = src
	}
	cp_consoles_url := site + "/api/consoles/" + console_id + "/upload_file"
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	temp_file, err := os.Open(src)
	fw, err := w.CreateFormFile("file", temp_file.Name())
	if _, err = io.Copy(fw, temp_file); err != nil {
		return
	}
	if fw, err = w.CreateFormField("path"); err != nil {
		return
	}
	if _, err = fw.Write([]byte("/app/" + dst)); err != nil {
		return
	}
	w.Close()
	req, err := http.NewRequest("POST", cp_consoles_url, &b)
	if err != nil {
		fmt.Printf("Error 3 %v \n", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	return nil
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	Debug("Create", req.Name, d.path)
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
	d.mimemap[f.name] = "inode/x-empty"
	if d.path == "/" {
		d.CreateFile(req.Name)
		//} else if strings.HasSuffix(f.name, ".swp") {
		//	return f, f, nil
	} else {
		d.CreateFile(d.path + "/" + req.Name)
	}
	return f, f, nil
}

const maxInt = int(^uint(0) >> 1)

var _ = fs.HandleWriter(&File{})

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	f.writers = 1
	Debug("Write", f.name)
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
	Debug("Flush", f.name)
	Debug("Flush Writers", strconv.Itoa(int(f.writers)))

	if f.writers == 0 {
		// Read-only handles also get flushes. Make sure we don't
		// overwrite valid file contents with a nil buffer.
		Debug("Flush Read Only", "")
		return nil
	}

	Debug("Flush Write", "")
	cache_key := f.dir.fs.container + ":" + f.dir.path
	Debug("Invalidate", cache_key)
	cp_cache.Delete(cache_key)
	f.UploadFile()
	return nil
}

var _ = fs.HandleReleaser(&File{})

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	//Debug("Release", f.name, req.Flags)
	Debug("Release", f.name)
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
	Debug("Mkdir", req.Name, d.path)
	path := req.Name
	if d.path != "" {
		path = d.path + "/" + path
	}
	if d.path == "/" || d.path == "" {
		d.CreateDir(req.Name)
	} else {
		d.CreateDir(d.path + "/" + req.Name)
	}
	n := &Dir{
		fs:   d.fs,
		path: path,
	}
	return n, nil
}

var _ = fs.NodeRemover(&Dir{})

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {

	Debug("Remove", req.Name, strconv.FormatBool(req.Dir))
	/*switch req.Dir {
	case true:
		return fuse.ENOENT

	case false:
		d.RemoveFile(req.Name)
		return fuse.ENOENT
	}*/
	return nil
}

/*
func addCommands(repl *replizer.Repl) {
	access_token, err := GetTokenAccess()
	SaveTokenToFile(access_token)
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}
	addNewCommand(repl, "list logs", ListConsoles(access_token),
		"List audit logs.")
}

func addNewCommand(repl *replizer.Repl, instr string, startFn replizer.CommandStartFn, help string) {
	repl.AddCommand(&replizer.Command{
		Instruction: instr,
		StartFn:     startFn,
		Help:        help,
	})
}*/

func CmdListConsoles() error {

	access_token, err := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	SaveTokenToFile(access_token)
	if err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}

	//fmt.Printf("%#v\n", consoles[0].Title)
	if format == "json" {
		consoles := JsonListConsoles(access_token)
		//json_consoles, _ := json.MarshalIndent(consoles, "", "    ")
		fmt.Println(string(consoles))

	} else {

		consoles := ListConsoles(access_token)
		output := []string{
			"CONSOLE ID |TITLE|TYPE|CREATED|MOUNTED|URL",
		}
		for i := range consoles {
			var mounted string
			mountpoint := GetMountsFromFile(consoles[i].ContainerName)
			if mountpoint == "" {
				mounted = "NO"
			} else {
				mounted = "YES"
			}
			layout := "2006-01-02T15:04:05.000Z"
			t, _ := time.Parse(layout, consoles[i].CreatedAt)
			console_cols := consoles[i].ContainerName + "|" + consoles[i].Title + "|" + consoles[i].ContainerType + "|" + t.Format("2006-01-02 15:04:05") + "|" + mounted + "|" + site + "/consoles/" + consoles[i].Permalink
			output = append(output, console_cols)
		}
		result := columnize.SimpleFormat(output)
		istitle := true
		scanner := bufio.NewScanner(strings.NewReader(result))
		for scanner.Scan() {
			//first line is the title
			if istitle {
				fmt.Println(color(scanner.Text(), "table"))
			} else {
				line := strings.Replace(scanner.Text(), "NO", color("NO", "off"), 1)
				fmt.Println(color(line, "data"))
			}
			istitle = false
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
	}
	return nil
}

func CmdStopConsole(console string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	fmt.Printf(color("Stopping console %s ... ", "response"), console)
	StopConsole(access_token, console)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdConfigure() error {
	CreateConfigDir()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(color("Get your API Key from %s/dashboard/profile \n", "response"), site)
	fmt.Print(color("Client ID: ", "prompt"))
	input_id, _ := reader.ReadString('\n')
	reader_secret := bufio.NewReader(os.Stdin)
	fmt.Print(color("Client Secret: ", "prompt"))
	input_secret, _ := reader_secret.ReadString('\n')
	fmt.Print(color("Testing credentials... ", "response"))
	client_id := strings.Trim(input_id, "\n")
	client_secret := strings.Trim(input_secret, "\n")
	access_token, err := GetTokenAccessFromCredentials(client_id, client_secret)
	if err != nil {
		fmt.Println("Error: ", err)
		return nil
	}
	fmt.Printf(color("Done. Token: %s \n", "response"), access_token)
	fmt.Printf(color("Saving credentials... ", "response"))
	SaveCredentialsToFile(client_id, client_secret)
	SaveTokenToFile(access_token)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdClearScreen() error {
	ClearScreen()
	return nil
}
func CmdStartConsole(console string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	fmt.Printf(color("Starting console %s ... ", "response"), console)
	StartConsole(access_token, console)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdConnectConsole(console string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	if valid, _ := isValidConsole(access_token, console); valid {
		StartConsole(access_token, console)
		fmt.Printf(color("Connecting to  %s ... ", "response"), console)
		ConnectConsole(access_token, console)
	} else {
		fmt.Printf(color("This is not a valid console. Please try again \n", "response"))
	}
	return nil
}
func CmdRestartConsole(console string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	fmt.Printf(color("Restarting console %s ... ", "response"), console)
	RestartConsole(access_token, console)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}

func CmdMountConsole(args []string) error {

	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	StartConsole(access_token, args[0])
	mountpoint := GetMountsFromFile(args[0])
	if mountpoint == "" {
		var mount_point string
		if len(args) > 1 {
			mount_point = args[1]
		} else {
			mount_point = ""
		}
		Debug("MountPoint", mount_point)
		fmt.Printf("Mounting /app directory ... \n")
		fmt.Printf("TIP: If you want to mount in the background please add \"&\" at the end of the mount command. \n")
		MountConsole(access_token, args[0], mount_point)
	} else {

		fmt.Printf("Container %s is already mounted in %s \n", args[0], mountpoint)
		reader_unmount := bufio.NewReader(os.Stdin)
		input_unmount := "yes"
		fmt.Printf("Do you want to unmount and then mount to a different directory?[yes]")
		input, _ := reader_unmount.ReadString('\n')
		input_unmount = strings.TrimRight(input, "\r\n")
		if input_unmount == "yes" {
			CmdUnmountConsole(args[0])
			CmdMountConsole(args)
		}
	}
	/*if err != nil {
	    fmt.Println("Error: ", err)
	    panic(err)
	}*/
	return nil
}
func CmdUnmountConsole(console string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	UnmountConsole(access_token, console)
	return nil
}

func CmdUploadToConsole(console_id string, dst string, src string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	UploadFileToConsole(access_token, console_id, dst, src)
	return nil
}
func CmdDownloadFromConsole(console_id string, src string, dst string) error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	DownloadFileFromConsole(access_token, console_id, src, dst)
	return nil
}

func BgMountConsole(console_id string, mountbase string) {
	cp_bin, _ := osext.Executable()
	var mountpoint string
	fmt.Printf(color("Mounting /app directory from %s ... ", "response"), console_id)
	cmd := exec.Command("nohup", cp_bin, "mount", console_id, mountbase)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error %v", err)
	} else {
		if strings.HasPrefix(mountbase, "/") {
			mountpoint = mountbase + "/" + console_id
		} else {
			pwd, _ := os.Getwd()
			mountpoint = pwd + "/" + mountbase + "/" + console_id
		}
		fmt.Printf(color("Done * Mounted on %s \n", "response"), mountpoint)
	}
}
func CmdCreateConsole() error {
	access_token, _ := GetTokenAccess()
	if access_token == "" {
		fmt.Printf("It looks like you didn't authorize your credentials. \n")
		CmdConfigure()
		return nil
	}
	var console ConsoleExtra
	container_type := "bash"
	title := ""
	reader_type := bufio.NewReader(os.Stdin)
	fmt.Print(color("Type (bash, ruby, python, ... ) [ bash ]: ", "prompt"))
	input, _ := reader_type.ReadString('\n')
	container_type = strings.TrimRight(input, "\r\n")
	//reader_size := bufio.NewReader(os.Stdin)
	//fmt.Print("Size?(medium,large,xlarge,xxlarge)[medium]: ")
	//input, _ = reader_size.ReadString('\n')
	//container_size = strings.TrimRight(input, "\r\n")
	reader_title := bufio.NewReader(os.Stdin)
	fmt.Print(color("Title [ ]: ", "prompt"))
	input, _ = reader_title.ReadString('\n')
	title = strings.TrimRight(input, "\r\n")
	if container_type == "" {
		fmt.Println("type")
		container_type = "bash"
	}
	console.Size = "medium"
	console.Mode = "draft"
	console.Type = container_type
	console.Title = title
	fmt.Printf(color("Creating console ...", "response"))
	container_name, console_url := CreateConsole(access_token, console)
	fmt.Printf(color(" Done. * %s \n", "response"), container_name)
	fmt.Printf(color("%s \n", "response"), console_url)
	return nil
}

type Person struct {
	Id   int
	Name string
	Age  int
}

func (p *Person) Write(w io.Writer) {
	b, _ := json.Marshal(*p)
	w.Write(b)
}

func main() {
	app := cli.NewApp()
	cli.HelpPrinter = func(out io.Writer, templ string, data interface{}) {
		funcMap := template.FuncMap{
			"join": strings.Join,
		}
		var b bytes.Buffer
		outbuf := bufio.NewWriter(&b)
		w := tabwriter.NewWriter(outbuf, 1, 8, 2, ' ', 0)
		t := template.Must(template.New("help").Funcs(funcMap).Parse(templ))
		err := t.Execute(w, data)
		if err != nil {
			// If the writer is closed, t.Execute will fail, and there's nothing
			// we can do to recover.
			if os.Getenv("CLI_TEMPLATE_ERROR_DEBUG") != "" {
				fmt.Printf("CLI TEMPLATE ERROR: %#v\n", err)
			}
			return
		}

		w.Flush()
		outbuf.Flush()
		//fmt.Printf("%v \n", b.String())
		scanner := bufio.NewScanner(strings.NewReader(b.String()))
		for scanner.Scan() {
			//first line is the title
			line := scanner.Text()
			if strings.HasSuffix(line, ":") {
				fmt.Println(color(line, "prompt"))
			} else if strings.HasPrefix(line, "     ") || strings.HasPrefix(line, "   --") {
				words := strings.Fields(line)
				if len(words) > 0 {
					iscommand := true
					command := ""
					for i := range words {
						if iscommand == true {
							command = command + " " + words[i]
							if strings.HasSuffix(words[i], ",") {
								iscommand = true
							} else {
								iscommand = false
							}
						}
					}
					command_line := strings.Replace(line, command, color(command, "data"), 1)
					fmt.Println(command_line)
				}
			} else {
				/*			} else if strings.HasPrefix(line, "   --") {
							words := strings.Fields(line)
							if len(words) > 0 {
								iscommand := true
								command := ""
								for i := range words {
									if iscommand == true {
										command = command + " " + words[i]
										if strings.HasSuffix(words[i], ",") {
											iscommand = true
										} else {
											iscommand = false
										}
									}
								}
								command_line := strings.Replace(line, command, color(command, "data"), 1)
								fmt.Println(command_line)
							}
						} else {*/
				fmt.Println(color(line, "response"))

			}

			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading standard input:", err)
			}
		}
	}
	app.Version = version
	app.Name = "codepicnic"
	app.Usage = "A CLI tool to manage your CodePicnic consoles"
	var container_size, container_type, title, hostname, current_mode string

	app.Action = func(c *cli.Context) error {
		Repl(c)

		// Create the repl, add command state machines, and start the repl.
		/*repl := replizer.NewRepl()
		repl.Name = "CodePicnic"
		repl.FormatResponse = replizer.PrettyJson

		// Create a statemachine per command available in the repl.
		addCommands(repl)
		repl.Start()*/
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name: "clear",
			//Aliases: []string{"cls"},
			Usage: "clear screen",
			Action: func(c *cli.Context) error {
				CmdClearScreen()
				return nil
			},
		},
		{
			Name:  "configure",
			Usage: "save configuration",
			Action: func(c *cli.Context) error {
				CmdConfigure()
				return nil
			},
		},
		{
			Name:  "connect",
			Usage: "connect to a console",
			Action: func(c *cli.Context) error {
				CmdConnectConsole(c.Args()[0])
				//fmt.Println(ProxyConsole(access_token, c.Args()[0]))
				return nil
			},
		},
		{
			Name:  "copy",
			Usage: "copy a file from/to a console",
			Action: func(c *cli.Context) error {
				fmt.Println("")
				panic(nil)
				return nil
			},
		},
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
				if access_token == "" {
					fmt.Printf("It looks like you didn't authorize your credentials. \n")
					CmdConfigure()
					return nil
				}
				if err != nil {
					fmt.Println("Error: ", err)
				}
				var console ConsoleExtra

				if c.NumFlags() == 0 {

					reader_type := bufio.NewReader(os.Stdin)
					fmt.Print("Type?(bash,ruby,python ... )[bash]: ")
					input, _ := reader_type.ReadString('\n')
					container_type = strings.TrimRight(input, "\r\n")
					reader_title := bufio.NewReader(os.Stdin)
					fmt.Print("Title?[]: ")
					input, _ = reader_title.ReadString('\n')
					title = strings.TrimRight(input, "\r\n")
					if container_type == "" {
						fmt.Println("type")
						container_type = "bash"
					}

				}
				console.Size = container_size
				console.Type = container_type
				console.Title = title
				console.Hostname = hostname
				console.Mode = current_mode

				fmt.Printf("Creating console ...")
				container_name, console_url := CreateConsole(access_token, console)
				fmt.Printf("done. * %s \n", container_name)
				fmt.Printf("%s \n", console_url)
				return nil
			},
		},
		{
			Name:  "exit",
			Usage: "exit the REPL",
			Action: func(c *cli.Context) error {
				fmt.Println("Bye!")
				panic(nil)
				return nil
			},
		},
		{
			Name: "list",
			//Aliases: []string{"ls"},
			Usage: "list consoles",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "format",
					Value:       "text",
					Usage:       "Output format: text, json",
					Destination: &format,
				},
			},
			Action: func(c *cli.Context) error {
				CmdListConsoles()
				return nil
			},
		},
		{
			Name:  "mount",
			Usage: "mount /app filesystem from a container",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "debug",
					Usage:       "Debugging",
					Destination: &debug,
				},
			},
			Action: func(c *cli.Context) error {
				CmdMountConsole(c.Args())
				return nil
			},
		},
		{
			Name:  "restart",
			Usage: "restart a console",
			Action: func(c *cli.Context) error {
				CmdRestartConsole(c.Args()[0])
				return nil
			},
		},
		{
			Name:  "start",
			Usage: "start a console",
			Action: func(c *cli.Context) error {
				CmdStartConsole(c.Args()[0])
				return nil
			},
		},
		{
			Name:  "stop",
			Usage: "stop a console",
			Action: func(c *cli.Context) error {
				CmdStopConsole(c.Args()[0])
				return nil
			},
		},
		{
			Name:  "unmount",
			Usage: "unmount /app filesystem from a container",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:        "debug",
					Usage:       "Debugging",
					Destination: &debug,
				},
			},
			Action: func(c *cli.Context) error {
				//access_token, _ := GetTokenAccess()
				/*if access_token == "" {
					fmt.Printf("It looks like you didn't authorize your credentials. \n")
					CmdConfigure()
					return nil
				}*/
				CmdUnmountConsole(c.Args()[0])
				/*if err != nil {
					fmt.Println("Error: ", err)
					panic(err)
				}*/
				return nil
			},
		},
	}
	app.Run(os.Args)
}
