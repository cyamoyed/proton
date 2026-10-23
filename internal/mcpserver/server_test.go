package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// startServer 在内存传输上启动 proton MCP server 并返回已连接的客户端会话。
func startServer(t *testing.T) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	server := NewServer("test")
	go func() {
		_ = server.Run(context.Background(), serverTransport)
	}()
	client := mcp.NewClient(&mcp.Implementation{Name: "proton-mcp-test"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connecting client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callTool[T any](t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (*mcp.CallToolResult, T) {
	t.Helper()
	var zero T
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res == nil {
		t.Fatalf("CallTool(%s): nil result", name)
	}
	if res.StructuredContent == nil {
		return res, zero
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshaling structured content: %v", err)
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshaling %s result: %v", name, err)
	}
	return res, out
}

// resultText 拼接结果中的文本内容，便于断言错误提示。
func resultText(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, content := range res.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			sb.WriteString(text.Text)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolateHome 将全局配置指到空目录，避免读到真实的 ~/.proton/config.yaml。
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// writePluginProject 创建一个由自定义插件驱动的临时项目，返回项目目录。
func writePluginProject(t *testing.T, pluginYAML string) string {
	t.Helper()
	isolateHome(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test-plugin.yaml"), pluginYAML)
	writeFile(t, filepath.Join(dir, ".proton.yaml"), "plugins:\n  - test-plugin.yaml\n")
	writeFile(t, filepath.Join(dir, "proton-mcp-marker.txt"), "")
	return dir
}

func testPluginYAML(sleepCommand string) string {
	return fmt.Sprintf(`name: mcp-test
version: 1.0.0
detect:
  files:
    - proton-mcp-marker.txt
commands:
  - name: Echo
    description: print a marker
    run: echo proton-mcp-smoke
  - name: Fail
    run: exit 3
  - name: Risky
    run: echo risky-done
    risk: high
  - name: Sleep
    run: %s
env:
  - name: EchoTool
    check: echo version
    required: true
`, sleepCommand)
}

func sleepCommand() string {
	if runtime.GOOS == "windows" {
		return "ping -n 30 127.0.0.1"
	}
	return "sleep 30"
}

func TestServerExposesProtonTools(t *testing.T) {
	session := startServer(t)
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{"list_tasks", "run_task", "check_env"} {
		if !slices.Contains(names, want) {
			t.Errorf("tool %q missing from %v", want, names)
		}
	}
}

func TestListTasksReturnsCatalog(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/mcp\n\ngo 1.24\n")

	session := startServer(t)
	res, out := callTool[listTasksOutput](t, session, "list_tasks", map[string]any{
		"directory": dir,
		"depth":     0,
	})
	if res.IsError {
		t.Fatalf("list_tasks failed: %s", resultText(res))
	}
	if out.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", out.SchemaVersion)
	}
	if !filepath.IsAbs(out.Root) {
		t.Errorf("root %q is not absolute", out.Root)
	}
	if len(out.Tasks) == 0 {
		t.Fatal("no tasks listed for a go project")
	}
	for _, task := range out.Tasks {
		if task.ID == "" || task.Command == "" {
			t.Errorf("task %+v has empty ID or command", task)
		}
	}
	// 二次调用返回相同目录，任务 ID 稳定。
	_, again := callTool[listTasksOutput](t, session, "list_tasks", map[string]any{
		"directory": dir,
		"depth":     0,
	})
	if len(again.Tasks) != len(out.Tasks) {
		t.Errorf("task count changed between calls: %d then %d", len(out.Tasks), len(again.Tasks))
	}
}

func TestRunTaskDryRunReturnsMetadata(t *testing.T) {
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	res, out := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":   "root:echo",
		"directory": dir,
		"dry_run":   true,
	})
	if res.IsError {
		t.Fatalf("dry_run failed: %s", resultText(res))
	}
	if out.Executed {
		t.Error("dry_run must not execute")
	}
	if out.Task.Command != "echo proton-mcp-smoke" {
		t.Errorf("task.Command = %q", out.Task.Command)
	}
	if !filepath.IsAbs(out.Task.ProjectDir) {
		t.Errorf("task.ProjectDir %q is not absolute", out.Task.ProjectDir)
	}
	if out.Output != "" {
		t.Errorf("dry_run output = %q, want empty", out.Output)
	}
}

func TestRunTaskRequiresApprovalForHighRisk(t *testing.T) {
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	res, _ := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":   "root:risky",
		"directory": dir,
	})
	if !res.IsError {
		t.Fatal("high-risk task without approved must be an error result")
	}
	if text := resultText(res); !strings.Contains(text, "approved") {
		t.Errorf("error text %q does not mention approved", text)
	}
}

func TestRunTaskExecutesAndCapturesOutput(t *testing.T) {
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	res, out := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":   "root:echo",
		"directory": dir,
	})
	if res.IsError {
		t.Fatalf("run_task failed: %s", resultText(res))
	}
	if !out.Executed {
		t.Error("executed = false, want true")
	}
	if out.ExitCode != 0 {
		t.Errorf("exit_code = %d, want 0", out.ExitCode)
	}
	if !strings.Contains(out.Output, "proton-mcp-smoke") {
		t.Errorf("output %q does not contain marker", out.Output)
	}
}

func TestRunTaskReportsNonZeroExit(t *testing.T) {
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	res, out := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":   "root:fail",
		"directory": dir,
	})
	if !res.IsError {
		t.Fatal("non-zero exit must be an error result")
	}
	if out.ExitCode != 3 {
		t.Errorf("exit_code = %d, want 3", out.ExitCode)
	}
}

func TestRunTaskTimesOutLongRunningCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("timeout test runs a real process")
	}
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	_, out := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":         "root:sleep",
		"directory":       dir,
		"timeout_seconds": 1,
	})
	if !out.TimedOut {
		t.Errorf("timed_out = false, want true (exit_code=%d output=%q)", out.ExitCode, out.Output)
	}
	if !out.Executed {
		t.Error("executed = false, want true")
	}
}

func TestRunTaskUnknownID(t *testing.T) {
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	res, _ := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":   "root:nope",
		"directory": dir,
	})
	if !res.IsError {
		t.Fatal("unknown task ID must be an error result")
	}
	if text := resultText(res); !strings.Contains(text, "list_tasks") {
		t.Errorf("error text %q does not mention list_tasks", text)
	}
}

func TestRunTaskMissingRequiredTools(t *testing.T) {
	dir := writePluginProject(t, `name: mcp-broken-env
version: 1.0.0
detect:
  files:
    - proton-mcp-marker.txt
commands:
  - name: Echo
    run: echo never
env:
  - name: DefinitelyMissing
    check: proton-definitely-missing-tool-12345 --version
    required: true
`)
	session := startServer(t)
	res, _ := callTool[runTaskOutput](t, session, "run_task", map[string]any{
		"task_id":   "root:echo",
		"directory": dir,
	})
	if !res.IsError {
		t.Fatal("missing required tools must be an error result")
	}
	if text := resultText(res); !strings.Contains(text, "DefinitelyMissing") {
		t.Errorf("error text %q does not name the missing tool", text)
	}
}

func TestCheckEnvReportsToolStatuses(t *testing.T) {
	dir := writePluginProject(t, testPluginYAML(sleepCommand()))
	session := startServer(t)
	res, out := callTool[checkEnvOutput](t, session, "check_env", map[string]any{
		"directory": dir,
	})
	if res.IsError {
		t.Fatalf("check_env failed: %s", resultText(res))
	}
	if len(out.Tools) == 0 {
		t.Fatal("no tool statuses reported")
	}
	found := false
	for _, tool := range out.Tools {
		if tool.Name == "EchoTool" {
			found = true
			if !tool.OK {
				t.Errorf("EchoTool not ok: %+v", tool)
			}
			if !tool.Required {
				t.Errorf("EchoTool required = false, want true")
			}
		}
	}
	if !found {
		t.Errorf("EchoTool missing from %v", out.Tools)
	}
}

func TestServeUsesProvidedStreams(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/serve\n\ngo 1.24\n")

	// 通过 IOTransport 直接驱动 Serve：进程内构造客户端对话。
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	done := make(chan error, 1)
	go func() {
		done <- Serve(serverReader, serverWriter, io.Discard, "test")
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "proton-serve-test"}, nil)
	session, err := client.Connect(context.Background(), &mcp.IOTransport{Reader: clientReader, Writer: clientWriter}, nil)
	if err != nil {
		t.Fatalf("connect over Serve streams: %v", err)
	}
	res, out := callTool[listTasksOutput](t, session, "list_tasks", map[string]any{"directory": dir, "depth": 0})
	if res.IsError {
		t.Fatalf("list_tasks over Serve failed: %s", resultText(res))
	}
	if len(out.Tasks) == 0 {
		t.Fatal("no tasks listed over Serve streams")
	}
	if err := session.Close(); err != nil && !errors.Is(err, io.EOF) {
		t.Logf("closing session: %v", err)
	}
	// 关闭客户端侧管道，让 Serve 返回。
	_ = clientWriter.Close()
	_ = clientReader.Close()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("Serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not return after client streams closed")
	}
}
