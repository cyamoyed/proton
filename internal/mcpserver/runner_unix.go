//go:build !windows

package mcpserver

import (
	"os/exec"
	"syscall"
)

// setupProcessTreeKill 将 Shell 放入独立进程组，超时向整组发送 SIGKILL，
// 避免 Shell 退出后孙进程残留。
func setupProcessTreeKill(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
}
