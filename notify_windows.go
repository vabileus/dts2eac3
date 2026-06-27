//go:build windows

package main

import (
	"os"
	"os/exec"
)

// nativeNotify shows a real Windows toast via the WinRT ToastNotificationManager.
// The script reads title/body from environment variables to avoid any quoting
// or injection issues with the user-supplied strings.
func nativeNotify(title, body string) error {
	const script = `
$ErrorActionPreference = 'Stop'
$null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType=WindowsRuntime]
$tmpl = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$nodes = $tmpl.GetElementsByTagName('text')
$null = $nodes.Item(0).AppendChild($tmpl.CreateTextNode($env:DTS_NOTIFY_TITLE))
$null = $nodes.Item(1).AppendChild($tmpl.CreateTextNode($env:DTS_NOTIFY_BODY))
$toast = [Windows.UI.Notifications.ToastNotification]::new($tmpl)
$appId = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appId).Show($toast)
`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(),
		"DTS_NOTIFY_TITLE="+title,
		"DTS_NOTIFY_BODY="+body,
	)
	return cmd.Run()
}
