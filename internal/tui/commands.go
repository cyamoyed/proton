package tui

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	"github.com/cyamoyed/proton/internal/dotenv"
)

// ExecuteCommand runs a command directly without TUI
// Returns the exit code from the executed command (0 for success)
// Commands come from trusted plugin configuration and are shown to the user
// before execution. They intentionally support shell syntax.
func ExecuteCommand(command, dir string) int {
	return executeCommand(command, dir, true)
}

// ExecuteCommandDirect runs a selected command without clearing the terminal.
// Non-interactive CLI commands use it so their preceding output is preserved.
func ExecuteCommandDirect(command, dir string) int {
	return executeCommand(command, dir, false)
}

func executeCommand(command, dir string, shouldClear bool) int {
	if shouldClear {
		clearScreen()
	}

	// Validate command is not empty
	if strings.TrimSpace(command) == "" {
		fmt.Println("Error: empty command")
		return 1
	}

	// Load .env file from the project directory
	envVars, _ := dotenv.LoadFromDir(dir)

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		if needsPowerShell(command) {
			// PowerShell-specific syntax detected; bypass execution policy
			// so scripts like npm.ps1 are not blocked.
			cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command)
		} else {
			// Default to cmd.exe — avoids PowerShell execution-policy blocks
			// on tools that ship .ps1 shims (npm, yarn, pnpm, etc.).
			cmd = exec.Command("cmd.exe", "/c", command)
		}
	} else {
		// Use login shell on Unix to load .bashrc/.zshrc
		// "-l" (login) loads shell initialization files
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		cmd = exec.Command(shell, "-l", "-c", command)
	}
	cmd.Dir = dir

	// Inject .env variables into the child process environment
	if len(envVars) > 0 {
		cmd.Env = mergeEnv(os.Environ(), envVars)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Ignore SIGINT in parent process so Ctrl+C only affects the child
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	// Drain any pending signals
	go func() {
		for range sigCh {
			// Ignore signals while child is running
		}
	}()

	err := cmd.Run()

	// Restore default signal handling
	signal.Stop(sigCh)
	close(sigCh)

	if err != nil {
		// Extract exit code from error
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		return 1
	}

	return 0
}

func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Ignore error - clear screen is best effort
	_ = cmd.Run()
}

// needsPowerShell reports whether command contains syntax that requires
// PowerShell rather than cmd.exe. It looks for common PS-specific patterns:
// cmdlet names (Verb-Noun), PS variables ($), special operators, and
// pipeline assignments. Simple commands like "npm run build" return false.
func needsPowerShell(command string) bool {
	lower := strings.ToLower(strings.TrimSpace(command))
	// PowerShell cmdlet pattern: "Verb-Noun" (e.g. Get-Content, Set-Location)
	if strings.ContainsAny(lower, "$") {
		return true
	}
	// Common PS-only operators and syntax
	psPatterns := []string{
		"get-", "set-", "new-", "remove-", "invoke-", "write-", "read-",
		"start-", "stop-", "add-", "clear-", "copy-", "move-", "rename-",
		"select-", "where-", "foreach-", "sort-", "measure-", "compare-",
		"export-", "import-", "convertto-", "convertfrom-", "out-",
		"-and ", "-or ", "-not ", "-eq ", "-ne ", "-gt ", "-lt ", "-ge ", "-le ",
		"-match ", "-like ", "-contains ", "-replace ",
		"| where", "| select", "| foreach", "| sort",
		"@(", "$(", "$_", "$psitem",
		"powershell", "pwsh",
	}
	for _, p := range psPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// mergeEnv merges .env variables into the existing environment.
// Existing process environment variables take precedence (are not overridden).
func mergeEnv(existing []string, dotenvVars map[string]string) []string {
	// Build a set of existing keys to avoid duplicates
	existingKeys := make(map[string]bool, len(existing))
	for _, kv := range existing {
		if idx := strings.IndexByte(kv, '='); idx >= 0 {
			existingKeys[kv[:idx]] = true
		}
	}

	// Append .env variables that are not already in the environment
	result := make([]string, 0, len(existing)+len(dotenvVars))
	result = append(result, existing...)
	for k, v := range dotenvVars {
		if !existingKeys[k] {
			result = append(result, k+"="+v)
		}
	}
	return result
}
