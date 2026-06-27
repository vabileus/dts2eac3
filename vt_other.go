//go:build !windows

package main

// enableVirtualTerminal is a no-op outside Windows: Unix terminals support
// ANSI escapes natively.
func enableVirtualTerminal() {}
