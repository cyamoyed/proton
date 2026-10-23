package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
	"unicode/utf8"

	"github.com/cyamoyed/proton/internal/shell"
)

// maxOutputBytes 限制单次执行返回给客户端的输出大小，超出部分截断。
const maxOutputBytes = 1 << 20 // 1 MiB

// waitDelay 是超时杀进程后等待输出管道关闭的宽限期：孙进程可能仍持有管道。
const waitDelay = 3 * time.Second

const (
	defaultTimeoutSeconds = 120
	maxTimeoutSeconds     = 600
)

// capturedResult 是一次被捕获执行的结果。
type capturedResult struct {
	ExitCode int
	TimedOut bool
	Output   string
}

// resolveTimeout 规范化超时秒数：缺省 120，钳制到 [1, 600]。
func resolveTimeout(seconds *int) time.Duration {
	resolved := defaultTimeoutSeconds
	if seconds != nil {
		resolved = *seconds
	}
	if resolved < 1 {
		resolved = 1
	}
	if resolved > maxTimeoutSeconds {
		resolved = maxTimeoutSeconds
	}
	return time.Duration(resolved) * time.Second
}

// RunCaptured 在 dir 下通过系统 Shell 执行 command，捕获合并输出并在超时后
// 终止整个进程树。Shell 选择与 .env 注入规则与交互式终端完全一致；
// 子进程不接管终端，交互式命令会立即收到 EOF。
func RunCaptured(ctx context.Context, command, dir string, timeout time.Duration) capturedResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name, args := shell.ShellCommand(command)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.WaitDelay = waitDelay
	if env, _ := shell.CommandEnv(dir); len(env) > 0 {
		cmd.Env = env
	}
	setupProcessTreeKill(cmd)

	// stdout 与 stderr 指向同一 writer 时 os/exec 复用同一管道，无写入竞争。
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)

	result := capturedResult{TimedOut: timedOut}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
	}
	result.Output = capOutput(output.String())
	return result
}

// capOutput 截断超限输出并避免在多字节字符中间截断。
func capOutput(output string) string {
	if len(output) <= maxOutputBytes {
		return output
	}
	truncated := output[:maxOutputBytes]
	for i := 0; i < utf8.UTFMax && len(truncated) > 0; i++ {
		r, size := utf8.DecodeLastRuneInString(truncated)
		if r != utf8.RuneError || size != 1 {
			break
		}
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "\n...[proton: output truncated at 1 MiB]"
}
