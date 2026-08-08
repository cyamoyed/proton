package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigUsesTargetDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	target := filepath.Join(t.TempDir(), "target")
	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, ".proton.yaml"), []byte("plugins:\n  - plugins/custom.yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(other); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })

	cfg, err := LoadConfig(target)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(target, "plugins", "custom.yaml")
	if len(cfg.Plugins) != 1 || cfg.Plugins[0] != want {
		t.Fatalf("plugins = %v, want [%s]", cfg.Plugins, want)
	}
}

func TestLoadConfigResolvesGlobalPluginsFromConfigDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	configDir := filepath.Join(home, ".proton")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("plugins:\n  - plugins/global.yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(configDir, "plugins", "global.yaml")
	if len(cfg.Plugins) != 1 || cfg.Plugins[0] != want {
		t.Fatalf("plugins = %v, want [%s]", cfg.Plugins, want)
	}
}

func TestLoadConfigReportsMalformedProjectConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, ".proton.yaml"), []byte("plugins: [broken"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(project)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want malformed YAML error")
	}
	if len(cfg.Plugins) != 0 {
		t.Fatalf("plugins = %v", cfg.Plugins)
	}
}
