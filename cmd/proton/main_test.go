package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldInstall(t *testing.T) {
	original := os.Args
	defer func() { os.Args = original }()

	os.Args = []string{"proton-installer-windows-amd64.exe"}
	if !shouldInstall() {
		t.Fatal("installer-named executable should install when run without arguments")
	}
	os.Args = []string{"proton", "install"}
	if !shouldInstall() {
		t.Fatal("install subcommand should trigger installation")
	}
	os.Args = []string{"proton"}
	if shouldInstall() {
		t.Fatal("installed proton command should launch normally")
	}
}

func TestRunListJSON(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	manifest := `{"name":"sample","scripts":{"test":"node test.js","zeta":"echo z","alpha":"echo a"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := runList([]string{"--json", "--no-recursive", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("runList() error = %v, stderr = %s", err, stderr.String())
	}
	var output catalogOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if output.SchemaVersion != 1 || output.Root != dir {
		t.Fatalf("catalog header = %+v", output)
	}
	if len(output.Tasks) != 4 {
		t.Fatalf("tasks = %d, want 4: %+v", len(output.Tasks), output.Tasks)
	}
	if output.Tasks[1].ID != "root:run-tests" || output.Tasks[1].Source != "package.json" {
		t.Fatalf("test task = %+v", output.Tasks[1])
	}
	if output.Tasks[2].Name != "alpha" || output.Tasks[3].Name != "zeta" {
		t.Fatalf("custom scripts are not deterministic: %+v", output.Tasks)
	}
}

func TestRunTaskDryRun(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/sample\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runTask([]string{"--dry-run", "--no-recursive", "root:run-tests", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runTask() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Command: go test ./...") {
		t.Fatalf("dry-run output = %q", stdout.String())
	}
}

func TestRunTaskRequiresApprovalForRiskyCommand(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "risky.yaml")
	pluginData := "name: risky\nversion: 1.0\ndetect:\n  files: [risky.marker]\ncommands:\n  - name: Deploy production\n    run: deploy-now\n    risk: high\n"
	if err := os.WriteFile(pluginPath, []byte(pluginData), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "risky.marker"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".proton.yaml"), []byte("plugins:\n  - risky.yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := runTask([]string{"--no-recursive", "root:deploy-production", dir}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("risk confirmation error = %v", err)
	}
}

func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}
