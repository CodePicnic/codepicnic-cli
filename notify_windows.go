package main

import (
	"path/filepath"

	"github.com/kardianos/osext"

	toast "gopkg.in/toast.v1"
)

func NotifyDesktop(message string) {
	cp_bin, _ := osext.Executable()
	image_path := filepath.Dir(cp_bin)
	image_file := image_path + "\\codepicnic.png"

	notification := toast.Notification{
		AppID:   "CodePicnic",
		Title:   "CodePicnic",
		Message: message,
		Icon:    image_file, // This file must exist (remove this line if it doesn't)
		/*Actions: []toast.Action{
			{"protocol", "I'm a button", ""},
			{"protocol", "Me too!", ""},
		},*/
	}
	_ := notification.Push()
	//if err != nil {
	//	fmt.Println(err)
	//}
}
