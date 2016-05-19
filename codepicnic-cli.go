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
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	//"time"
)

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
}

type ConsoleCollection struct {
	Consoles []ConsoleJson `json:"consoles"`
}

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

	cp_token_url := "https://codepicnic.com/oauth/token"
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

	cp_consoles_url := "https://codepicnic.com/api/consoles"

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

//func ListConsoles(access_token string) []Console {
func ListConsoles(access_token string) []ConsoleJson {

	//cp_consoles_url := "https://codepicnic.com/api/consoles.json"
	cp_consoles_url := "https://codepicnic.com/api/consoles/all"
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	//fmt.Println(time.Now().Format("20060102150405"))
	resp, err := client.Do(req)
	//fmt.Println(time.Now().Format("20060102150405"))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	//fmt.Println("response Status:", resp.Status)
	//fmt.Printf("%+v\n", resp)
	var console_collection ConsoleCollection
	body, err := ioutil.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &console_collection)
	//_ = json.NewDecoder(resp.Body).Decode(&console_collection)
	//fmt.Printf("%+v\n", string(body))
	//fmt.Printf("%#v\n", console_collection.Consoles[0].Title)
	return console_collection.Consoles
}

func StopConsole(access_token string, container_name string) {

	cp_consoles_url := "https://codepicnic.com/api/consoles/" + container_name + "/stop"
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

	cp_consoles_url := "https://codepicnic.com/api/consoles/" + container_name + "/start"
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

	cp_consoles_url := "https://codepicnic.com/api/consoles/" + container_name + "/restart"
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

func ConnectConsole(access_token string, container_name string) {

	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient("tcp://52.200.53.168:4000", "v1.22", nil, defaultHeaders)
	if err != nil {
		panic(err)
	}
	//r, err := cli.ContainerInspect(context.Background(), container_name)
	r, err := cli.ContainerExecCreate(context.Background(), container_name, types.ExecConfig{User: "", Cmd: []string{"bash"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	fmt.Println(r.ID)

	/**
	err = cli.ContainerExecStart(context.Background(), r.ID, types.ExecStartCheck{
		Detach: true,
		Tty:    false,
	})
	_, err = cli.ContainerExecAttach(context.Background(), r.ID, types.ExecConfig{User: "", Cmd: []string{"ls"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}*/
	aResp, err := cli.ContainerExecAttach(context.Background(), r.ID, types.ExecConfig{User: "", Cmd: []string{"ls"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})

	if err != nil {
		fmt.Println(err)
		panic(err)
	}
	/*
		aResp, err := cli.ContainerAttach(context.Background(), container_name, types.ContainerAttachOptions{
			Stream: true,
			Stdin:  false,
			Stdout: true,
			Stderr: true,
		})*/
	tty := true
	if err != nil {
		log.Fatalf("Couldn't attach to container: %s", err)
	}
	/*
		//go func() {
		if tty {
			io.Copy(os.Stdout, aResp.Reader)
		} else {
			stdcopy.StdCopy(os.Stdout, os.Stderr, aResp.Reader)
		}
		//}()*/

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

	//_, err = cli.ContainerWait(context.Background(), container_name)
	//if err != nil {
	//	log.Fatalf("Error waiting for container to finished: %s", err)
	//}
	return
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
					console_cols := strconv.Itoa(consoles[i].Id) + "|" + consoles[i].Title + "|" + consoles[i].ContainerName + "|" + consoles[i].ContainerType + "|" + consoles[i].CreatedAt + "|" + "http://codepicnic.com/consoles/" + consoles[i].Permalink
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
				ConnectConsole(access_token, c.Args()[0])
				return nil
			},
		},
	}
	app.Run(os.Args)
}
