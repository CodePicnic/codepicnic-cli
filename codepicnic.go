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
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"text/template"
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

func CreateConsole(access_token string, console_extra ConsoleExtra) (string, string) {

	cp_consoles_url := site + "/api/consoles"

	//cp_payload := `{ "console:    { "grant_type": "client_credentials","client_id": "` + client_id + `", "client_secret": "` + client_secret + `"}`
	cp_payload := ` { "console": { "container_size": "` + console_extra.Size + `", "container_type": "` + console_extra.Type + `", "title": "` + console_extra.Title + `" , "hostname": "` + console_extra.Hostname + `", "current_mode": "` + console_extra.Mode + `" }  }`
	var jsonStr = []byte(cp_payload)
	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	//req.Header.Set("Content-Type", "application/json")
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
			//fmt.Printf("stdinDone\n")
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
	//fmt.Printf("done\n")
	aResp.Conn.Close()
	return
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
		//Start the REPL if not argument given
		Repl(c)
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
				if err != nil {
					return nil
				}
				if access_token == "" {
					fmt.Printf("It looks like you didn't authorize your credentials. \n")
					CmdConfigure()
					return nil
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
