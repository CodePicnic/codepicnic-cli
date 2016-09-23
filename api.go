package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/go-ini/ini"
	"io/ioutil"
	"net/http"
	"strings"
)

const ERROR_NOT_AUTHORIZED = "Not Authorized"
const ERROR_NOT_CONNECTED = "Disconnected"
const ERROR_EMPTY_CREDENTIALS = "No Credentials"
const ERROR_EMPTY_TOKEN = "No Token"
const ERROR_INVALID_TOKEN = "Invalid Token"

const TOKEN_LEN = 64

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

func GetTokenAccess() (string, error) {
	client_id, client_secret := GetCredentialsFromFile()
	if client_id == "" || client_secret == "" {
		return "", errors.New(ERROR_EMPTY_CREDENTIALS)
	}
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
	if err != nil {
		if strings.Contains(err.Error(), "read: connection refused") {
			fmt.Println(color("Can't connect to the CodePicnic API. Please verify your connection or try again.", "error"))
			return "", errors.New(ERROR_NOT_CONNECTED)
		}
	}
	//fmt.Println("response Status:", resp.Status)
	//fmt.Println("response Status:", resp.StatusCode)
	if resp.StatusCode == 401 {
		return "", errors.New(ERROR_NOT_AUTHORIZED)
	}
	defer resp.Body.Close()
	var token Token
	_ = json.NewDecoder(resp.Body).Decode(&token)
	SaveTokenToFile(token.Access)
	return token.Access, nil
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

func GetTokenAccessFromFile() (token string) {
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		return
	}
	token = cfg.Section("credentials").Key("access_token").String()
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

func ListConsoles(access_token string) ([]ConsoleJson, error) {

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
	if resp.StatusCode == 401 {
		return nil, errors.New(ERROR_INVALID_TOKEN)
	}
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
	return console_collection.Consoles, nil
}

func isValidConsole(token string, console string) (bool, error) {
	consoles, _ := ListConsoles(token)
	for i := range consoles {
		if console == consoles[i].ContainerName {
			return true, nil
		}
	}
	return false, nil
}

func JsonListConsoles(access_token string) string {

	cp_consoles_url := site + "/api/consoles/all"
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
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
	//fmt.Printf("%+v\n", string(body))
	return string(body)
}

type JsonCommand struct {
	command string
	result  string
}

func ExecConsole(token string, console string, command string) ([]JsonCommand, error) {
	cp_consoles_url := site + "/api/consoles/" + console + "/exec"
	cp_payload := ` { "commands": "` + command + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var CmdCollection []JsonCommand

	jsonBody, err := gabs.ParseJSON(body)
	jsonPaths, _ := jsonBody.ChildrenMap()
	for key, child := range jsonPaths {
		var cmd JsonCommand
		cmd.command = string(key)
		cmd.result = child.Data().(string)
		//Debug("key, value, type", key, child.Data().(string), jsonTypes[jsonFile.path])
		//Debug("key, value, type", key, child.Data().(string), jsonFile.mime)
		CmdCollection = append(CmdCollection, cmd)

	}

	//fmt.Printf("%+v\n", string(body))
	return CmdCollection, nil
}
