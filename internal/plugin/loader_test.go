package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fupeijun/proton/internal/config"
)

func TestLoadPlugins(t *testing.T) {
	plugins, err := LoadPlugins(nil)
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}
	if len(plugins) == 0 {
		t.Fatal("LoadPlugins() returned no plugins")
	}

	// Check that python plugin exists
	found := false
	for _, p := range plugins {
		if p.Name == "python" {
			found = true
			if len(p.Commands) == 0 {
				t.Error("python plugin has no commands")
			}
			break
		}
	}
	if !found {
		t.Error("python plugin not found")
	}
}

func TestLoadPluginsCount(t *testing.T) {
	plugins, err := LoadPlugins(nil)
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}

	expectedCount := 7
	if len(plugins) != expectedCount {
		t.Errorf("LoadPlugins() returned %d plugins, want %d", len(plugins), expectedCount)
	}
}

func TestPluginStructure(t *testing.T) {
	plugins, err := LoadPlugins(nil)
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}

	for _, p := range plugins {
		if p.Name == "" {
			t.Error("plugin has empty name")
		}
		if p.Version == "" {
			t.Errorf("plugin %q has empty version", p.Name)
		}
		if len(p.Detect.Files) == 0 && len(p.Detect.Dirs) == 0 {
			t.Errorf("plugin %q has no detect files or dirs", p.Name)
		}
		if len(p.Commands) == 0 {
			t.Errorf("plugin %q has no commands", p.Name)
		}
	}
}

func TestLoadPluginsReportsInvalidConfiguredPlugin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.yaml")
	if err := os.WriteFile(path, []byte("name: [invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := LoadPlugins(&config.Config{Plugins: []string{path}})
	if err == nil {
		t.Fatal("LoadPlugins() error = nil, want malformed plugin error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("LoadPlugins() error = %q, want plugin path", err)
	}
	if len(plugins) == 0 {
		t.Fatal("valid builtin plugins should remain available")
	}
}
