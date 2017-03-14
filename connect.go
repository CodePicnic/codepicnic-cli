package main

import (
	"crypto/tls"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/buger/goterm"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/promise"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/docker/pkg/term"
	"net/http"
	//"github.com/docker/go-connections/tlsconfig"
	"golang.org/x/net/context"

	"io"
	//"log"
	"os"
	//"strings"
)

//func (cli *DockerCli) holdHijackedConnection(tty bool, inputStream io.ReadCloser, outputStream, errorStream io.Writer, resp types.HijackedResponse) error {
func holdHijackedConnection(tty bool, inputStream io.ReadCloser, outputStream, errorStream io.Writer, resp types.HijackedResponse) error {
	var err error
	//outputStream = os.Stdout
	//inputStream = os.Stdin
	inFd, isTerminalIn := term.GetFdInfo(inputStream)
	//fmt.Printf("%+v", winsize)
	//terminalHeight := goterm.Height()
	//terminalWidth := goterm.Width()

	//fmt.Println("Terminal height : ", terminalHeight)
	//fmt.Println("Terminal width : ", terminalWidth)
	//outFd, isTerminalOut := term.GetFdInfo(outputStream)
	if isTerminalIn && os.Getenv("NORAW") == "" {
		state, err := term.SetRawTerminal(inFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(inFd, state)
	}
	receiveStdout := make(chan error, 1)
	if outputStream != nil || errorStream != nil {
		go func() {
			// When TTY is ON, use regular copy
			if tty && outputStream != nil {
				_, err = io.Copy(outputStream, resp.Reader)
			} else {
				_, err = stdcopy.StdCopy(outputStream, errorStream, resp.Reader)
			}
			logrus.Debugf("[hijack] End of stdout")
			receiveStdout <- err
		}()
	}

	stdinDone := make(chan struct{})
	go func() {
		if inputStream != nil {
			io.Copy(resp.Conn, inputStream)
		}

		if err := resp.CloseWrite(); err != nil {
			//fmt.Printf("Couldn't send EOF: %s", err)
		}
		close(stdinDone)
	}()

	select {
	case err := <-receiveStdout:
		if err != nil {
			logrus.Debugf("Error receiveStdout: %s", err)
			return err
		}
	case <-stdinDone:
		if outputStream != nil || errorStream != nil {
			if err := <-receiveStdout; err != nil {
				logrus.Debugf("Error receiveStdout: %s", err)
				return err
			}
		}
	}
	//close(stdinDone)
	return nil
}

//func ProxyConsole(access_token string, container_name string) error {
func ProxyConsole(access_token string, container_name string) {

	defaultHeaders := map[string]string{"User-Agent": "Docker-Client/1.10.3 (codepicnic)"}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	cl := &http.Client{Transport: tr}
	cli, err := client.NewClient(swarm_host, "v1.22", cl, defaultHeaders)
	if err != nil {
		fmt.Println(err)
		return
	}
	//r, err := cli.ContainerInspect(context.Background(), container_name)
	r, err := cli.ContainerExecCreate(context.Background(), container_name, types.ExecConfig{User: "", Cmd: []string{"bash"}, Tty: true, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println(r.ID)

	//resp, err := cli.client.ContainerExecAttach(execID, *execConfig)
	resize_options := types.ResizeOptions{
		Height: uint(goterm.Height()),
		Width:  uint(goterm.Width()),
	}
	aResp, err := cli.ContainerExecAttach(context.Background(), r.ID, types.ExecConfig{Tty: true, Cmd: []string{"bash"}, Env: nil, AttachStdin: true, AttachStderr: true, AttachStdout: true, Detach: false})
	cli.ContainerExecResize(context.Background(), r.ID, resize_options)
	// Interactive exec requested.
	var (
		out, stderr io.Writer
		in          io.ReadCloser
		errCh       chan error
	)
	in, out, stderr = term.StdStreams()
	if err != nil {
		return
		//return err
	}
	//defer aResp.Conn.Close()
	defer aResp.Close()
	tty := true
	if in != nil && tty {
	}
	errCh = promise.Go(func() error {
		return holdHijackedConnection(tty, in, out, stderr, aResp)
	})
	if err := <-errCh; err != nil {
		fmt.Printf("Error hijack: %s", err)
		return
	}
	//in.Close()
	return
	//return nil
}
