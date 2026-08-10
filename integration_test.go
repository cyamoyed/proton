package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyamoyed/proton/internal/detector"
	"github.com/cyamoyed/proton/internal/plugin"
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

func TestIntegrationSwiftUIIOSProject(t *testing.T) {
	tmpDir := t.TempDir()
	project := filepath.Join(tmpDir, "Example.xcodeproj")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte("SDKROOT = iphoneos;\ncom.apple.product-type.application"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "ExampleApp.swift"), []byte("import SwiftUI\n@main struct ExampleApp: App {}"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatal(err)
	}
	detected := detector.DetectProjectWithDepth(tmpDir, plugins, 0)
	if len(detected) != 1 || detected[0].Plugin.Name != "swiftui-ios" {
		t.Fatalf("SwiftUI iOS project detection = %+v", detected)
	}
	if len(detected[0].Plugin.Commands) < 3 {
		t.Fatalf("SwiftUI iOS commands = %+v", detected[0].Plugin.Commands)
	}
}

func TestIntegrationUIKitProjectIsNotSwiftUI(t *testing.T) {
	tmpDir := t.TempDir()
	project := filepath.Join(tmpDir, "UIKitApp.xcodeproj")
	if err := os.Mkdir(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte("IPHONEOS_DEPLOYMENT_TARGET = 18.0;"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "AppDelegate.swift"), []byte("import UIKit"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, detected := range detector.DetectProjectWithDepth(tmpDir, plugins, 0) {
		if detected.Plugin.Name == "swiftui-ios" {
			t.Fatal("UIKit-only project must not be detected as SwiftUI")
		}
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

func TestIntegrationDeclaredTaskFiles(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		contents   string
		pluginName string
		command    string
	}{
		{name: "just", file: "justfile", contents: "check:\n\tgo test ./...\n", pluginName: "just", command: "check"},
		{name: "taskfile", file: "Taskfile.yml", contents: "version: '3'\ntasks:\n  check:\n    cmds: [go test ./...]\n", pluginName: "taskfile", command: "check"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, test.file), []byte(test.contents), 0644); err != nil {
				t.Fatal(err)
			}
			plugins, err := plugin.LoadPlugins(nil)
			if err != nil {
				t.Fatal(err)
			}
			detected := detector.DetectProjectWithDepth(dir, plugins, 0)
			if len(detected) != 1 || detected[0].Plugin.Name != test.pluginName {
				t.Fatalf("detected = %+v", detected)
			}
			if len(detected[0].Plugin.Commands) != 1 || detected[0].Plugin.Commands[0].Name != test.command {
				t.Fatalf("commands = %+v", detected[0].Plugin.Commands)
			}
		})
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

func TestIntegrationMavenProjectCommandMatrix(t *testing.T) {
	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		dir         string
		wantCommand string
	}{
		{name: "jar", dir: filepath.Join("testdata", "maven", "simple-jar"), wantCommand: "打包 JAR"},
		{name: "war", dir: filepath.Join("testdata", "maven", "web-war"), wantCommand: "打包 WAR"},
		{name: "reactor", dir: filepath.Join("testdata", "maven", "multi-module"), wantCommand: "构建全部模块"},
		{name: "executable module", dir: filepath.Join("testdata", "maven", "multi-module", "application"), wantCommand: "运行应用"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := detector.DetectProjectWithDepth(test.dir, plugins, 0)
			if len(projects) != 1 || projects[0].Plugin.Name != "java-maven" {
				t.Fatalf("Maven project detection = %+v", projects)
			}
			for _, command := range projects[0].Plugin.Commands {
				if command.Name == test.wantCommand {
					return
				}
			}
			t.Fatalf("commands %+v do not include %q", projects[0].Plugin.Commands, test.wantCommand)
		})
	}
}

func TestIntegrationMavenUsesResolvedFrameworkType(t *testing.T) {
	dir := t.TempDir()
	pom := `<project><parent><artifactId>platform</artifactId></parent><artifactId>orders</artifactId>` +
		`<dependencies><dependency><groupId>org.springframework.cloud</groupId><artifactId>spring-cloud-starter-openfeign</artifactId></dependency></dependencies>` +
		`<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
	plugins, err := plugin.LoadPlugins(nil)
	if err != nil {
		t.Fatal(err)
	}
	projects := detector.DetectProjectWithDepth(dir, plugins, 0)
	if len(projects) != 1 || len(projects[0].Types) != 1 || projects[0].Types[0] != "spring-cloud-maven" {
		t.Fatalf("resolved project types = %+v", projects)
	}
}
