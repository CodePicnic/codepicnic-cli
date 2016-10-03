// +build darwin
package main

import (
	"github.com/deckarep/gosx-notifier"
)

func NotifyDesktop() {
	note := gosxnotifier.NewNotification("Console succesfully mounted")
	note.Title = "CodePicnic"
	note.Push()

}
