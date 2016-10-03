package main

import (
	"github.com/ctcpip/notifize"
)

func NotifyDesktop() {

	notifize.Display("CodePicnic", "Console succesfully mounted", false, getHomeDir()+"/"+cfg_dir+"/"+notify_file)
}
