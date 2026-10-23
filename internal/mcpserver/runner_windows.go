//go:build windows

package mcpserver

import (
	"os/exec"
	"strconv"
)

// setupProcessTreeKill 让超时终止覆盖整个进程树：cmd.exe /c 只杀自身会
// 留下孙进程，taskkill /T /F 连同子进程一并结束。
func setupProcessTreeKill(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	}
}
