package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fupeijun/proton/internal/detector"
	"github.com/fupeijun/proton/internal/plugin"
)

func TestIntegrationPythonProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte("flask"), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) == 0 {
		t.Fatal("No projects detected")
	}

	found := false
	for _, d := range detected {
		if d.Plugin.Name == "python" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Python project not detected")
	}
}

func TestIntegrationNodeProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name": "test"}`), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) == 0 {
		t.Fatal("No projects detected")
	}

	found := false
	for _, d := range detected {
		if d.Plugin.Name == "nodejs" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Node.js project not detected")
	}
}

func TestIntegrationGoProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) == 0 {
		t.Fatal("No projects detected")
	}

	found := false
	for _, d := range detected {
		if d.Plugin.Name == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Go project not detected")
	}
}

func TestIntegrationRustProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\""), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) == 0 {
		t.Fatal("No projects detected")
	}

	found := false
	for _, d := range detected {
		if d.Plugin.Name == "rust" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Rust project not detected")
	}
}

func TestIntegrationJavaProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "pom.xml"), []byte("<project></project>"), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) == 0 {
		t.Fatal("No projects detected")
	}

	found := false
	for _, d := range detected {
		if d.Plugin.Name == "java-maven" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Java project not detected")
	}
}

func TestIntegrationCppProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.0)"), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) == 0 {
		t.Fatal("No projects detected")
	}

	found := false
	for _, d := range detected {
		if d.Plugin.Name == "cpp" {
			found = true
			break
		}
	}
	if !found {
		t.Error("C++ project not detected")
	}
}

func TestIntegrationMixedProject(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)

	plugins, _ := plugin.LoadPlugins(nil)
	detected := detector.DetectProject(tmpDir, plugins)

	if len(detected) != 1 {
		t.Fatalf("Expected one grouped project, got %d", len(detected))
	}
	if len(detected[0].Types) != 3 {
		t.Errorf("Expected 3 detected technologies, got %v", detected[0].Types)
	}
}

func TestIntegrationCommandsAvailable(t *testing.T) {
	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatalf("Failed to load plugins: %v", err)
	}

	for _, p := range plugins {
		if len(p.Commands) == 0 {
			t.Errorf("Plugin %q has no commands", p.Name)
		}
	}
}

func TestIntegrationMakefileAloneIsNotCpp(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte("test:\n\t@echo ok"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, detected := range detector.DetectProject(tmpDir, plugins) {
		if detected.Plugin.Name == "cpp" {
			t.Fatal("a generic Makefile must not be detected as a C++ project")
		}
	}
}

func TestIntegrationGradleProject(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "build.gradle"), []byte("plugins {}"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, detected := range detector.DetectProject(tmpDir, plugins) {
		if detected.Plugin.Name == "java-gradle" {
			return
		}
	}
	t.Fatal("Gradle project not detected")
}
