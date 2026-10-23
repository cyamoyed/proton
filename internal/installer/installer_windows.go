//go:build windows

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func installLocation() (dir, target string, err error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, err = os.UserConfigDir()
		if err != nil {
			return "", "", fmt.Errorf("locating user data directory: %w", err)
		}
	}
	dir = filepath.Join(base, "Proton", "bin")
	return dir, filepath.Join(dir, "proton.exe"), nil
}

func addToUserPath(dir string) error {
	return updateWindowsPath(dir, true)
}

func removeFromUserPath(dir string) error {
	return updateWindowsPath(dir, false)
}

func updateWindowsPath(dir string, add bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("opening user environment settings: %w", err)
	}
	defer func() { _ = key.Close() }()
	current, _, err := key.GetStringValue("Path")
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("reading user PATH: %w", err)
	}
	parts := make([]string, 0)
	for _, entry := range strings.Split(current, ";") {
		entry = strings.TrimSpace(entry)
		if entry != "" && !strings.EqualFold(entry, dir) {
			parts = append(parts, entry)
		}
	}
	if add {
		parts = append(parts, dir)
	}
	if err := key.SetExpandStringValue("Path", strings.Join(parts, ";")); err != nil {
		return fmt.Errorf("writing user PATH: %w", err)
	}
	notifyEnvironmentChanged()
	return nil
}

func notifyEnvironmentChanged() {
	const (
		hwndBroadcast    = 0xffff
		wmSettingChange  = 0x001a
		smtoAbortIfHung  = 0x0002
		broadcastTimeout = 250
	)
	message, _ := windows.UTF16PtrFromString("Environment")
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
	var result uintptr
	_, _, _ = proc.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(message)), smtoAbortIfHung, broadcastTimeout, uintptr(unsafe.Pointer(&result)))
}

func removeInstalledExecutable(target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return nil
	}
	current, _ := os.Executable()
	if sameFile(current, target) {
		staged := target + ".remove"
		_ = os.Remove(staged)
		if err := os.Rename(target, staged); err != nil {
			return fmt.Errorf("staging running executable for removal: %w", err)
		}
		const cleanup = `ping -n 2 127.0.0.1 >nul & del /f /q "%PROTON_REMOVE_TARGET%"`
		cmd := exec.Command("cmd.exe", "/d", "/c", cleanup)
		cmd.Env = append(os.Environ(), "PROTON_REMOVE_TARGET="+staged)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("scheduling executable removal: %w", err)
		}
		return nil
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", target, err)
	}
	return nil
}
