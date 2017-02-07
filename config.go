package main

import (
	"fmt"
	"github.com/go-ini/ini"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func GetLastVersion() (string, error) {
	var version_url = "http://deb.codepicnic.com/version"
	req, err := http.NewRequest("GET", version_url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", user_agent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf(err.Error())
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return strings.TrimRight(string(body), "\r\n"), nil
}
func IsLastVersion() bool {

	last_version, err := GetLastVersion()
	if err != nil {
		return true
	}
	float_last_version, err := strconv.ParseFloat(last_version, 64)
	float_version, err := strconv.ParseFloat(version, 64)
	if float_last_version > float_version {
		return false
	}
	return true
}

func CreateConfigDir() {
	config_file := config_dir + string(filepath.Separator) + cfg_file
	os.Mkdir(config_dir, 0755)
	if _, err := os.Stat(config_file); os.IsNotExist(err) {
		f, err := os.Create(config_file)
		if err != nil {
			fmt.Println(color(msg_rwperms, "error"))
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
		fmt.Println(color(msg_rwperms, "error"))
	}
	return

}
func RemoveMountFromFile(container string) {

	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		panic(err)
	}
	cfg.Section("mounts").DeleteKey(container)
	err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)

	if err != nil {
		fmt.Println(color(msg_rwperms, "error"))
	}
	return

}

func GetMountsFromFile(container string) string {
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		fmt.Println(color("Error.", "error"))
		fmt.Println(color(msg_rwperms, "error"))
	}
	mountpoint := cfg.Section("mounts").Key(container).String()
	return mountpoint
}
func GetAllMountsFromFile() []string {
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		fmt.Println(color("Error.", "error"))
		fmt.Println(color(msg_rwperms, "error"))
	}
	mountpoint := cfg.Section("mounts").KeyStrings()
	return mountpoint
}

func SaveTokenToFile(access_token string) {

	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		fmt.Println(color("Error.", "error"))
		fmt.Println(color(msg_rwperms, "error"))
	}
	cfg.Section("credentials").Key("access_token").SetValue(access_token)
	err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)

	if err != nil {
		fmt.Println(color("Error.", "error"))
		fmt.Println(color(msg_rwperms, "error"))
	}
	return

}

func IsFirstCheck() bool {
	day := time.Duration(24) * time.Hour
	cfg, err := ini.Load(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
	if err != nil {
		fmt.Println(color("Error.", "error"))
		fmt.Println(color(msg_rwperms, "error"))
	}
	last_update := cfg.Section("update").Key("last_update")
	if last_update.String() == "" {
		err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
		if err != nil {
			fmt.Println(color("Error.", "error"))
			fmt.Println(color(msg_rwperms, "error"))
		}
		cfg.Section("update").Key("last_update").SetValue(time.Now().Format(time.RFC3339))
		err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
		return true
	} else {
		last_update_time := last_update.MustTime()
		diff_update := time.Now().Sub(last_update_time)
		if diff_update > day {
			cfg.Section("update").Key("last_update").SetValue(time.Now().Format(time.RFC3339))
			err = cfg.SaveTo(getHomeDir() + "/" + cfg_dir + "/" + cfg_file)
			return true
		} else {
			return false
		}
		return false
	}

}
