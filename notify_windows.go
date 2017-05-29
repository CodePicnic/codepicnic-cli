package main

import (
	"fmt"

	toast "gopkg.in/toast.v1"
)

func NotifyDesktop(message string) {
	notification := toast.Notification{
		AppID:   "CodePicnic",
		Title:   "CodePicnic",
		Message: message,
		//Icon:    "go.png", // This file must exist (remove this line if it doesn't)
		/*Actions: []toast.Action{
			{"protocol", "I'm a button", ""},
			{"protocol", "Me too!", ""},
		},*/
	}
	err := notification.Push()
	if err != nil {
		fmt.Println(err)
	}
}
