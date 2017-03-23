package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/CodePicnic/codepicnic-go"
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/system"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"text/template"
)

//const site = "https://codepicnic.com"

const FragSeparator = ':'
const cfg_dir = ".codepicnic"
const cfg_file = "config"
const cfg_log = "codepicnic.log"
const share_dir_darwin = "/usr/local/codepicnic"
const share_dir_linux = "/usr/share/codepicnic"
const notify_file = "codepicnic.png"
const msg_bugs = "While where’re on beta, please write us your thoughts/bugs at bugs@codepicnic.com"

var version string
var site string
var swarm_host string
var format string
var debug string

var user_agent = "CodePicnic-CLI/" + version + " (" + GetOSVersion() + ")"
var config_dir = getHomeDir() + string(filepath.Separator) + cfg_dir
var msg_rwperms = "Make sure you have read and write permissions to " + config_dir + " directory."

var consoles_short_url = "https://codp.in/c/"
var repo_url = "http://deb.codepicnic.com"

// https://github.com/docker/docker/blob/master/cli/command/container/cp.go
func splitContainerFromPath(arg string) (container, path string) {
	if system.IsAbs(arg) {
		return "", arg
	}

	parts := strings.SplitN(arg, ":", 2)

	if len(parts) == 1 || strings.HasPrefix(parts[0], ".") {
		return "", arg
	}

	return parts[0], parts[1]
}

func getHomeDir() string {

	user_data, err := user.Current()
	if err != nil {
		return ""
	}
	return user_data.HomeDir

}

func ConnectConsole(access_token string, container_name string) {

	defaultHeaders := map[string]string{"User-Agent": "Docker-Client/1.10.3 (linux)"}
	cli, err := client.NewClient(swarm_host, "v1.22", nil, defaultHeaders)
	if err != nil {
		logrus.Fatalf("Error NewClient: %s", err)
		panic(msg_bugs)
	}
	//r, err := cli.ContainerInspect(context.Background(), container_name)
	r, err := cli.ContainerExecCreate(context.Background(), container_name, types.ExecConfig{User: "", Cmd: []string{"bash"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	if err != nil {
		logrus.Fatalf("Error ExecCreate: %s", err)
		panic(msg_bugs)
	}
	//fmt.Println(r.ID)

	aResp, err := cli.ContainerExecAttach(context.Background(), r.ID, types.ExecConfig{Tty: true, Cmd: []string{"bash"}, Env: nil, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})

	if err != nil {
		logrus.Fatalf("Error ExecAttach: %s", err)
		panic(msg_bugs)
	}
	tty := true
	if err != nil {
		logrus.Fatalf("Couldn't attach to container: %s", err)
	}
	defer aResp.Close()
	receiveStdout := make(chan error, 1)
	if os.Stdout != nil || os.Stderr != nil {
		go func() {
			fmt.Printf("Reader: %s", aResp.Reader)
			// When TTY is ON, use regular copy
			if tty && os.Stdout != nil {
				_, err = io.Copy(os.Stdout, aResp.Reader)
			} else {
				_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, aResp.Reader)
			}
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if os.Stdin != nil {
			io.Copy(aResp.Conn, os.Stdin)
			//fmt.Printf("stdinDone\n")
		}

		if err := aResp.CloseWrite(); err != nil {
			if strings.HasSuffix(err.Error(), "use of closed network connection") {
				//Connection already closed
			} else {
				logrus.Fatalf("Couldn't send EOF: %s", err)
			}
		}
		//close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			logrus.Fatalf("Error receiveStdout: %s", err)
		}
	case <-stdinDone:
		if os.Stdout != nil || os.Stderr != nil {
			if err := <-receiveStdout; err != nil {
				logrus.Fatalf("Error receiveStdout: %s", err)
			}
		}
	}
	close(stdinDone)
	//stdinw := bufio.NewReader(os.Stdin)
	//fmt.Printf("done\n")
	aResp.Conn.Close()
	return
}

func init() {
	log_file := config_dir + string(filepath.Separator) + cfg_log
	log_fh, err := os.OpenFile(log_file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		//login to stderr
		logrus.SetOutput(ioutil.Discard)
	} else {
		//defer log_fh.Close()
		logrus.SetOutput(log_fh)
	}
	if debug == "true" {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	//token, _ := codepicnic.GetToken()
	//logrus.Debug("Token: ", token)
}

func main() {
	app := cli.NewApp()
	cli.HelpPrinter = func(out io.Writer, templ string, data interface{}) {
		funcMap := template.FuncMap{
			"join": strings.Join,
		}
		var b bytes.Buffer
		outbuf := bufio.NewWriter(&b)
		w := tabwriter.NewWriter(outbuf, 1, 8, 2, ' ', 0)
		t := template.Must(template.New("help").Funcs(funcMap).Parse(templ))
		err := t.Execute(w, data)
		if err != nil {
			// If the writer is closed, t.Execute will fail, and there's nothing
			// we can do to recover.
			if os.Getenv("CLI_TEMPLATE_ERROR_DEBUG") != "" {
				fmt.Printf("CLI TEMPLATE ERROR: %#v\n", err)
			}
			return
		}

		w.Flush()
		outbuf.Flush()
		//fmt.Printf("%v \n", b.String())
		scanner := bufio.NewScanner(strings.NewReader(b.String()))
		for scanner.Scan() {
			//first line is the title
			line := scanner.Text()
			if strings.HasSuffix(line, ":") {
				fmt.Println(color(line, "prompt"))
			} else if strings.HasPrefix(line, "     ") || strings.HasPrefix(line, "   --") {
				words := strings.Fields(line)
				if len(words) > 0 {
					iscommand := true
					command := ""
					for i := range words {
						if iscommand == true {
							command = command + " " + words[i]
							if strings.HasSuffix(words[i], ",") {
								iscommand = true
							} else {
								iscommand = false
							}
						}
					}
					command_line := strings.Replace(line, command, color(command, "data"), 1)
					fmt.Println(command_line)
				}
			} else {
				fmt.Println(color(line, "response"))

			}

			if err := scanner.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "reading standard input:", err)
			}
		}
	}
	app.Version = version
	app.Name = "codepicnic"
	app.Usage = "A CLI tool to manage your CodePicnic consoles"
	var container_size, container_type, title, hostname, current_mode string
	var client_id, client_secret string
	client_id, client_secret = GetCredentialsFromFile()
	err := codepicnic.Init(client_id, client_secret)
	if err != nil {
		fmt.Printf(color("Authorization error", "error"))
	}
	codepicnic.SetUserAgent(user_agent)
	app.Action = func(c *cli.Context) error {
		//Start the REPL if not argument given

		cs := make(chan os.Signal, 2)
		signal.Notify(cs, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-cs
			fmt.Println(color("Bye!", "exit"))
			os.Exit(0)
		}()
		if c.NArg() == 0 {
			Repl(c)
		} else {
			fmt.Println(color("Command not recognized. Have you tried 'codepicnic help'?", "response"))
		}
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:   "bgmount",
			Usage:  "mount /app filesystem from a container",
			Hidden: true,
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				client_id, client_secret := GetCredentialsFromFile()
				err := codepicnic.Init(client_id, client_secret)
				if err != nil {
					fmt.Printf(color("Authorization error", "error"))
				}
				CmdMountConsole(c.Args())
				return nil
			},
		},
		{
			Name:   "check",
			Usage:  "check version",
			Hidden: true,
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				err := CmdCheck()
				if err != nil {
					fmt.Println(color(msg_rwperms, "error"))
				}
				return nil
			},
		},
		{
			Name: "clear",
			//Aliases: []string{"cls"},
			Usage:     "clear screen",
			ArgsUsage: " ",
			Action: func(c *cli.Context) error {
				CmdClearScreen()
				return nil
			},
		},
		{
			Name:      "configure",
			Usage:     "save configuration",
			ArgsUsage: " ",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "id",
					Value:       "",
					Destination: &client_id,
				},
				cli.StringFlag{
					Name:        "secret",
					Value:       "",
					Destination: &client_secret,
				},
			},
			Action: func(c *cli.Context) error {
				if c.NumFlags() == 0 {
					CmdConfigure()
				} else {
					CmdConfigureCredentials(client_id, client_secret)
				}
				return nil
			},
		},
		{
			Name:      "connect",
			Usage:     "connect to a console",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var console string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
				} else if c.NArg() == 1 {
					console = c.Args().Get(0)
				} else {
					//print error
				}
				CmdConnectConsole(console)
				return nil
			},
		},
		{
			Name:      "control",
			Usage:     "connect to a console and mount it as a local filesystem",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var mountbase string
				var input_unmount string
				var console_id string
				if c.NArg() == 0 {
					console_id = GetFromPrompt("Console Id", "")
				} else if c.NArg() == 1 {
					console_id = c.Args().Get(0)
				} else {
					//print error
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
				return nil
			},
		},

		{
			Name:      "copy",
			Usage:     "copy a file from/to a console",
			ArgsUsage: "[FILE_PATH] [CONSOLE_ID]:[DESTINATION_FILE_PATH] or [CONSOLE_ID]:[FILE_PATH] [DESTINATION_FILE_PATH]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var copy_src, copy_dst, src_container, src_path, dst_path, dst_container string
				if c.NArg() == 0 {
					fmt.Printf(color("Copy a file from/to a console. Don't forget to include ':' after the Id of your console.\n", "response"))
					copy_src = GetFromPrompt("Source", "")
					copy_dst = GetFromPrompt("Destination", "")
				} else if c.NArg() == 2 {
					copy_src = c.Args().Get(0)
					copy_dst = c.Args().Get(1)
				} else {
					//print error
				}
				src_container, src_path = splitContainerFromPath(copy_src)
				dst_container, dst_path = splitContainerFromPath(copy_dst)
				if dst_path == "." {
					dst_path = src_path
				}
				if src_container != "" {
					CmdDownloadFromConsole(src_container, src_path, dst_path)
				}
				if dst_container != "" {
					CmdUploadToConsole(dst_container, dst_path, src_path)
				}

				return nil
			},
		},
		{
			Name: "create",
			//Aliases: []string{"c"},
			Usage:     "create and start a new console",
			ArgsUsage: " ",

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "size",
					Value:       "medium",
					Usage:       "Container Size",
					Destination: &container_size,
				},
				cli.StringFlag{
					Name:        "type",
					Value:       "bash",
					Usage:       "Container Type",
					Destination: &container_type,
				},
				cli.StringFlag{
					Name:        "title",
					Value:       "",
					Usage:       "Pick a name for your console. Make it personal!",
					Destination: &title,
				},

				cli.StringFlag{
					Name:        "hostname",
					Value:       "",
					Usage:       "Any name you'd like to be used as your console hostname.",
					Destination: &hostname,
				},

				cli.StringFlag{
					Name:        "mode",
					Value:       "draft",
					Usage:       "The mode the console is currently in.",
					Destination: &current_mode,
				},
			},

			Action: func(c *cli.Context) error {
				CmdValidateCredentials()

				var console ConsoleExtra
				if c.NumFlags() == 0 {
					console = ConsoleExtra{}
					CmdCreateConsole(console)
				} else {
					console.Size = container_size
					console.Type = container_type
					console.Title = title
					console.Hostname = hostname
					console.Mode = current_mode
					CmdCreateConsole(console)
				}
				/*
					var console ConsoleExtra

					if c.NumFlags() == 0 {

						reader_type := bufio.NewReader(os.Stdin)
						fmt.Print("Type?(bash,ruby,python ... )[bash]: ")
						input, _ := reader_type.ReadString('\n')
						container_type = strings.TrimRight(input, "\r\n")
						reader_title := bufio.NewReader(os.Stdin)
						fmt.Print("Title?[]: ")
						input, _ = reader_title.ReadString('\n')
						title = strings.TrimRight(input, "\r\n")
						if container_type == "" {
							fmt.Println("type")
							container_type = "bash"
						}

					}
					console.Size = container_size
					console.Type = container_type
					console.Title = title
					console.Hostname = hostname
					console.Mode = current_mode

					fmt.Printf("Creating console ...")
					container_name, console_url := CreateConsole(access_token, console)
					fmt.Printf("done. * %s \n", container_name)
					fmt.Printf("%s \n", console_url)
				*/
				return nil
			},
		},
		{
			Name:      "exec",
			Usage:     "exec a command into console",
			ArgsUsage: "[CONSOLE_ID] \"[COMMAND]\"",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var console, command string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
					command = GetFromPrompt("Command", "")
				} else if c.NArg() == 1 {
					console = c.Args().Get(0)
					command = GetFromPrompt("Command", "")
				} else if c.NArg() == 2 {
					console = c.Args().Get(0)
					command = c.Args().Get(1)
				} else {
					return nil
				}
				CmdExecConsole(console, command)
				return nil
			},
		},
		{
			Name:      "exit",
			Usage:     "exit the REPL",
			ArgsUsage: " ",
			Action: func(c *cli.Context) error {
				fmt.Println(color("Bye!", "exit"))
				return nil
			},
		},
		{
			Name:      "inspect",
			Usage:     "inspect a console",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				CmdInspectConsole(c.Args()[0])
				return nil
			},
		},

		{
			Name: "list",
			//Aliases: []string{"ls"},
			Usage:     "list consoles",
			ArgsUsage: " ",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "format",
					Value:       "text",
					Usage:       "Output format: text, json",
					Destination: &format,
				},
			},
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				CmdListConsoles()
				return nil
			},
		},
		{
			Name:      "mount",
			Usage:     "mount /app filesystem from a container",
			ArgsUsage: "[CONSOLE_ID] [DESTINATION_DIRECTORY]",
			//Flags: []cli.Flag{
			//	cli.BoolFlag{
			//		Name:        "debug",
			//		Usage:       "Debugging",
			//		Destination: &debug,
			//	},
			//},
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()

				var mountbase string
				var input_unmount string
				var console_id string
				if c.NArg() == 0 {
					console_id = GetConsoleFromPrompt()
					mountbase = GetMountFromPrompt()
				} else if c.NArg() == 1 {
					console_id = c.Args().Get(0)
					mountbase = GetMountFromPrompt()
				} else if c.NArg() == 2 {
					console_id = c.Args().Get(0)
					mountbase = c.Args().Get(1)
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

				//CmdMountConsole(c.Args())
				return nil
			},
		},
		{
			Name:      "remove",
			Usage:     "remove a console",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var console string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
					if console == "" {
						fmt.Printf(color("This is not a valid console. Please try again \n", "error"))
						return nil
					}
				} else if c.NArg() == 1 {
					console = c.Args().Get(0)
				} else {
					return nil
				}
				CmdRemoveConsole(console)
				return nil
			},
		},
		{
			Name:      "restart",
			Usage:     "restart a console",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var console string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
					if console == "" {
						fmt.Printf(color("This is not a valid console. Please try again \n", "error"))
						return nil
					}
				} else if c.NArg() == 1 {
					console = c.Args().Get(0)
				} else {
					return nil
				}
				CmdRestartConsole(console)
				return nil
			},
		},
		{
			Name:      "stacks",
			Usage:     "list stacks",
			ArgsUsage: " ",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "format",
					Value:       "text",
					Usage:       "Output format: text, json",
					Destination: &format,
				},
			},
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				CmdListStacks()
				return nil
			},
		},
		{
			Name:      "start",
			Usage:     "start a console",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var console string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
					if console == "" {
						fmt.Printf(color("This is not a valid console. Please try again \n", "error"))
						return nil
					}
				} else if c.NArg() == 1 {
					console = c.Args().Get(0)
				} else {
					return nil
				}
				CmdStartConsole(console)
				return nil
			},
		},
		{
			Name:      "stop",
			Usage:     "stop a console",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				CmdValidateCredentials()
				var console string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
					if console == "" {
						fmt.Printf(color("This is not a valid console. Please try again \n", "error"))
						return nil
					}
				} else if c.NArg() == 1 {
					console = c.Args().Get(0)
				} else {
					return nil
				}
				CmdStopConsole(console)
				return nil
			},
		},
		{
			Name:      "unmount",
			Usage:     "unmount /app filesystem from a container",
			ArgsUsage: "[CONSOLE_ID]",
			Action: func(c *cli.Context) error {
				var console string
				if c.NArg() == 0 {
					console = GetFromPrompt("Console Id", "")
					if console == "" {
						fmt.Printf(color("This is not a valid console. Please try again \n", "error"))
						return nil
					}
				} else if c.NArg() == 1 {
					if c.Args()[0] == "all" {
						CmdUnmountAllConsoles()
					} else {
						console = c.Args().Get(0)
					}
				} else {
					return nil
				}
				CmdUnmountConsole(console)
				return nil
			},
		},
		{
			Name:  "update",
			Usage: "update CodePicnic",
			Action: func(c *cli.Context) error {
				CmdUpdate()
				return nil
			},
		},
	}
	err = CmdCheck()
	if err != nil {
		fmt.Println(color(msg_rwperms, "error"))
		color_exit()
		return
	}
	app.Run(os.Args)
	color_exit()
}
