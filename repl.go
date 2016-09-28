package main

import (
	"bufio"
	"fmt"
	"github.com/codegangsta/cli"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const str_prompt = "CodePicnic> "

//color functions

func color(s string, t string) string {
	color_map := make(map[string]string)
	color_map["table"] = "244"
	color_map["response"] = "81"
	color_map["entry"] = "68"
	color_map["off"] = "66"
	color_map["data"] = "72"
	color_map["prompt"] = "133"
	color_map["exit"] = "81"
	color_map["error"] = "196"
	esc_start := "\033[38;5;"
	esc_m := "m"
	esc_default := "\x1b[39m"
	esc_end := "\033[38;5;68m"
	esc_data := "\033[38;5;72m"
	if t == "exit" {
		return esc_start + color_map[t] + esc_m + s + esc_default
	} else if t == "off" {
		return esc_start + color_map[t] + esc_m + s + esc_data
	} else {
		return esc_start + color_map[t] + esc_m + s + esc_end
	}
}

func color_exit() {
	esc_default := "\x1b[39m"
	fmt.Printf(esc_default)
	return
}
func TrimColor(s string) string {
	s = strings.TrimRight(s, "\r\n")
	s = strings.TrimLeft(s, "\033[38;5;68")
	return s
}

//clear functions
var clear map[string]func()

func init() {
	clear = make(map[string]func())
	clear["linux"] = func() {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
	clear["windows"] = func() {
		cmd := exec.Command("cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
	clear["darwin"] = func() {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}

func ClearScreen() {
	value, ok := clear[runtime.GOOS]
	if ok {
		value()
	}
}

//prompt functions

func GetConsoleFromPrompt() string {
	reader_console := bufio.NewReader(os.Stdin)
	fmt.Print(color("Console Id: ", "prompt"))
	input, _ := reader_console.ReadString('\n')
	return strings.TrimRight(input, "\r\n")
}
func GetMountFromPrompt() string {
	reader_console := bufio.NewReader(os.Stdin)
	fmt.Print(color("Mount Point [.]: ", "prompt"))
	input, _ := reader_console.ReadString('\n')
	return strings.TrimRight(input, "\r\n")
}

func GetFromPrompt(ask string, def string) string {
	reader_console := bufio.NewReader(os.Stdin)
	if def != "" {
		def = " [" + def + "]"
	}
	//fmt.Print(color("%s %s: ", "prompt"), ask, def)
	fmt.Printf(color("%s%s: ", "prompt"), ask, def)
	input, _ := reader_console.ReadString('\n')
	return strings.TrimRight(input, "\r\n")
}

func NewRepl(c *cli.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "." {
			break
		}
		//fmt.Println("hola ", line, "\n") // or do something else with line
		CmdConnectConsole("1e631b5132a75dc59bf132ad08a0faf6")
	}
}
func Repl(c *cli.Context) {
	var console_id string
	var copy_src, copy_dst, src_container, src_path, dst_path, dst_container string

	debug = false
	access_token, err := GetTokenAccess()
	if err != nil {
		if strings.Contains(err.Error(), "Disconnected") {
			fmt.Println(color("Can't connect to the CodePicnic API. Please verify your connection or try again.", "error"))
		} else if strings.Contains(err.Error(), "Not Authorized") {
			fmt.Println(color("It looks like you didn't authorize your credentials.", "error"))
			CmdConfigure()
		}
	} else if access_token == "" {
		fmt.Println(color("It looks like you didn't authorize your credentials.", "error"))
		CmdConfigure()
	}
	input := ""
	for input != "." {
		in := bufio.NewReader(os.Stdin)
		fmt.Print(color(str_prompt, "prompt"))
		input, err := in.ReadString('\n')
		input = TrimColor(input)
		inputArgs := strings.Fields(input)
		if len(inputArgs) == 0 {
			fmt.Println(color("Command not recognized. Have you tried 'help'?", "response"))
		} else {
			command := inputArgs[0]
			switch command {
			//case "clear", "cls":
			case "clear":
				CmdClearScreen()
			//case "list", "ls":
			case "configure":
				CmdConfigure()
			case "connect":
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
				} else if len(inputArgs) == 2 {
					console_id = inputArgs[1]
				} else {
					cli.ShowCommandHelp(c, command)
					break
				}
				CmdConnectConsole(console_id)
			case "control":
				var mountbase string
				var input_unmount string
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
				} else if len(inputArgs) == 2 {
					console_id = inputArgs[1]
				} else {
					cli.ShowCommandHelp(c, command)
					break
				}
				mountstat := GetMountsFromFile(console_id)
				if mountstat == "" {
					BgMountConsole(console_id, mountbase)
				} else {
					fmt.Printf(color("Container %s is already mounted in %s. \n", "response"), console_id, mountstat)
					reader_unmount := bufio.NewReader(os.Stdin)
					fmt.Printf(color("Do you want to unmount and then mount to a different directory? [ yes ]: ", "prompt"))
					input, _ := reader_unmount.ReadString('\n')
					input_unmount = TrimColor(input)
					if input_unmount == "yes" || input_unmount == "" {
						mountbase = GetMountFromPrompt()
						CmdUnmountConsole(console_id)
						BgMountConsole(console_id, mountbase)
					}
				}
				CmdConnectConsole(console_id)
			case "list":
				if len(inputArgs) > 1 {
					if inputArgs[2] == "json" {
						format = "json"
					} else {
						format = "text"
					}
				}
				CmdListConsoles()
			case "mount":
				var mountbase string
				var input_unmount string
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
					mountbase = GetMountFromPrompt()
				} else if len(inputArgs) > 2 {
					console_id = inputArgs[1]
					mountbase = inputArgs[2]
				} else {
					console_id = inputArgs[1]
				}
				//check if console is already mounted
				mountstat := GetMountsFromFile(console_id)
				if mountstat == "" {
					BgMountConsole(console_id, mountbase)
				} else {
					fmt.Printf(color("Container %s is already mounted in %s. \n", "response"), console_id, mountstat)
					reader_unmount := bufio.NewReader(os.Stdin)
					fmt.Printf(color("Do you want to unmount and then mount to a different directory? [ yes ]: ", "prompt"))
					input, _ := reader_unmount.ReadString('\n')
					input_unmount = TrimColor(input)
					if input_unmount == "yes" || input_unmount == "" {
						mountbase = GetMountFromPrompt()
						CmdUnmountConsole(console_id)
						BgMountConsole(console_id, mountbase)
					}
				}

				/*f, err := os.Create("cmd.log")
				cmd.Stdout = f
				cmd.Stderr = f*/
				//mountArgs := append(inputArgs[:0], inputArgs[1:]...)
				//CmdMountConsole(mountArgs)
			case "unmount":
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
				} else if len(inputArgs) > 2 {
					//Error print help
				} else {
					console_id = inputArgs[1]
				}
				CmdUnmountConsole(console_id)
			case "stop":
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
				} else if len(inputArgs) > 2 {
					//Error print help
				} else {
					console_id = inputArgs[1]
				}
				CmdStopConsole(console_id)
			case "start":
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
				} else if len(inputArgs) > 2 {
					//Error print help
				} else {
					console_id = inputArgs[1]
				}
				CmdStartConsole(console_id)
			case "restart":
				if len(inputArgs) < 2 {
					console_id = GetConsoleFromPrompt()
				} else if len(inputArgs) > 2 {
					//Error print help
				} else {
					console_id = inputArgs[1]
				}
				CmdRestartConsole(console_id)
			case "create":
				CmdCreateConsole()
			case "help":
				cli.ShowAppHelp(c)
			case "exit":
				fmt.Println(color("Bye!", "exit"))
				panic(err)
			case "exec":
				if len(inputArgs) < 2 {
					console_id = GetFromPrompt("Console Id", "")
					command = GetFromPrompt("Command", "")
				} else if len(inputArgs) > 2 {
					//Error print help
				} else {
					//Error print help
				}
				CmdExecConsole(console_id, command)
			case "copy":
				if len(inputArgs) < 2 {
					fmt.Printf(color("Copy a file from/to a console. Don't forget to include ':' after the Id of your console.\n", "response"))
					copy_src = GetFromPrompt("Source", "")
					copy_dst = GetFromPrompt("Destination", "")
				} else if len(inputArgs) > 2 {
					//Error print help
				} else {
					//Error print help
				}
				src_container, src_path = splitContainerFromPath(copy_src)
				dst_container, dst_path = splitContainerFromPath(copy_dst)
				if src_container != "" {
					CmdDownloadFromConsole(src_container, src_path, dst_path)
				}
				if dst_container != "" {
					CmdUploadToConsole(dst_container, dst_path, src_path)
				}
			default:
				fmt.Println(color("Command not recognized. Have you tried 'help'?", "response"))
			}
		}
	}
}
