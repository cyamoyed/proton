// Package shell 封装插件命令的 Shell 选择与运行环境构造，
// 供交互式终端（tui）与非交互执行（MCP 等）共用同一套规则。
package shell

import (
	"os"
	"runtime"
	"strings"

	"github.com/cyamoyed/proton/internal/dotenv"
)

// ShellCommand 根据命令内容与平台选择执行用的 Shell 及参数。
// 命令本身作为最后一个参数追加，调用方负责设置工作目录与环境。
func ShellCommand(command string) (name string, args []string) {
	if runtime.GOOS == "windows" {
		if needsPowerShell(command) {
			// PowerShell-specific syntax detected; bypass execution policy
			// so scripts like npm.ps1 are not blocked.
			return "powershell.exe", []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command}
		}
		// Default to cmd.exe — avoids PowerShell execution-policy blocks
		// on tools that ship .ps1 shims (npm, yarn, pnpm, etc.).
		return "cmd.exe", []string{"/c", command}
	}
	// Use login shell on Unix to load .bashrc/.zshrc
	// "-l" (login) loads shell initialization files
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return shell, []string{"-l", "-c", command}
}

// CommandEnv 返回子进程环境变量：加载目录下 .env 后与当前进程环境合并，
// 已存在的进程环境变量优先（.env 不覆盖）。无 .env 时返回 nil。
func CommandEnv(dir string) ([]string, error) {
	envVars, err := dotenv.LoadFromDir(dir)
	if err != nil {
		return nil, err
	}
	if len(envVars) == 0 {
		return nil, nil
	}
	return mergeEnv(os.Environ(), envVars), nil
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
