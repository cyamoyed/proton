package core

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	originalMode uint32
	saved        bool
)

// SaveTerminalState saves the current console mode
func SaveTerminalState() {
	handle := windows.Handle(os.Stdin.Fd())
	if err := windows.GetConsoleMode(handle, &originalMode); err == nil {
		saved = true
	}
}

// RestoreTerminalState restores the console mode to its original state
func RestoreTerminalState() {
	if !saved {
		return
	}
	handle := windows.Handle(os.Stdin.Fd())
	// Restore is best-effort; nothing sensible to do if the call fails.
	_ = windows.SetConsoleMode(handle, originalMode)
	saved = false
}
