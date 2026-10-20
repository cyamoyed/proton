package detector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyamoyed/proton/internal/plugin"
)

func TestDetectProject(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	plugins := []*plugin.Plugin{
		{
			Name: "nodejs",
			Detect: plugin.Detect{
				Files: []string{"package.json"},
			},
		},
		{
			Name: "python",
			Detect: plugin.Detect{
				Files: []string{"requirements.txt"},
			},
		},
	}

	detected := DetectProject(tmpDir, plugins)
	if len(detected) != 1 {
		t.Fatalf("expected 1 detected project, got %d", len(detected))
	}
	if detected[0].Plugin.Name != "nodejs" {
		t.Errorf("expected nodejs, got %s", detected[0].Plugin.Name)
	}
}

func TestDetectMultipleProjects(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "requirements.txt"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	plugins := []*plugin.Plugin{
		{Name: "nodejs", Detect: plugin.Detect{Files: []string{"package.json"}}},
		{Name: "python", Detect: plugin.Detect{Files: []string{"requirements.txt"}}},
	}

	detected := DetectProject(tmpDir, plugins)
	if len(detected) != 1 {
		t.Fatalf("expected technologies in one grouped project, got %d", len(detected))
	}
	if len(detected[0].Types) != 2 {
		t.Fatalf("expected 2 detected technologies, got %v", detected[0].Types)
	}
}

func TestDetectProjectWithDepth(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module nested"), 0644); err != nil {
		t.Fatal(err)
	}
	plugins := []*plugin.Plugin{{Name: "go", Detect: plugin.Detect{Files: []string{"go.mod"}}}}
	if got := DetectProjectWithDepth(root, plugins, 0); len(got) != 0 {
		t.Fatalf("depth zero detected nested project: %+v", got)
	}
	if got := DetectProjectWithDepth(root, plugins, 1); len(got) != 1 {
		t.Fatalf("depth one detected %d projects, want 1", len(got))
	}
}

func TestDetectProjectMatchesWildcardDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Example.xcodeproj"), 0755); err != nil {
		t.Fatal(err)
	}
	plugins := []*plugin.Plugin{{
		Name:   "xcode",
		Detect: plugin.Detect{Dirs: []string{"*.xcodeproj"}},
	}}

	detected := DetectProjectWithDepth(root, plugins, 0)
	if len(detected) != 1 || detected[0].Plugin.Name != "xcode" {
		t.Fatalf("wildcard directory detection = %+v, want xcode project", detected)
	}
}

func TestDetectProjectDoesNotDoubleCountCaseVariants(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "project.yml"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	plugins := []*plugin.Plugin{{
		Name:   "task-source",
		Detect: plugin.Detect{Files: []string{"project.yml", "Project.yml"}},
	}}
	detected := DetectProjectWithDepth(root, plugins, 0)
	if len(detected) != 1 || detected[0].Confidence != 10 {
		t.Fatalf("detected = %+v, want one physical match worth 10", detected)
	}
}
