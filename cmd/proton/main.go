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
	"github.com/cyamoyed/proton/internal/envcheck"
	"github.com/cyamoyed/proton/internal/installer"
	"github.com/cyamoyed/proton/internal/tui"
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
		}
	}
	return runInteractive(args, stdout, stderr)
}

func runInteractive(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("proton", flag.ContinueOnError)
	flags.SetOutput(stderr)
	depth := flags.Int("depth", 3, "maximum subdirectory scan depth (0 scans only the target directory)")
	showVersion := flags.Bool("version", false, "print version and exit")
	noRecursive := flags.Bool("no-recursive", false, "scan only the target directory")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Proton discovers project tasks and runs them interactively or from automation.")
		fmt.Fprintln(stderr, "\nUsage:\n  proton [options] [directory]\n  proton list [options] [directory]\n  proton run [options] <task-id> [directory]\n  proton install\n  proton uninstall\n\nOptions:")
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
	fmt.Fprintf(stdout, "Task: %s\nDirectory: %s\nSource: %s\n", task.ID, task.ProjectDir, task.Source)
	if task.Risk != "" {
		fmt.Fprintf(stdout, "Risk: %s\n", task.Risk)
	}
	if len(task.Required) > 0 {
		fmt.Fprintf(stdout, "Required tools: %s\n", strings.Join(task.Required, ", "))
	}
	fmt.Fprintf(stdout, "Command: %s\n", task.Command)
	if *dryRun {
		return nil
	}
	if (task.Confirm || strings.EqualFold(task.Risk, "high")) && !*yes {
		return fmt.Errorf("task %q requires confirmation; review it with --dry-run and execute with --yes", task.ID)
	}
	if missing := missingRequiredTools(task); len(missing) > 0 {
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

func printCatalog(output io.Writer, tasks []core.Task) {
	if len(tasks) == 0 {
		fmt.Fprintln(output, "No tasks detected.")
		return
	}
	currentProject := ""
	for _, task := range tasks {
		if task.Project != currentProject {
			if currentProject != "" {
				fmt.Fprintln(output)
			}
			fmt.Fprintf(output, "%s [%s]\n", task.Project, strings.Join(task.Types, ", "))
			currentProject = task.Project
		}
		risk := ""
		if task.Risk != "" {
			risk = " [" + task.Risk + "]"
		}
		fmt.Fprintf(output, "  %-36s %s%s\n", task.ID, task.Name, risk)
		fmt.Fprintf(output, "    $ %s\n", task.Command)
		if task.Source != "" {
			fmt.Fprintf(output, "    source: %s\n", task.Source)
		}
	}
}

func missingRequiredTools(task core.Task) []string {
	var missing []string
	for _, status := range envcheck.CheckEnvironment(task.Checks) {
		if status.Required && !status.OK {
			missing = append(missing, status.Name)
		}
	}
	return missing
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
