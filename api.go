package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
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
	//Url           string `json:"url"`
	//TerminalUrl   string `json:"terminal_url"`
}

type ConsoleCollection struct {
	Consoles []ConsoleJson `json:"consoles"`
}

func GetTokenAccess() (string, error) {
	client_id, client_secret := GetCredentialsFromFile()
	if client_id == "" || client_secret == "" {
		return "", nil
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

func isValidConsole(token string, console string) (bool, error) {
	consoles := ListConsoles(token)
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
