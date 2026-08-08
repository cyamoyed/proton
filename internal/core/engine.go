package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyamoyed/proton/internal/config"
	"github.com/cyamoyed/proton/internal/detector"
	"github.com/cyamoyed/proton/internal/plugin"
	"github.com/cyamoyed/proton/internal/tui"
)

type Engine struct {
	plugins []*plugin.Plugin
}

type Options struct {
	Dir   string
	Depth int
}

// Discovery is the reusable result of scanning a directory. Warning contains
// non-fatal plugin loading errors when at least one usable plugin remains.
type Discovery struct {
	Root     string
	Projects []*detector.DetectedProject
	Warning  error
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
	discovery, err := e.Discover(options)
	if err != nil {
		return err
	}
	if discovery.Warning != nil {
		fmt.Fprintf(os.Stderr, "Warning: some plugins could not be loaded:\n%v\n", discovery.Warning)
	}
	projects := discovery.Projects
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

// Discover detects projects without starting the interactive interface. It is
// used by both the TUI and non-interactive CLI commands.
func (e *Engine) Discover(options Options) (Discovery, error) {
	dir := options.Dir
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return Discovery{}, err
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return Discovery{}, fmt.Errorf("resolving scan directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Discovery{}, fmt.Errorf("opening scan directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return Discovery{}, fmt.Errorf("scan path is not a directory: %s", dir)
	}

	cfg, configErr := config.LoadConfig(dir)
	plugins, loadErr := plugin.LoadPlugins(cfg)
	if len(plugins) == 0 {
		if loadErr != nil {
			return Discovery{}, fmt.Errorf("no usable plugins: %w", loadErr)
		}
		return Discovery{}, fmt.Errorf("no usable plugins")
	}
	e.plugins = plugins

	return Discovery{
		Root:     dir,
		Projects: detector.DetectProjectWithDepth(dir, e.plugins, options.Depth),
		Warning:  errors.Join(configErr, loadErr),
	}, nil
}

func println(s string) {
	os.Stdout.WriteString(s + "\n")
}
