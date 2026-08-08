package core

import (
	"path/filepath"
	"testing"

	"github.com/cyamoyed/proton/internal/detector"
	"github.com/cyamoyed/proton/internal/plugin"
)

func TestCatalogBuildsDeterministicTaskIDs(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "services", "api")
	discovery := Discovery{
		Root: root,
		Projects: []*detector.DetectedProject{{
			Dir:        projectDir,
			Types:      []string{"go"},
			Confidence: 10,
			Plugin: &plugin.Plugin{
				Source: "plugins/go.yaml",
				Commands: []plugin.Command{
					{Name: "Run tests", Run: "go test ./..."},
					{Name: "Run tests", Run: "go test -race ./..."},
				},
			},
		}},
	}

	tasks := Catalog(discovery)
	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(tasks))
	}
	if tasks[0].ID != "services/api:run-tests" || tasks[1].ID != "services/api:run-tests-2" {
		t.Fatalf("task IDs = %q, %q", tasks[0].ID, tasks[1].ID)
	}
	if tasks[0].Source != "plugins/go.yaml" {
		t.Fatalf("source = %q", tasks[0].Source)
	}
}

func TestFindTaskRequiresExactID(t *testing.T) {
	tasks := []Task{{ID: "root:build", Name: "Build"}}
	if task, ok := FindTask(tasks, "root:build"); !ok || task.Name != "Build" {
		t.Fatalf("exact task was not found: %+v, %v", task, ok)
	}
	if _, ok := FindTask(tasks, "build"); ok {
		t.Fatal("partial task ID must not match")
	}
}

func TestCatalogUsesCommandSpecificEnvironmentChecks(t *testing.T) {
	root := t.TempDir()
	discovery := Discovery{
		Root: root,
		Projects: []*detector.DetectedProject{{
			Dir:        root,
			Types:      []string{"just", "taskfile"},
			Confidence: 20,
			Plugin: &plugin.Plugin{
				Env: []plugin.EnvCheck{
					{Name: "just", Required: true},
					{Name: "task", Required: true},
				},
				Commands: []plugin.Command{{
					Name:       "build",
					Run:        "just build",
					Checks:     []plugin.EnvCheck{{Name: "just", Required: true}},
					Confidence: 10,
				}},
			},
		}},
	}
	tasks := Catalog(discovery)
	if len(tasks) != 1 || len(tasks[0].Required) != 1 || tasks[0].Required[0] != "just" {
		t.Fatalf("task requirements = %+v", tasks)
	}
	if tasks[0].Confidence != 10 {
		t.Fatalf("task confidence = %d, want command-specific score 10", tasks[0].Confidence)
	}
}
