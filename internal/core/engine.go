package core

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fupeijun/proton/internal/config"
	"github.com/fupeijun/proton/internal/detector"
	"github.com/fupeijun/proton/internal/plugin"
	"github.com/fupeijun/proton/internal/tui"
)

type Engine struct {
	plugins []*plugin.Plugin
}

type Options struct {
	Dir   string
	Depth int
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Run() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	return e.RunWithOptions(Options{Dir: dir, Depth: 3})
}

func (e *Engine) RunWithOptions(options Options) error {
	// Load config
	cfg := config.LoadConfig()

	// Load plugins
	plugins, err := plugin.LoadPlugins(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: some plugins could not be loaded:\n%v\n", err)
		if len(plugins) == 0 {
			return fmt.Errorf("no usable plugins: %w", err)
		}
	}
	e.plugins = plugins

	dir := options.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving scan directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("opening scan directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("scan path is not a directory: %s", dir)
	}

	// Detect projects
	projects := detector.DetectProjectWithDepth(dir, e.plugins, options.Depth)
	if len(projects) == 0 {
		println("No projects detected in current directory.")
		return nil
	}

	// Save terminal state before TUI starts
	SaveTerminalState()
	defer RestoreTerminalState()

	// Run TUI
	m := tui.NewModel(projects)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	// Check if user selected a command to execute
	if model, ok := finalModel.(tui.Model); ok {
		if cmd, dir, ok := model.GetExecuteCmd(); ok {
			RestoreTerminalState()
			// Execute command in restored terminal and propagate exit code
			exitCode := tui.ExecuteCommand(cmd, dir)
			if exitCode != 0 {
				os.Exit(exitCode)
			}
		}
	}

	return nil
}

func println(s string) {
	os.Stdout.WriteString(s + "\n")
}
