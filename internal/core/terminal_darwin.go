//go:build darwin

package core

import (
	"os"

	"golang.org/x/sys/unix"
)

var (
	originalTermios *unix.Termios
	saved           bool
)

// SaveTerminalState saves the current terminal state
func SaveTerminalState() {
	fd := int(os.Stdin.Fd())
	if termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA); err == nil {
		originalTermios = termios
		saved = true
	}
}

// RestoreTerminalState restores the terminal to its original state
func RestoreTerminalState() {
	if !saved || originalTermios == nil {
		return
	}
	fd := int(os.Stdin.Fd())
	unix.IoctlSetTermios(fd, unix.TIOCSETA, originalTermios)
	saved = false
}
