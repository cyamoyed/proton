// Package mcpserver 通过 Model Context Protocol（stdio 传输）暴露 proton 的
// 任务目录与执行能力，让 AI Agent 无需解析 CLI 文本即可发现并运行项目任务。
package mcpserver

import (
	"context"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// instructions 向连接的客户端描述推荐工作流。
const instructions = `Proton discovers runnable tasks (start, test, build, ...) in local repositories and polyglot monorepos.

Recommended workflow:
1. Call list_tasks to get the catalog; task IDs are exact strings like "root:build" or "services/api:run" — never guess or partially match them.
2. Call run_task with dry_run=true to review the command, working directory, source plugin, risk level, and required tools.
3. Call run_task again to execute. Tasks marked high-risk or requiring confirmation need approved=true, which should only be set after the user explicitly agrees.
4. Long-running commands (dev servers) are killed at the timeout; captured output is still returned with timed_out=true.

Commands run through the system shell in the task's project directory, with the project's .env loaded.`

// Serve 在给定流上运行 MCP server，直到客户端断开连接。
// stdout 专用于 JSON-RPC，诊断信息只写 stderr。
func Serve(stdin io.ReadCloser, stdout io.WriteCloser, stderr io.Writer, version string) error {
	server := NewServer(version)
	return server.Run(context.Background(), &mcp.IOTransport{Reader: stdin, Writer: stdout})
}

// NewServer 构建注册了全部工具的 proton MCP server。
func NewServer(version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "proton",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	registerTools(server)
	return server
}
