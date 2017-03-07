package main

import (
	"github.com/ctcpip/notifize"
)

func NotifyDesktop(message string) {

	notifize.Display("CodePicnic", message, false, share_dir_linux+"/"+notify_file)
}
