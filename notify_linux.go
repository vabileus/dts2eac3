//go:build linux

package main

import "os/exec"

func nativeNotify(title, body string) error {
	// notify-send is part of libnotify-bin and present on most desktops.
	cmd := exec.Command("notify-send", "-a", "dts2eac3", title, body)
	return cmd.Run()
}
