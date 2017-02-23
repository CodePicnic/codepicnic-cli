package main

import (
	"bufio"
	"fmt"
	"github.com/kardianos/osext"
	"github.com/ryanuber/columnize"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
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
			} else if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
				fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
				return nil
			}
		}
		if len(consoles) == 0 {
			fmt.Printf(color("Aww, you haven't created any consoles yet. Try 'codepicnic create'.\n", "error"))
			return nil
		}
		output := []string{
			"CONSOLE ID |TITLE|TYPE|CREATED|MOUNTED|URL",
		}
		for i := range consoles {
			var mounted string
			var mountpoint string
			if consoles[i].ContainerName == "" {
			} else {
				mountpoint = GetMountsFromFile(consoles[i].ContainerName)
				if mountpoint == "" {
					mounted = "NO"
				} else {
					mounted = "YES"
				}
				layout := "2006-01-02T15:04:05.000Z"
				t, _ := time.Parse(layout, consoles[i].CreatedAt)
				console_title := consoles[i].Title
				if len(console_title) > 23 {
					console_title = console_title[:20] + "..."
				}
				console_cols := consoles[i].ContainerName + "|" + console_title + "|" + consoles[i].ContainerType + "|" + t.Format("2006-01-02 15:04:05") + "|" + mounted + "|" + consoles_short_url + consoles[i].Permalink
				output = append(output, console_cols)
			}
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
	err := StopConsole(access_token, console)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
			fmt.Printf(color("Error.\n", "error"))
			fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			return nil
		}
	}
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
	fmt.Printf(color(msg_bugs+"\n", "response"))
	return nil
}

func CmdConfigureCredentials(client_id string, client_secret string) error {
	CreateConfigDir()
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
	fmt.Printf(color(msg_bugs+"\n", "response"))
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
	err := StartConsole(access_token, console)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
			fmt.Printf(color("Error.\n", "error"))
			fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			return nil
		}
	}
	fmt.Printf(color("Done.\n", "response"))
	return nil
}
func CmdConnectConsole(console string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	if valid, _, err := isValidConsole(access_token, console); valid {
		fmt.Printf(color("Connecting to  %s ... ", "response"), console)
		StartConsole(access_token, console)
		ProxyConsole(access_token, console)
		//ConnectConsole(access_token, console)
	} else {
		if err != nil {
			if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
				fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
				return nil
			}
		}
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
	err := RestartConsole(access_token, console)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
			fmt.Printf(color("Error.\n", "error"))
			fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			return nil
		}
	}
	fmt.Printf(color("Done.\n", "response"))
	return nil
}

func CmdRemoveConsole(console string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	reader_remove := bufio.NewReader(os.Stdin)
	input_remove := "yes"
	fmt.Printf(color("Are you sure you want to remove the console? [yes]: ", "prompt"))
	input, _ := reader_remove.ReadString('\n')
	input_remove = strings.TrimRight(input, "\r\n")
	if input_remove == "yes" {
		fmt.Printf(color("Removing console %s ... ", "response"), console)
		err := RemoveConsole(access_token, console)
		if err != nil {
			if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
				fmt.Printf(color("Error.\n", "error"))
				fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
				return nil
			}
		}
		fmt.Printf(color("Done.\n", "response"))
	} else {
		fmt.Printf(color("Removing console %s ... \n", "response"), console)
	}
	return nil
}

func CmdMountConsole(args []string) error {

	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	err := StartConsole(access_token, args[0])
	if err != nil {
		if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
			fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			return nil
		}
	}
	mountpoint := GetMountsFromFile(args[0])
	if mountpoint == "" {
		var mount_point string
		if len(args) > 1 {
			mount_point = args[1]
		} else {
			mount_point = ""
		}
		fmt.Printf("Mounting /app directory ... \n")
		fmt.Printf("TIP: If you want to mount in the background please add \"&\" at the end of the mount command. \n")
		err = MountConsole(access_token, args[0], mount_point)
		if err != nil {
			fmt.Printf(color("There was an error mounting your console. Please  verify if the console exists and try again.\n", "error"))
		}
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
func CmdInspectConsole(console_id string) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	console, _ := GetConsoleInfo(access_token, console_id)

	if (ConsoleJson{}) == console {
		fmt.Printf(color("This is not a valid console. Please try again \n", "error"))
	} else {
		fmt.Printf(color("Console Id: %s \n", "response"), console.ContainerName)
		if console.Title != "" {
			fmt.Printf(color("Console Title: %s \n", "response"), console.Title)
		}
		fmt.Printf(color("Console Type: %s \n", "response"), console.ContainerType)
		fmt.Printf(color("Console URL: %s \n", "response"), consoles_short_url+console.Permalink)
		fmt.Printf(color("External URL: %s \n", "response"), "https://"+console.ContainerName+"-"+console.ContainerType+".codepicnic.com")
	}

	return nil
}

func CmdUnmountAllConsoles() error {
	mounted_consoles := GetAllMountsFromFile()
	for _, console := range mounted_consoles {
		if GetMountsFromFile(console) == "" {
		} else {
			UnmountConsole(console)
		}
	}
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
	results, err := ExecConsole(access_token, console_id, command)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
			fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			return nil
		}
	}
	for i := range results {
		fmt.Printf(color(results[i].result, "response"))
	}
	return nil
}

func BgMountConsole(console_id string, mountbase string) {
	access_token := GetTokenAccessFromFile()
	cp_bin, _ := osext.Executable()
	var mountpoint string
	var mountlink string
	if valid, console, err := isValidConsole(access_token, console_id); valid {
		if len(console.Title) > 0 {
			mountlink = console.Permalink
		} else {
			mountlink = console_id
		}
		if strings.HasPrefix(mountbase, "/") {
			mountpoint = mountbase + "/" + mountlink
		} else if mountbase == "" {
			pwd, _ := os.Getwd()
			mountpoint = pwd + "/" + mountlink

		} else {
			pwd, _ := os.Getwd()
			mountpoint = pwd + "/" + mountbase + "/" + mountlink
		}
		if _, err := os.Stat(mountpoint); err == nil {
			fmt.Printf(color("Mount point %s already exists, please remove it or try to mount in a different directory \n", "response"), mountpoint)
		} else {
			fmt.Printf(color("Mounting /app directory from %s ... ", "response"), console_id)
			cmd := exec.Command("nohup", cp_bin, "bgmount", console_id, mountbase)
			err := cmd.Start()
			if err != nil {
				fmt.Printf("Error %v", err)
			} else {
				fmt.Printf(color("Done * Mounted on %s \n", "response"), mountpoint)
				NotifyDesktop()
			}
		}
	} else {
		if err != nil {
			if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
				fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			}
		} else {
			fmt.Printf(color("This is not a valid console. Please try again \n", "response"))
		}
	}

}

/*
func CmdCreateConsole() error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	var console ConsoleExtra
	fmt.Printf(color("Creating console ...", "response"))
	container_name, console_url := CreateConsole(access_token, console)
	fmt.Printf(color(" Done. * %s \n", "response"), container_name)
	fmt.Printf(color("%s \n", "response"), console_url)
	return nil
}*/

func CmdCreateConsole(console ConsoleExtra) error {
	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}

	if (ConsoleExtra{}) == console {
		container_type := "bash"
		title := ""
		reader_type := bufio.NewReader(os.Stdin)
		fmt.Print(color("Type (bash, ruby, python, ... ) [ bash ]: ", "prompt"))
		input, _ := reader_type.ReadString('\n')
		container_type = strings.TrimRight(input, "\r\n")
		reader_title := bufio.NewReader(os.Stdin)
		fmt.Print(color("Title [ ]: ", "prompt"))
		input, _ = reader_title.ReadString('\n')
		title = strings.TrimRight(input, "\r\n")
		if container_type == "" {
			container_type = "bash"
		}
		console.Size = "medium"
		console.Mode = "draft"
		console.Type = container_type
		console.Title = title

	} else {
		if console.Type == "" {
			console.Type = "bash"
		}
		if console.Size == "" {
			console.Type = "medium"
		}
		if console.Mode == "" {
			console.Type = "draft"
		}
	}
	fmt.Printf(color("Creating console ...", "response"))
	container_name, console_url, err := CreateConsole(access_token, console)
	if err != nil {
		if strings.Contains(err.Error(), ERROR_USAGE_EXCEEDED) {
			fmt.Printf(color(" Error.\n", "error"))
			fmt.Printf(color("You have exceeded your monthly usage. Upgrade your account at https://codepicnic.com/dashboard/funds.\n", "error"))
			return nil
		}
	}
	if container_name == "" {
		fmt.Printf(color(" Error.\n", "error"))
		fmt.Printf(color("There was an error creating the console. Please verify if you choose the right type.\n", "error"))
	} else {
		fmt.Printf(color(" Done! \n", "response"))
		fmt.Printf(color("Console ID:  %s \n", "response"), container_name)
		fmt.Printf(color("Console URL: %s \n", "response"), console_url)
		stack, _ := GetStackInfo(access_token, console.Type)

		if (StackJson{}) == stack {
		} else {
			if stack.Group == "framework" {
				fmt.Printf(color("%s URL: %s \n", "response"), stack.ShortName, "https://"+container_name+"-"+stack.Identifier+".codepicnic.com")
			}

		}
	}
	return nil
}

func CmdValidateCredentials() error {
	token, err := GetTokenAccess()
	if err != nil {
		if strings.Contains(err.Error(), "Disconnected") {
			fmt.Println(color("Can't connect to the CodePicnic API. Please verify your connection or try again.", "error"))
		} else if strings.Contains(err.Error(), "Not Authorized") {
			fmt.Println(color("It looks like you didn't authorize your credentials.", "error"))
			CmdConfigure()
		} else if strings.Contains(err.Error(), ERROR_EMPTY_CREDENTIALS) {
			fmt.Println(color("It looks like you didn't authorize your credentials.", "error"))
			CmdConfigure()
		} else {
			fmt.Println(color(err.Error(), "error"))
		}
	} else if token == "" {
		fmt.Println(color("It looks like you didn't authorize your credentials.", "error"))
		CmdConfigure()
	}
	return nil
}
func CmdListStacks() error {

	access_token := GetTokenAccessFromFile()
	if len(access_token) != TOKEN_LEN {
		access_token, _ = CmdGetTokenAccess()
	}
	if format == "json" {
		//consoles := JsonListConsoles(access_token)
		//fmt.Println(string(consoles))

	} else {

		stacks, err := ListStacks(access_token)
		if err != nil {
			if strings.Contains(err.Error(), ERROR_INVALID_TOKEN) {
				access_token, err = CmdGetTokenAccess()
				stacks, err = ListStacks(access_token)
			}
		}
		output := []string{
			"IDENTIFIER|NAME|GROUP",
		}
		for i := range stacks {
			if strings.HasPrefix(stacks[i].Identifier, "devpad") {
			} else {
				stack_cols := stacks[i].Identifier + "|" + stacks[i].Name + "|" + stacks[i].Group
				output = append(output, stack_cols)
			}
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

func CmdUpdate() error {
	if IsLastVersion() == false {
		//input_update := "yes"
		//fmt.Printf(color("Update CodePicnic CLI? [yes]: ", "prompt"))
		//reader_update := bufio.NewReader(os.Stdin)
		//input, _ := reader_update.ReadString('\n')
		//input_update = strings.TrimRight(input, "\r\n")
		input_update := "yes"
		if input_update == "yes" {
			file_tmp := "/tmp/codepicnic-" + version
			cp_bin, _ := osext.Executable()

			file, err := os.Open(cp_bin)
			if err != nil {
				return err
			}
			fi, err := file.Stat()
			if err != nil {
				return err
			}
			fi_sys := fi.Sys().(*syscall.Stat_t)

			// Create the file
			out, err := os.Create(file_tmp)

			if err != nil {
				return err
			}
			defer out.Close()

			// Get the data
			last_version, _ := GetLastVersion()
			fmt.Printf(color("Downloading last version (%s) ...", "response"), last_version)
			resp, err := http.Get(repo_url + "/binaries/" + runtime.GOOS + "/codepicnic")
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			// Writer the body to file
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				return err
			}

			err = os.Rename(file_tmp, cp_bin)

			if err != nil {
				return err
			}
			os.Chown(cp_bin, int(fi_sys.Uid), int(fi_sys.Gid))
			os.Chmod(cp_bin, fi.Mode())
			fmt.Printf(color(" Done.\n", "response"))

		}
	}
	return nil
}

func CmdCheck() {
	CreateConfigDir()
	if IsFirstCheck() == true {
		if IsLastVersion() == false {
			last_version, _ := GetLastVersion()
			fmt.Printf(color("Version %s is out! Update your version with 'codepicnic update'\n", "response"), last_version)
		}
	}

}
