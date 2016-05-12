package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/go-ini/ini"
	"github.com/ryanuber/columnize"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"strconv"
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
	Consoles []ConsoleExtra `json:"consoles"`
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

func getHomeDir() string {

	user_data, err := user.Current()
	if err != nil {
		panic(err)
	}
	return user_data.HomeDir

}

func GetTokenAccess() string {

	cp_token_url := "https://codepicnic.com/oauth/token"
	client_id, client_secret := GetCredentialsFromFile()
	cp_payload := `{ "grant_type": "client_credentials","client_id": "` + client_id + `", "client_secret": "` + client_secret + `"}`
	var jsonStr = []byte(cp_payload)
	req, err := http.NewRequest("POST", cp_token_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	//fmt.Println("response Status:", resp.Status)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var token Token
	_ = json.NewDecoder(resp.Body).Decode(&token)
	return token.Access
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

func CreateConsole(access_token string) string {

	cp_consoles_url := "https://codepicnic.com/api/consoles"
	var jsonStr = []byte(` { "console": { "container_size": "medium", "container_type": "bash" }  }`)
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
func ListConsoles(access_token string) []ConsoleExtra {

	cp_consoles_url := "https://codepicnic.com/api/consoles.json"
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
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

func main() {
	app := cli.NewApp()
	app.Name = "codepicnic"
	app.Usage = "codepicnic-cli is a tool to manage your CodePicnic consoles"

	app.Commands = []cli.Command{
		{
			Name: "create",
			//Aliases: []string{"c"},
			Usage: "create and start a new console",
			Action: func(c *cli.Context) error {
				access_token := GetTokenAccess()
				container_name := CreateConsole(access_token)
				fmt.Println(container_name)
				return nil
			},
		},
		{
			Name:    "list",
			Aliases: []string{"ls"},
			Usage:   "list consoles",
			Action: func(c *cli.Context) error {
				access_token := GetTokenAccess()
				consoles := ListConsoles(access_token)
				//fmt.Printf("%#v\n", consoles[0].Title)
				output := []string{
					"ID | TITLE | NAME | CONTAINER NAME | CONTAINER TYPE | CREATED | PERMALINK",
				}
				for i := range consoles {
					console_cols := strconv.Itoa(consoles[i].Id) + "|" + consoles[i].Title + "|" + consoles[i].Name + "|" + consoles[i].ContainerName + "|" + consoles[i].ContainerType + "|" + consoles[i].CreatedAt + "|" + "http://codepicnic.com/consoles/" + consoles[i].Permalink
					output = append(output, console_cols)
				}
				result := columnize.SimpleFormat(output)
				fmt.Println(result)
				return nil
			},
		},
	}

	app.Run(os.Args)
}
