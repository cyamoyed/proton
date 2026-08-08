package envcheck

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cyamoyed/proton/internal/plugin"
)

type EnvStatus struct {
	Name     string
	Version  string
	OK       bool
	Required bool
}

func CheckEnvironment(checks []plugin.EnvCheck) []EnvStatus {
	statuses := make([]EnvStatus, len(checks))
	var wait sync.WaitGroup
	wait.Add(len(checks))
	for index, check := range checks {
		go func() {
			defer wait.Done()
			statuses[index] = checkEnvironment(check)
		}()
	}
	wait.Wait()
	return statuses
}

func checkEnvironment(check plugin.EnvCheck) EnvStatus {
	status := EnvStatus{
		Name:     check.Name,
		Required: check.Required,
	}

	// Try to run the check command
	parts := strings.Fields(check.Check)
	if len(parts) == 0 {
		return status
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// On Windows, use cmd.exe /c to run commands
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", check.Check)
	} else {
		cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return status
	}

	// Extract version from output
	status.Version = extractVersion(string(output))
	status.OK = true
	return status
}

func extractVersion(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		if len(line) > 0 && len(line) < 100 {
			return line
		}
	}
	return ""
}
