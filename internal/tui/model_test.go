package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cyamoyed/proton/internal/detector"
	"github.com/cyamoyed/proton/internal/envcheck"
	"github.com/cyamoyed/proton/internal/plugin"
)

func TestModelIgnoresStaleEnvironmentResult(t *testing.T) {
	projects := []*detector.DetectedProject{
		{Plugin: &plugin.Plugin{Name: "first"}, Dir: "first-dir"},
		{Plugin: &plugin.Plugin{Name: "second"}, Dir: "second-dir"},
	}
	m := NewModel(projects)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.envRequest != 2 {
		t.Fatalf("environment request = %d, want 2", m.envRequest)
	}

	updated, _ = m.Update(envCheckResultMsg{
		request:  1,
		statuses: []envcheck.EnvStatus{{Name: "stale", OK: true}},
	})
	m = updated.(Model)
	if !m.envChecking || len(m.envStatus) != 0 {
		t.Fatalf("stale result changed current environment state: %+v", m.envStatus)
	}

	updated, _ = m.Update(envCheckResultMsg{
		request:  2,
		statuses: []envcheck.EnvStatus{{Name: "current", OK: true}},
	})
	m = updated.(Model)
	if m.envChecking || len(m.envStatus) != 1 || m.envStatus[0].Name != "current" {
		t.Fatalf("current result was not applied: %+v", m.envStatus)
	}
}

func TestModelReturnsSelectedProjectDirectory(t *testing.T) {
	projectDir := t.TempDir()
	projects := []*detector.DetectedProject{{
		Plugin: &plugin.Plugin{Commands: []plugin.Command{{Name: "test", Run: "go version"}}},
		Dir:    projectDir,
	}}
	m := NewModel(projects)
	m.envChecking = false
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	command, dir, ok := m.GetExecuteCmd()
	if !ok || command != "go version" || dir != projectDir {
		t.Fatalf("selection = (%q, %q, %v), want command and project directory", command, dir, ok)
	}
}

func TestModelBlocksCommandWhenRequiredEnvironmentMissing(t *testing.T) {
	m := NewModel([]*detector.DetectedProject{{
		Plugin: &plugin.Plugin{Commands: []plugin.Command{{Name: "test", Run: "go test"}}},
		Dir:    t.TempDir(),
	}})
	m.envChecking = false
	m.envStatus = []envcheck.EnvStatus{{Name: "go", Required: true, OK: false}}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || m.executeOnExit || m.notice == "" {
		t.Fatalf("missing required tool did not block execution: %+v", m)
	}
}

func TestModelUsesCommandSpecificEnvironmentRequirements(t *testing.T) {
	m := NewModel([]*detector.DetectedProject{{
		Plugin: &plugin.Plugin{
			Commands: []plugin.Command{{
				Name:   "build",
				Run:    "just build",
				Checks: []plugin.EnvCheck{{Name: "just", Required: true}},
			}},
		},
		Dir: t.TempDir(),
	}})
	m.envChecking = false
	m.envStatus = []envcheck.EnvStatus{
		{Name: "just", Required: true, OK: true},
		{Name: "task", Required: true, OK: false},
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !m.executeOnExit || m.executeCmd != "just build" {
		t.Fatalf("unrelated missing tool blocked command: %+v", m)
	}
}

func TestModelConfirmsRiskyCommand(t *testing.T) {
	m := NewModel([]*detector.DetectedProject{{
		Plugin: &plugin.Plugin{Commands: []plugin.Command{{Name: "deploy", Run: "deploy", Risk: "high"}}},
		Dir:    t.TempDir(),
	}})
	m.envChecking = false
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil || !m.confirming || m.executeOnExit {
		t.Fatalf("risky command skipped confirmation: %+v", m)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if !m.executeOnExit || m.executeCmd != "deploy" {
		t.Fatalf("confirmed command was not selected: %+v", m)
	}
}

func TestExecuteCommandUsesProjectDirectory(t *testing.T) {
	projectDir := t.TempDir()
	marker := "proton-command-cwd-test"
	if code := ExecuteCommand("go env GOMOD > "+marker, projectDir); code != 0 {
		t.Fatalf("ExecuteCommand() exit code = %d", code)
	}
	if _, err := os.Stat(filepath.Join(projectDir, marker)); err != nil {
		t.Fatalf("command did not run in project directory: %v", err)
	}
}
