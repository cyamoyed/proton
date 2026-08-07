package installer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Install copies the running executable into the per-user installation
// directory and ensures that directory is available on PATH.
func Install() (string, error) {
	return InstallWithProgress(nil)
}

// InstallWithProgress installs Proton and reports user-visible milestones.
func InstallWithProgress(progress func(string)) (string, error) {
	report := func(message string) {
		if progress != nil {
			progress(message)
		}
	}
	report("Locating the installer...")
	source, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locating installer executable: %w", err)
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return "", fmt.Errorf("resolving installer executable: %w", err)
	}
	dir, target, err := installLocation()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating install directory %s: %w", dir, err)
	}
	report("Copying Proton to " + target + "...")
	if !sameFile(source, target) {
		if err := copyExecutable(source, target); err != nil {
			return "", err
		}
	}
	report("Updating your user PATH...")
	if err := addToUserPath(dir); err != nil {
		return "", err
	}
	return target, nil
}

// Uninstall removes the installed executable. On Unix the harmless PATH entry
// is retained; on Windows the install directory is removed from user PATH.
func Uninstall() (string, error) {
	return UninstallWithProgress(nil)
}

// UninstallWithProgress removes Proton and reports user-visible milestones.
func UninstallWithProgress(progress func(string)) (string, error) {
	report := func(message string) {
		if progress != nil {
			progress(message)
		}
	}
	dir, target, err := installLocation()
	if err != nil {
		return "", err
	}
	report("Removing Proton from your user PATH...")
	if err := removeFromUserPath(dir); err != nil {
		return "", err
	}
	report("Removing " + target + "...")
	if err := removeInstalledExecutable(target); err != nil {
		return "", err
	}
	return target, nil
}

func copyExecutable(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening installer %s: %w", source, err)
	}
	defer in.Close()
	temporary := target + ".new"
	out, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("creating %s: %w", temporary, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("copying executable: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("closing installed executable: %w", closeErr)
	}
	_ = os.Remove(target)
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("activating installed executable: %w", err)
	}
	return nil
}

func sameFile(a, b string) bool {
	aInfo, aErr := os.Stat(a)
	bInfo, bErr := os.Stat(b)
	return aErr == nil && bErr == nil && os.SameFile(aInfo, bInfo)
}

func pathContains(pathValue, dir string, separator string) bool {
	for _, entry := range strings.Split(pathValue, separator) {
		if strings.EqualFold(strings.TrimSpace(entry), dir) {
			return true
		}
	}
	return false
}
