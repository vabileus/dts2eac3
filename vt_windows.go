//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

// enableVirtualTerminal turns on ANSI escape handling for the Windows console
// (conhost/Windows Terminal on Win10+). No external dependency: we call
// kernel32 directly through the stdlib syscall package.
func enableVirtualTerminal() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	if r, _, _ := getConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return // not a console (e.g. redirected to a file)
	}
	setConsoleMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminalProcessing))
}
