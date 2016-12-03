package main

import (
	"bufio"
	"os/exec"
	"strings"
)

func GetOSVersion() string {
	os_version := "Linux"
	var dist, release string
	out, err := exec.Command("lsb_release", "-a").Output()
	if err != nil {
		os_version = "Linux"
	} else {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			str_line := scanner.Text()
			if strings.HasPrefix(str_line, "Distributor") {
				dist = strings.Replace(str_line, "Distributor ID:\t", "", -1)
				os_version = os_version + " " + dist
			} else if strings.HasPrefix(str_line, "Release") {
				release = strings.Replace(str_line, "Release:\t", "", -1)
				os_version = os_version + " " + release
			}
		}
	}
	return os_version

}
