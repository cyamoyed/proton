//go:build !windows

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installLocation() (dir, target string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("locating home directory: %w", err)
	}
	dir = filepath.Join(home, ".local", "bin")
	return dir, filepath.Join(dir, "proton"), nil
}

func addToUserPath(dir string) error {
	if pathContains(os.Getenv("PATH"), dir, ":") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locating home directory: %w", err)
	}
	rc := filepath.Join(home, ".profile")
	shell := os.Getenv("SHELL")
	if strings.HasSuffix(shell, "/zsh") {
		rc = filepath.Join(home, ".zshrc")
	} else if strings.HasSuffix(shell, "/bash") {
		rc = filepath.Join(home, ".bashrc")
	}
	line := `export PATH="$HOME/.local/bin:$PATH"`
	data, err := os.ReadFile(rc)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading shell profile %s: %w", rc, err)
	}
	if strings.Contains(string(data), line) {
		return nil
	}
	f, err := os.OpenFile(rc, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("opening shell profile %s: %w", rc, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "\n# Proton CLI\n%s\n", line); err != nil {
		return fmt.Errorf("updating shell profile %s: %w", rc, err)
	}
	return nil
}

func removeFromUserPath(string) error { return nil }

func removeInstalledExecutable(target string) error {
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", target, err)
	}
	return nil
}
