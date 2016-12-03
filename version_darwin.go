package main

import (
	"bufio"
	"os/exec"
	"strings"
)

func GetOSVersion() string {

	os_version := "macOS"
	var dist string
	out, err := exec.Command("sw_vers").Output()
	if err != nil {
		os_version = "macOs"
	} else {
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			str_line := scanner.Text()
			if strings.HasPrefix(str_line, "ProductVersion") {
				dist = strings.Replace(str_line, "ProductVersion:\t", "", -1)
				os_version = os_version + " " + dist
			}
		}
	}
	return os_version
}
