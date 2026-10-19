package tui

import (
	"os"
	"path/filepath"
	"strings"
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

func TestModelViewUsesChineseLabels(t *testing.T) {
	m := NewModel([]*detector.DetectedProject{{
		Plugin: &plugin.Plugin{
			Name:     "go",
			Commands: []plugin.Command{{Name: "Run", Run: "go run ."}},
			Env:      []plugin.EnvCheck{{Name: "go"}},
		},
		Types: []string{"go"},
		Dir:   t.TempDir(),
	}})
	m.envChecking = false
	m.envStatus = []envcheck.EnvStatus{{Name: "go", OK: true, Version: "go version go1.22.1"}}

	view := m.View()
	for _, want := range []string{"项目", "命令", "环境", "执行命令", "Proton 项目启动器"} {
		if !strings.Contains(view, want) {
			t.Errorf("View() missing %q in output:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Commands") || strings.Contains(view, "Environment") {
		t.Errorf("View() still contains English labels:\n%s", view)
	}
}

func TestModelViewSwitchesPanels(t *testing.T) {
	m := NewModel([]*detector.DetectedProject{{
		Plugin: &plugin.Plugin{
			Name:     "go",
			Commands: []plugin.Command{{Name: "Run", Run: "go run ."}},
			Env:      []plugin.EnvCheck{{Name: "go"}},
		},
		Types: []string{"go"},
		Dir:   t.TempDir(),
	}})
	m.envChecking = false
	m.envStatus = []envcheck.EnvStatus{{Name: "go", OK: true}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.panel != envPanel {
		t.Fatalf("panel = %d, want envPanel", m.panel)
	}
	if !strings.Contains(m.View(), "✓ go") {
		t.Fatalf("environment row not visible after switching panel:\n%s", m.View())
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"abc", 5, "abc"},
		{"abcdefgh", 5, "abcd…"},
		{"中文字符串很长", 9, "中文字符…"},
		{"", 3, ""},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.max); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

func TestNeedsPowerShell(t *testing.T) {
	// Commands that should use cmd.exe (no PowerShell needed)
	for _, cmd := range []string{
		"npm run start",
		"npm test",
		"go build ./...",
		"python main.py",
		"cargo build",
		"make all",
		"echo hello world",
	} {
		if needsPowerShell(cmd) {
			t.Errorf("needsPowerShell(%q) = true, want false", cmd)
		}
	}
	// Commands that should use PowerShell
	for _, cmd := range []string{
		"Get-Content file.txt",
		"echo $HOME",
		"$_ = 'test'",
		"Invoke-WebRequest url",
		"where-Object { $_.Name -eq 'foo' }",
		"@('a','b') | ForEach-Object { $_ }",
	} {
		if !needsPowerShell(cmd) {
			t.Errorf("needsPowerShell(%q) = false, want true", cmd)
		}
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
