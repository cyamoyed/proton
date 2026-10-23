package tui

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"

	"github.com/cyamoyed/proton/internal/shell"
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

	name, args := shell.ShellCommand(command)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	// Inject .env variables from the project directory into the child
	// process environment; failures fall back to the plain environment.
	if env, _ := shell.CommandEnv(dir); len(env) > 0 {
		cmd.Env = env
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
