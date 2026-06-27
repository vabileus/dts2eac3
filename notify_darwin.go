//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

func nativeNotify(title, body string) error {
	// AppleScript: display notification "BODY" with title "TITLE"
	script := `display notification "` + escapeAS(body) +
		`" with title "` + escapeAS(title) + `"`
	return exec.Command("osascript", "-e", script).Run()
}

// escapeAS escapes double quotes and backslashes for an AppleScript literal.
func escapeAS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
