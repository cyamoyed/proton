package envcheck

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/fupeijun/proton/internal/plugin"
)

type EnvStatus struct {
	Name     string
	Version  string
	OK       bool
	Required bool
}

func CheckEnvironment(checks []plugin.EnvCheck) []EnvStatus {
	var statuses []EnvStatus

	for _, check := range checks {
		status := EnvStatus{
			Name:     check.Name,
			Required: check.Required,
		}

		// Try to run the check command
		parts := strings.Fields(check.Check)
		if len(parts) == 0 {
			statuses = append(statuses, status)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			// On Windows, use cmd.exe /c to run commands
			cmd = exec.CommandContext(ctx, "cmd.exe", "/c", check.Check)
		} else {
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		}

		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			status.OK = false
			statuses = append(statuses, status)
			continue
		}

		// Extract version from output
		status.Version = extractVersion(string(output))
		status.OK = true
		statuses = append(statuses, status)
	}

	return statuses
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
