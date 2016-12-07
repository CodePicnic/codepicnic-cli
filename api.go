package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/go-ini/ini"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
)

const ERROR_NOT_AUTHORIZED = "Not Authorized"
const ERROR_NOT_CONNECTED = "Disconnected"
const ERROR_EMPTY_CREDENTIALS = "No Credentials"
const ERROR_EMPTY_TOKEN = "No Token"
const ERROR_INVALID_TOKEN = "Invalid Token"
const ERROR_USAGE_EXCEEDED = "Usage Exceeded"

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
type StackCollection struct {
	Stacks []StackJson `json:"container_types"`
}

type StackJson struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name"`
	ShortName  string `json:"short_name"`
	Version    string `json:"version"`
	ImageName  string `json:"image_name"`
	Group      string `json:"group"`
}

//Another type of console :(
type ConsoleShowJson struct {
	Url           string `json:"url"`
	EmbedUrl      string `json:"embed_url"`
	ContainerName string `json:"container_name"`
	TerminalUrl   string `json:"terminal_url"`
	IsHeadless    string `json:"is_headless"`
	Title         string `json:"title"`
	Permalink     string `json:"permalink"`
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
	req.Header.Set("User-Agent", user_agent)
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

	cp_consoles_url := site + "/api/consoles/all.json"
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	//fmt.Println(time.Now().Format("20060102150405"))
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return nil, errors.New(ERROR_USAGE_EXCEEDED)
	}
	//fmt.Println("response Status:", resp.Status)
	//fmt.Println("response Status:", resp.StatusCode)
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

func isValidConsole(token string, console string) (bool, ConsoleJson, error) {
	var console_empty ConsoleJson
	consoles, err := ListConsoles(token)
	if err != nil {
		return false, console_empty, err
	}

	for i := range consoles {
		if console == consoles[i].ContainerName {
			return true, consoles[i], nil
		}
	}
	return false, console_empty, nil
}

func JsonListConsoles(access_token string) string {

	cp_consoles_url := site + "/api/consoles/all"
	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	req.Header.Set("User-Agent", user_agent)
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
	var CmdCollection []JsonCommand
	cp_consoles_url := site + "/api/consoles/" + console + "/exec"
	cp_payload := ` { "commands": "` + command + `" }`
	var jsonStr = []byte(cp_payload)

	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if resp.StatusCode == 401 {
		return CmdCollection, errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return CmdCollection, errors.New(ERROR_USAGE_EXCEEDED)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

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

func StartConsole(access_token string, container_name string) error {

	cp_consoles_url := site + "/api/consoles/" + container_name + "/start"
	req, err := http.NewRequest("POST", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return errors.New(ERROR_USAGE_EXCEEDED)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return nil
}

func StopConsole(access_token string, container_name string) error {

	cp_consoles_url := site + "/api/consoles/" + container_name + "/stop"
	req, err := http.NewRequest("POST", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return errors.New(ERROR_USAGE_EXCEEDED)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return nil
}

func RestartConsole(access_token string, container_name string) error {

	cp_consoles_url := site + "/api/consoles/" + container_name + "/restart"
	req, err := http.NewRequest("POST", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode == 401 {
		return errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return errors.New(ERROR_USAGE_EXCEEDED)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return nil
}

func RemoveConsole(access_token string, console string) error {
	cp_consoles_url := site + "/api/consoles" + "/" + console
	var jsonStr = []byte("")
	req, err := http.NewRequest("DELETE", cp_consoles_url, bytes.NewBuffer(jsonStr))
	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if resp.StatusCode == 401 {
		return errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return errors.New(ERROR_USAGE_EXCEEDED)
	}
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	return nil
}
func ListStacks(access_token string) ([]StackJson, error) {

	cp_types_url := site + "/api/container_types.json"
	req, err := http.NewRequest("GET", cp_types_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", user_agent)
	req.Header.Set("Authorization", "Bearer "+access_token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return nil, errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return nil, errors.New(ERROR_USAGE_EXCEEDED)
	}
	var stack_collection StackCollection
	body, err := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &stack_collection)
	if err != nil {
		fmt.Printf("%+v\n", err)
	}
	//_ = json.NewDecoder(resp.Body).Decode(&console_collection)
	//fmt.Printf("%+v\n", string(body))
	//fmt.Printf("%#v\n", console_collection.Consoles[0].Title)
	return stack_collection.Stacks, nil
}

func CreateConsole(access_token string, console_extra ConsoleExtra) (string, string, error) {

	cp_consoles_url := site + "/api/consoles"

	//cp_payload := `{ "console:    { "grant_type": "client_credentials","client_id": "` + client_id + `", "client_secret": "` + client_secret + `"}`
	cp_payload := ` { "console": { "container_size": "` + console_extra.Size + `", "container_type": "` + console_extra.Type + `", "title": "` + console_extra.Title + `" , "hostname": "` + console_extra.Hostname + `", "current_mode": "` + console_extra.Mode + `" }  }`
	var jsonStr = []byte(cp_payload)
	req, err := http.NewRequest("POST", cp_consoles_url, bytes.NewBuffer(jsonStr))
	//req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+access_token)
	//req.Header.Set("User-Agent", "CodePicnic-CLI/"+version+ " ("+GetOSVersion()+")")
	req.Header.Set("User-Agent", user_agent)
	client := &http.Client{}
	resp, err := client.Do(req)
	if resp.StatusCode == 401 {
		return "", "", errors.New(ERROR_INVALID_TOKEN)
	} else if resp.StatusCode == 429 {
		return "", "", errors.New(ERROR_USAGE_EXCEEDED)
	}
	//fmt.Println("response Status:", resp.Status)
	//fmt.Println("response Status:", resp.StatusCode)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	var console Console
	_ = json.NewDecoder(resp.Body).Decode(&console)
	return console.ContainerName, console.Url, nil
}

func DownloadFileFromConsole(token string, console_id string, src string, dst string) error {
	if dst == "" {
		dst = src
	}
	cp_consoles_url := site + "/api/consoles/" + console_id + "/" + src

	req, err := http.NewRequest("GET", cp_consoles_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", user_agent)

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
	req.Header.Set("User-Agent", user_agent)

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
