package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/cyamoyed/proton/internal/core"
	"github.com/cyamoyed/proton/internal/envcheck"
	"github.com/cyamoyed/proton/internal/plugin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultDepth = 3

// list_tasks
type listTasksInput struct {
	Directory string `json:"directory,omitempty" jsonschema:"project directory to scan. Defaults to the current working directory"`
	Depth     *int   `json:"depth,omitempty" jsonschema:"maximum subdirectory scan depth (default 3. Use 0 to scan only the directory itself)"`
}

type listTasksOutput struct {
	SchemaVersion int         `json:"schema_version"`
	Root          string      `json:"root"`
	Warning       string      `json:"warning,omitempty"`
	Tasks         []core.Task `json:"tasks"`
}

// run_task
type runTaskInput struct {
	TaskID         string `json:"task_id" jsonschema:"exact task ID returned by list_tasks (for example root:build). IDs must match exactly"`
	Directory      string `json:"directory,omitempty" jsonschema:"project directory to scan for the task. Defaults to the current working directory"`
	Depth          *int   `json:"depth,omitempty" jsonschema:"maximum subdirectory scan depth (default 3. Use 0 to scan only the directory itself)"`
	DryRun         bool   `json:"dry_run,omitempty" jsonschema:"return the task details and command without executing anything"`
	Approved       bool   `json:"approved,omitempty" jsonschema:"approve execution of tasks marked high-risk or requiring confirmation. Only set this after the user explicitly agrees"`
	TimeoutSeconds *int   `json:"timeout_seconds,omitempty" jsonschema:"execution timeout in seconds (default 120, clamped to 1-600). Long-running commands such as dev servers are killed at the timeout and their captured output is still returned"`
}

type runTaskOutput struct {
	Task     core.Task `json:"task"`
	Executed bool      `json:"executed"`
	ExitCode int       `json:"exit_code,omitempty"`
	TimedOut bool      `json:"timed_out,omitempty"`
	Output   string    `json:"output,omitempty"`
}

// check_env
type checkEnvInput struct {
	Directory string `json:"directory,omitempty" jsonschema:"project directory to scan. Defaults to the current working directory"`
}

type envToolStatus struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required,omitempty"`
}

type checkEnvOutput struct {
	Tools []envToolStatus `json:"tools"`
}

func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:  "list_tasks",
		Title: "List proton tasks",
		Description: "List runnable tasks discovered in a project directory (polyglot monorepos included). " +
			"Returns the full catalog with stable task IDs, commands, project directories, risk levels and required tools. " +
			"Use the exact IDs with run_task; never guess or partially match them.",
	}, listTasks)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "run_task",
		Title: "Run a proton task",
		Description: "Run a proton task by its exact ID from list_tasks. " +
			"Call with dry_run=true first to review the command, directory and risk. " +
			"Tasks marked high-risk or requiring confirmation need approved=true. " +
			"Returns captured combined output, exit code and timed_out.",
	}, runTask)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_env",
		Title:       "Check required tools",
		Description: "Check whether the development tools required by the discovered tasks are installed, with detected versions.",
	}, checkEnv)
}

func listTasks(ctx context.Context, req *mcp.CallToolRequest, in listTasksInput) (*mcp.CallToolResult, listTasksOutput, error) {
	dir, depth, err := resolveScanArgs(in.Directory, in.Depth)
	if err != nil {
		return nil, listTasksOutput{}, err
	}
	discovery, tasks, err := discoverCatalog(dir, depth)
	if err != nil {
		return nil, listTasksOutput{}, err
	}
	out := listTasksOutput{
		SchemaVersion: 1,
		Root:          discovery.Root,
		Tasks:         tasks,
	}
	if tasks == nil {
		out.Tasks = []core.Task{}
	}
	if discovery.Warning != nil {
		out.Warning = discovery.Warning.Error()
	}
	return nil, out, nil
}

func runTask(ctx context.Context, req *mcp.CallToolRequest, in runTaskInput) (*mcp.CallToolResult, runTaskOutput, error) {
	dir, depth, err := resolveScanArgs(in.Directory, in.Depth)
	if err != nil {
		return nil, runTaskOutput{}, err
	}
	_, tasks, err := discoverCatalog(dir, depth)
	if err != nil {
		return nil, runTaskOutput{}, err
	}
	task, ok := core.FindTask(tasks, in.TaskID)
	if !ok {
		return nil, runTaskOutput{}, fmt.Errorf("task %q was not found; call list_tasks to see task IDs", in.TaskID)
	}
	out := runTaskOutput{Task: task}
	if in.DryRun {
		return nil, out, nil
	}
	if core.TaskNeedsApproval(task) && !in.Approved {
		return nil, out, fmt.Errorf(
			"task %q requires confirmation; review it with dry_run and execute again with approved=true", task.ID)
	}
	if missing := core.MissingRequiredTools(task); len(missing) > 0 {
		return nil, out, fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}

	result := RunCaptured(ctx, task.Command, task.ProjectDir, resolveTimeout(in.TimeoutSeconds))
	out.Executed = true
	out.ExitCode = result.ExitCode
	out.TimedOut = result.TimedOut
	out.Output = result.Output
	if result.ExitCode != 0 {
		return &mcp.CallToolResult{IsError: true}, out, nil
	}
	return nil, out, nil
}

func checkEnv(ctx context.Context, req *mcp.CallToolRequest, in checkEnvInput) (*mcp.CallToolResult, checkEnvOutput, error) {
	dir, _, err := resolveScanArgs(in.Directory, nil)
	if err != nil {
		return nil, checkEnvOutput{}, err
	}
	_, tasks, err := discoverCatalog(dir, defaultDepth)
	if err != nil {
		return nil, checkEnvOutput{}, err
	}

	seen := make(map[string]bool)
	var checks []plugin.EnvCheck
	for _, task := range tasks {
		for _, check := range task.Checks {
			key := check.Name + "\x00" + check.Check
			if !seen[key] {
				seen[key] = true
				checks = append(checks, check)
			}
		}
	}

	statuses := envcheck.CheckEnvironment(checks)
	tools := make([]envToolStatus, 0, len(statuses))
	for _, status := range statuses {
		tools = append(tools, envToolStatus{
			Name:     status.Name,
			Version:  status.Version,
			OK:       status.OK,
			Required: status.Required,
		})
	}
	return nil, checkEnvOutput{Tools: tools}, nil
}

// resolveScanArgs 规范化扫描参数：目录缺省为当前目录，深度缺省 3 且必须非负。
func resolveScanArgs(directory string, depth *int) (string, int, error) {
	if directory == "" {
		directory = "."
	}
	resolved := defaultDepth
	if depth != nil {
		resolved = *depth
	}
	if resolved < 0 {
		return "", 0, fmt.Errorf("depth must be zero or greater")
	}
	return directory, resolved, nil
}

// discoverCatalog 执行发现管线并返回任务目录，是三个工具共用的入口。
func discoverCatalog(dir string, depth int) (core.Discovery, []core.Task, error) {
	discovery, err := core.NewEngine().Discover(core.Options{Dir: dir, Depth: depth})
	if err != nil {
		return core.Discovery{}, nil, err
	}
	return discovery, core.Catalog(discovery), nil
}
