package main

import (

    "fmt"
    "os/exec"
"bufio"
"strings"

)

func GetOSVersion() string {
    os_version := "Linux Ubuntu 14.04"
    out, err := exec.Command("lsb_release",  "-a").Output()
	fmt.Println(os_version)
    if err != nil {
    os_version = "Linux Ubuntu 14.04"
	fmt.Println(err)
	
    }

scanner := bufio.NewScanner(strings.NewReader(string(out)))

for scanner.Scan() {

    fmt.Println(scanner.Text())

}
    return os_version

}
