package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cyamoyed/proton/internal/core"
	"github.com/cyamoyed/proton/internal/installer"
	"github.com/cyamoyed/proton/internal/mcpserver"
	"github.com/cyamoyed/proton/internal/tui"
	"github.com/mattn/go-runewidth"
)

var version = "dev"

type catalogOutput struct {
	SchemaVersion int         `json:"schema_version"`
	Root          string      `json:"root"`
	Tasks         []core.Task `json:"tasks"`
}

func main() {
	if shouldInstall() {
		fmt.Println("Installing Proton...")
		target, err := installer.InstallWithProgress(func(message string) { fmt.Println("  " + message) })
		if err != nil {
			fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Proton installed successfully: %s\n", target)
		fmt.Println("Open a new terminal, navigate to a project, and run: proton")
		if launchedAsInstaller() {
			fmt.Println("\nPress Enter to close this window...")
			_, _ = fmt.Scanln()
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "uninstall" {
		fmt.Println("Uninstalling Proton...")
		target, err := installer.UninstallWithProgress(func(message string) { fmt.Println("  " + message) })
		if err != nil {
			fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Proton uninstalled: %s\n", target)
		return
	}

	err := dispatch(os.Args[1:], os.Stdout, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func dispatch(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "list":
			return runList(args[1:], stdout, stderr)
		case "run":
			return runTask(args[1:], stdout, stderr)
		case "mcp":
			return runMCP(args[1:], stderr)
		}
	}
	return runInteractive(args, stdout, stderr)
}

// runMCP 启动面向 AI Agent 的 stdio MCP server。JSON-RPC 固定使用真实的
// 进程标准流，与 dispatch 传入的 writer 无关。
func runMCP(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("proton mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: proton mcp")
		fmt.Fprintln(stderr, "\nStarts a Model Context Protocol server on stdio for AI agents.")
		fmt.Fprintln(stderr, "Tools: list_tasks, run_task, check_env. See the README for client configuration.")
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		flags.Usage()
		return fmt.Errorf("expected no arguments")
	}
	return mcpserver.Serve(os.Stdin, os.Stdout, stderr, version)
}

func runInteractive(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("proton", flag.ContinueOnError)
	flags.SetOutput(stderr)
	depth := flags.Int("depth", 3, "maximum subdirectory scan depth (0 scans only the target directory)")
	showVersion := flags.Bool("version", false, "print version and exit")
	noRecursive := flags.Bool("no-recursive", false, "scan only the target directory")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Proton discovers project tasks and runs them interactively or from automation.")
		fmt.Fprintln(stderr, "\nUsage:\n  proton [options] [directory]\n  proton list [options] [directory]\n  proton run [options] <task-id> [directory]\n  proton mcp\n  proton install\n  proton uninstall\n\nOptions:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, "proton", version)
		return nil
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return fmt.Errorf("expected at most one directory")
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	options, err := makeOptions(dir, *depth, *noRecursive)
	if err != nil {
		return err
	}
	return core.NewEngine().RunWithOptions(options)
}

func runList(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("proton list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	depth := flags.Int("depth", 3, "maximum subdirectory scan depth")
	noRecursive := flags.Bool("no-recursive", false, "scan only the target directory")
	jsonOutput := flags.Bool("json", false, "print a machine-readable task catalog")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: proton list [options] [directory]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return fmt.Errorf("expected at most one directory")
	}
	dir := "."
	if flags.NArg() == 1 {
		dir = flags.Arg(0)
	}
	options, err := makeOptions(dir, *depth, *noRecursive)
	if err != nil {
		return err
	}
	discovery, err := discover(options, stderr)
	if err != nil {
		return err
	}
	tasks := core.Catalog(discovery)
	if tasks == nil {
		tasks = []core.Task{}
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(catalogOutput{SchemaVersion: 1, Root: discovery.Root, Tasks: tasks})
	}
	printCatalog(stdout, tasks)
	return nil
}

func runTask(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("proton run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	depth := flags.Int("depth", 3, "maximum subdirectory scan depth")
	noRecursive := flags.Bool("no-recursive", false, "scan only the target directory")
	dryRun := flags.Bool("dry-run", false, "show the selected command without executing it")
	yes := flags.Bool("yes", false, "approve commands marked high-risk or requiring confirmation")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: proton run [options] <task-id> [directory]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() < 1 || flags.NArg() > 2 {
		flags.Usage()
		return fmt.Errorf("expected a task ID and optional directory")
	}
	taskID := flags.Arg(0)
	dir := "."
	if flags.NArg() == 2 {
		dir = flags.Arg(1)
	}
	options, err := makeOptions(dir, *depth, *noRecursive)
	if err != nil {
		return err
	}
	discovery, err := discover(options, stderr)
	if err != nil {
		return err
	}
	task, ok := core.FindTask(core.Catalog(discovery), taskID)
	if !ok {
		return fmt.Errorf("task %q was not found; run 'proton list' to see task IDs", taskID)
	}
	fmt.Fprintf(stdout, "任务: %s\n目录: %s\n来源: %s\n", task.ID, task.ProjectDir, task.Source)
	if task.Risk != "" {
		fmt.Fprintf(stdout, "风险: [%s]\n", riskLabel(task.Risk))
	}
	if len(task.Required) > 0 {
		fmt.Fprintf(stdout, "必需工具: %s\n", strings.Join(task.Required, ", "))
	}
	fmt.Fprintf(stdout, "命令: %s\n", task.Command)
	if *dryRun {
		return nil
	}
	if core.TaskNeedsApproval(task) && !*yes {
		return fmt.Errorf("task %q requires confirmation; review it with --dry-run and execute with --yes", task.ID)
	}
	if missing := core.MissingRequiredTools(task); len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}
	if exitCode := tui.ExecuteCommandDirect(task.Command, task.ProjectDir); exitCode != 0 {
		return fmt.Errorf("task %q exited with code %d", task.ID, exitCode)
	}
	return nil
}

func discover(options core.Options, stderr io.Writer) (core.Discovery, error) {
	discovery, err := core.NewEngine().Discover(options)
	if err != nil {
		return core.Discovery{}, err
	}
	if discovery.Warning != nil {
		fmt.Fprintf(stderr, "Warning: some plugins could not be loaded:\n%v\n", discovery.Warning)
	}
	return discovery, nil
}

func makeOptions(dir string, depth int, noRecursive bool) (core.Options, error) {
	if depth < 0 {
		return core.Options{}, fmt.Errorf("--depth must be zero or greater")
	}
	if noRecursive {
		depth = 0
	}
	return core.Options{Dir: dir, Depth: depth}, nil
}

// printCatalog 按项目分组输出任务目录，名称与命令排在最前便于快速定位。
// 只列出含启动类（应用启动入口）的模块，公共库、聚合根等无启动入口的模块自动隐藏。
func printCatalog(output io.Writer, tasks []core.Task) {
	if len(tasks) == 0 {
		fmt.Fprintln(output, "未检测到任务。")
		return
	}
	tasks = startableProjects(tasks)

	var group []core.Task
	firstGroup := true
	for index, task := range tasks {
		group = append(group, task)
		if index == len(tasks)-1 || tasks[index+1].Project != task.Project {
			if !firstGroup {
				fmt.Fprintln(output)
			}
			firstGroup = false
			printProjectGroup(output, group)
			group = nil
		}
	}
}

// printProjectGroup 输出单个项目的任务列表：一行一条，启动命令排最前并标记 ▶。
// 任务 ID 由「组头 ID 前缀 + 行尾短 ID」拼接而成，行尾仅在短 ID 与任务名不同时显示。
func printProjectGroup(output io.Writer, tasks []core.Task) {
	ordered := make([]core.Task, 0, len(tasks))
	for _, task := range tasks {
		if isStartupCommand(task.Command) {
			ordered = append(ordered, task)
		}
	}
	for _, task := range tasks {
		if !isStartupCommand(task.Command) {
			ordered = append(ordered, task)
		}
	}

	nameWidth := 0
	for _, task := range ordered {
		if width := runewidth.StringWidth(task.Name); width > nameWidth {
			nameWidth = width
		}
	}
	nameWidth += 2

	projectID := ordered[0].Project
	if projectID == "." {
		projectID = "root"
	}
	idPrefix := projectID + ":"

	fmt.Fprintf(output, "项目 %s [%s] · %d 个任务\n",
		projectID, strings.Join(ordered[0].Types, ", "), len(ordered))
	fmt.Fprintf(output, "  ID 前缀: %s\n", idPrefix)
	for _, task := range ordered {
		marker := "  "
		if isStartupCommand(task.Command) {
			marker = "▶ "
		}
		padding := nameWidth - runewidth.StringWidth(task.Name)
		line := marker + task.Name + strings.Repeat(" ", padding) + "$ " + task.Command + riskSuffix(task.Risk)
		if idSuffix := strings.TrimPrefix(task.ID, idPrefix); idSuffix != task.Name {
			line += "  (ID: " + idSuffix + ")"
		}
		fmt.Fprintln(output, line)
	}
}

// startableProjects 仅保留含启动类（能直接启动应用）的项目组。
// 若扫描范围内没有任何可启动模块，则原样返回，避免纯构建类项目（如 C++）被全部隐藏。
func startableProjects(tasks []core.Task) []core.Task {
	var groups [][]core.Task
	var group []core.Task
	for index, task := range tasks {
		group = append(group, task)
		if index == len(tasks)-1 || tasks[index+1].Project != task.Project {
			groups = append(groups, group)
			group = nil
		}
	}

	anyStartable := false
	for _, group := range groups {
		if groupStartable(group) {
			anyStartable = true
			break
		}
	}
	if !anyStartable {
		return tasks
	}

	kept := make([]core.Task, 0, len(tasks))
	for _, group := range groups {
		if groupStartable(group) {
			kept = append(kept, group...)
		}
	}
	return kept
}

// groupStartable 判断项目组内是否包含直接启动应用的命令
func groupStartable(tasks []core.Task) bool {
	for _, task := range tasks {
		if isStartupCommand(task.Command) {
			return true
		}
	}
	return false
}

// isStartupCommand 判断命令是否在本目录直接启动应用。
// 含 -pl 的多模块转发命令（如「mvn -pl gateway spring-boot:run」）不算：
// 启动类位于子模块，而不是命令所在的聚合根。
func isStartupCommand(run string) bool {
	if run == "" {
		return false
	}
	for _, token := range strings.Fields(run) {
		if token == "-pl" {
			return false
		}
	}
	switch {
	case strings.Contains(run, "spring-boot:run"),
		strings.Contains(run, "quarkus:dev"),
		strings.Contains(run, "exec:java"),
		strings.Contains(run, "gradle run"),
		strings.Contains(run, "gradlew run"),
		strings.Contains(run, "gradlew.bat run"),
		strings.Contains(run, "go run"),
		strings.Contains(run, "cargo run"),
		strings.Contains(run, "python main"),
		strings.Contains(run, "simctl launch"),
		strings.Contains(run, " run dev"),
		strings.Contains(run, " run start"):
		return true
	}
	return false
}

// riskSuffix 返回风险等级的中文后缀，例如「[高危]」
func riskSuffix(risk string) string {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "", "-", "none":
		return ""
	case "high":
		return " [高危]"
	case "medium":
		return " [中危]"
	case "low":
		return " [低危]"
	}
	return " [" + risk + "]"
}

// riskLabel 返回风险等级的中文标签，无风险时返回空字符串
func riskLabel(risk string) string {
	label := strings.TrimPrefix(strings.TrimSpace(riskSuffix(risk)), "[")
	return strings.TrimSuffix(label, "]")
}

func shouldInstall() bool {
	if len(os.Args) == 2 && os.Args[1] == "install" {
		return true
	}
	if len(os.Args) != 1 {
		return false
	}
	return launchedAsInstaller()
}

func launchedAsInstaller() bool {
	return len(os.Args) == 1 && strings.Contains(strings.ToLower(filepath.Base(os.Args[0])), "installer")
}
