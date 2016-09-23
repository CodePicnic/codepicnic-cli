package main

import (
	"bufio"
	"fmt"
	"github.com/kardianos/osext"
	"github.com/ryanuber/columnize"
	"os"
	"os/exec"
	"strings"
	"time"
)

func CmdGetTokenAccess() (string, error) {

	access_token, err := GetTokenAccess()
	if err != nil {
		if strings.Contains(err.Error(), ERROR_NOT_AUTHORIZED) {
			fmt.Println(color("It looks like you didn't authorize your credentials.", "error"))
			CmdConfigure()
			access_token = GetTokenAccessFromFile()
			return access_token, nil
		} else if strings.Contains(err.Error(), ERROR_NOT_CONNECTED) {
			fmt.Println(color("Can't connect to the CodePicnic API. Please try again", "error"))
		}
	}
	return access_token, nil

}

func CmdListConsoles() error {

	//access_token, err := GetTokenAccess()
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	//fmt.Printf("%#v\n", consoles[0].Title)
	if format == "json" {
		consoles := JsonListConsoles(access_token)
		//json_consoles, _ := json.MarshalIndent(consoles, "", "    ")
		fmt.Println(string(consoles))

	} else {

		consoles, err := ListConsoles(access_token)
		if err != nil {
			if strings.Contains(err.Error(), ERROR_INVALID_TOKEN) {
				access_token, err = CmdGetTokenAccess()
				consoles, err = ListConsoles(access_token)
			}
		}
		output := []string{
			"CONSOLE ID |TITLE|TYPE|CREATED|MOUNTED|URL",
		}
		for i := range consoles {
			var mounted string
			mountpoint := GetMountsFromFile(consoles[i].ContainerName)
			if mountpoint == "" {
				mounted = "NO"
			} else {
				mounted = "YES"
			}
			layout := "2006-01-02T15:04:05.000Z"
			t, _ := time.Parse(layout, consoles[i].CreatedAt)
			console_cols := consoles[i].ContainerName + "|" + consoles[i].Title + "|" + consoles[i].ContainerType + "|" + t.Format("2006-01-02 15:04:05") + "|" + mounted + "|" + site + "/consoles/" + consoles[i].Permalink
			output = append(output, console_cols)
		}
		result := columnize.SimpleFormat(output)
		istitle := true
		scanner := bufio.NewScanner(strings.NewReader(result))
		for scanner.Scan() {
			//first line is the title
			if istitle {
				fmt.Println(color(scanner.Text(), "table"))
			} else {
				line := strings.Replace(scanner.Text(), "NO", color("NO", "off"), 1)
				fmt.Println(color(line, "data"))
			}
			istitle = false
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "reading standard input:", err)
		}
	}
	return nil
}

func CmdStopConsole(console string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	fmt.Printf(color("Stopping console %s ... ", "response"), console)
	StopConsole(access_token, console)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdConfigure() error {
	CreateConfigDir()
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(color("Get your API Key from %s/dashboard/profile \n", "response"), site)
	fmt.Print(color("Client ID: ", "prompt"))
	input_id, _ := reader.ReadString('\n')
	reader_secret := bufio.NewReader(os.Stdin)
	fmt.Print(color("Client Secret: ", "prompt"))
	input_secret, _ := reader_secret.ReadString('\n')
	fmt.Print(color("Testing credentials... ", "response"))
	client_id := strings.Trim(input_id, "\n")
	client_secret := strings.Trim(input_secret, "\n")
	access_token, err := GetTokenAccessFromCredentials(client_id, client_secret)
	if err != nil {
		fmt.Println("Error: ", err)
		return nil
	}
	fmt.Printf(color("Done. Token: %s \n", "response"), access_token)
	fmt.Printf(color("Saving credentials... ", "response"))
	SaveCredentialsToFile(client_id, client_secret)
	SaveTokenToFile(access_token)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdClearScreen() error {
	ClearScreen()
	return nil
}
func CmdStartConsole(console string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	fmt.Printf(color("Starting console %s ... ", "response"), console)
	StartConsole(access_token, console)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdConnectConsole(console string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	if valid, _ := isValidConsole(access_token, console); valid {
		StartConsole(access_token, console)
		fmt.Printf(color("Connecting to  %s ... ", "response"), console)
		ConnectConsole(access_token, console)
	} else {
		fmt.Printf(color("This is not a valid console. Please try again \n", "response"))
	}
	return nil
}
func CmdRestartConsole(console string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	fmt.Printf(color("Restarting console %s ... ", "response"), console)
	RestartConsole(access_token, console)
	fmt.Printf(color("Done.\n", "response"))
	return nil
}

func CmdMountConsole(args []string) error {

	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	StartConsole(access_token, args[0])
	mountpoint := GetMountsFromFile(args[0])
	if mountpoint == "" {
		var mount_point string
		if len(args) > 1 {
			mount_point = args[1]
		} else {
			mount_point = ""
		}
		Debug("MountPoint", mount_point)
		fmt.Printf("Mounting /app directory ... \n")
		fmt.Printf("TIP: If you want to mount in the background please add \"&\" at the end of the mount command. \n")
		MountConsole(access_token, args[0], mount_point)
	} else {

		fmt.Printf("Container %s is already mounted in %s \n", args[0], mountpoint)
		reader_unmount := bufio.NewReader(os.Stdin)
		input_unmount := "yes"
		fmt.Printf("Do you want to unmount and then mount to a different directory?[yes]")
		input, _ := reader_unmount.ReadString('\n')
		input_unmount = strings.TrimRight(input, "\r\n")
		if input_unmount == "yes" {
			CmdUnmountConsole(args[0])
			CmdMountConsole(args)
		}
	}
	/*if err != nil {
	    fmt.Println("Error: ", err)
	    panic(err)
	}*/
	return nil
}
func CmdUnmountConsole(console string) error {
	UnmountConsole(console)
	return nil
}

func CmdUploadToConsole(console_id string, dst string, src string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	UploadFileToConsole(access_token, console_id, dst, src)
	return nil
}
func CmdDownloadFromConsole(console_id string, src string, dst string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	DownloadFileFromConsole(access_token, console_id, src, dst)
	return nil
}

func CmdExecConsole(console_id string, command string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	results, _ := ExecConsole(access_token, console_id, command)
	for i := range results {
		fmt.Printf(color(results[i].result, "response"))
	}
	return nil
}

func BgMountConsole(console_id string, mountbase string) {
	cp_bin, _ := osext.Executable()
	var mountpoint string
	fmt.Printf(color("Mounting /app directory from %s ... ", "response"), console_id)
	cmd := exec.Command("nohup", cp_bin, "mount", console_id, mountbase)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error %v", err)
	} else {
		if strings.HasPrefix(mountbase, "/") {
			mountpoint = mountbase + "/" + console_id
		} else {
			pwd, _ := os.Getwd()
			mountpoint = pwd + "/" + mountbase + "/" + console_id
		}
		fmt.Printf(color("Done * Mounted on %s \n", "response"), mountpoint)
	}
}
func CmdCreateConsole() error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	var console ConsoleExtra
	container_type := "bash"
	title := ""
	reader_type := bufio.NewReader(os.Stdin)
	fmt.Print(color("Type (bash, ruby, python, ... ) [ bash ]: ", "prompt"))
	input, _ := reader_type.ReadString('\n')
	container_type = strings.TrimRight(input, "\r\n")
	//reader_size := bufio.NewReader(os.Stdin)
	//fmt.Print("Size?(medium,large,xlarge,xxlarge)[medium]: ")
	//input, _ = reader_size.ReadString('\n')
	//container_size = strings.TrimRight(input, "\r\n")
	reader_title := bufio.NewReader(os.Stdin)
	fmt.Print(color("Title [ ]: ", "prompt"))
	input, _ = reader_title.ReadString('\n')
	title = strings.TrimRight(input, "\r\n")
	if container_type == "" {
		fmt.Println("type")
		container_type = "bash"
	}
	console.Size = "medium"
	console.Mode = "draft"
	console.Type = container_type
	console.Title = title
	fmt.Printf(color("Creating console ...", "response"))
	container_name, console_url := CreateConsole(access_token, console)
	fmt.Printf(color(" Done. * %s \n", "response"), container_name)
	fmt.Printf(color("%s \n", "response"), console_url)
	return nil
}
