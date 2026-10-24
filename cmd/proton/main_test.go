package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyamoyed/proton/internal/core"
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

func TestRunMCPRejectsUnknownFlagsAndArguments(t *testing.T) {
	var stderr bytes.Buffer
	if err := runMCP([]string{"--bogus"}, &stderr); err == nil {
		t.Error("unknown flag should fail fast without starting the server")
	}
	if err := runMCP([]string{"extra"}, &stderr); err == nil {
		t.Error("positional argument should fail fast without starting the server")
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
	if output.Tasks[1].ID != "root:运行测试" || output.Tasks[1].Source != "package.json" {
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
	err := runTask([]string{"--dry-run", "--no-recursive", "root:运行测试", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runTask() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "命令: go test ./...") {
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

func TestPrintCatalogGroupsTasksByProject(t *testing.T) {
	tasks := []core.Task{
		{ID: "root:运行", Project: ".", Types: []string{"go"}, Name: "运行", Command: "go run .", Source: "go.yaml"},
		{ID: "root:运行测试", Project: ".", Types: []string{"go"}, Name: "运行测试", Command: "go test ./...", Risk: "high"},
		{ID: "services/api:构建", Project: "services/api", Types: []string{"go"}, Name: "构建", Command: "go build ./...", Description: "编译整个项目"},
		{ID: "services/api:运行", Project: "services/api", Types: []string{"go"}, Name: "运行", Command: "go run ./cmd/api"},
	}
	var out bytes.Buffer
	printCatalog(&out, tasks)
	text := out.String()

	for _, want := range []string{
		"项目 root [go] · 2 个任务",
		"ID 前缀: root:",
		"运行      $ go run .",
		"运行测试  $ go test ./... [高危]",
		"项目 services/api [go] · 2 个任务",
		"ID 前缀: services/api:",
		"构建  $ go build ./...",
		"运行  $ go run ./cmd/api",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("printCatalog output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "描述:") || strings.Contains(text, "来源:") {
		t.Errorf("printCatalog output should not contain descriptions or sources:\n%s", text)
	}
	if strings.Contains(text, "source:") || strings.Contains(text, "No tasks detected") {
		t.Errorf("printCatalog output contains English residue:\n%s", text)
	}
}

func TestPrintCatalogEmpty(t *testing.T) {
	var out bytes.Buffer
	printCatalog(&out, nil)
	if !strings.Contains(out.String(), "未检测到任务") {
		t.Fatalf("empty catalog output = %q", out.String())
	}
}

func TestPrintCatalogHidesModulesWithoutStartupEntry(t *testing.T) {
	tasks := []core.Task{
		// 聚合根只有转发启动命令（-pl）和构建命令 → 隐藏
		{ID: "root:构建全部模块", Project: ".", Types: []string{"spring-cloud-maven"}, Name: "构建全部模块", Command: "mvn package"},
		{ID: "root:运行 Spring Cloud：gateway", Project: ".", Types: []string{"spring-cloud-maven"}, Name: "运行 Spring Cloud：gateway", Command: "mvn -pl gateway spring-boot:run"},
		// 服务模块直接启动 → 保留全部命令
		{ID: "gateway:运行-spring-cloud-服务", Project: "gateway", Types: []string{"spring-cloud-maven"}, Name: "运行 Spring Cloud 服务", Command: "mvn spring-boot:run"},
		{ID: "gateway:打包-jar", Project: "gateway", Types: []string{"spring-cloud-maven"}, Name: "打包 JAR", Command: "mvn package"},
		// 公共库模块无启动入口 → 隐藏
		{ID: "common:编译", Project: "common", Types: []string{"java-maven"}, Name: "编译", Command: "mvn compile"},
	}

	var out bytes.Buffer
	printCatalog(&out, tasks)
	text := out.String()

	for _, want := range []string{
		"项目 gateway [spring-cloud-maven] · 2 个任务",
		"ID 前缀: gateway:",
		"▶ 运行 Spring Cloud 服务",
		"$ mvn spring-boot:run",
		"(ID: 运行-spring-cloud-服务)",
		"打包 JAR",
		"$ mvn package",
		"(ID: 打包-jar)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("printCatalog output missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "▶ 运行 Spring Cloud 服务") > strings.Index(text, "打包 JAR") {
		t.Errorf("startup command must be listed first:\n%s", text)
	}
	for _, unwanted := range []string{"构建全部模块", "运行 Spring Cloud：gateway", "项目 common", "mvn compile"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("printCatalog output should not contain %q:\n%s", unwanted, text)
		}
	}
}

func TestPrintCatalogShowsAllWhenNothingIsStartable(t *testing.T) {
	tasks := []core.Task{
		{ID: "root:构建（Make）", Project: ".", Types: []string{"cpp"}, Name: "构建（Make）", Command: "make"},
		{ID: "root:构建（CMake）", Project: ".", Types: []string{"cpp"}, Name: "构建（CMake）", Command: "cmake -B build && cmake --build build"},
	}
	var out bytes.Buffer
	printCatalog(&out, tasks)
	text := out.String()
	if !strings.Contains(text, "构建（Make）") || !strings.Contains(text, "构建（CMake）") {
		t.Fatalf("non-startable-only scan should fall back to full list:\n%s", text)
	}
}

func TestIsStartupCommand(t *testing.T) {
	cases := []struct {
		run  string
		want bool
	}{
		{"mvn spring-boot:run", true},
		{"mvn quarkus:dev", true},
		{"mvn exec:java", true},
		{".\\mvnw.cmd spring-boot:run", true},
		{"go run .", true},
		{"cargo run", true},
		{"python main.py", true},
		{"gradle run", true},
		{".\\gradlew.bat run", true},
		{"npm run dev", true},
		{"pnpm run start", true},
		{"mvn -pl gateway spring-boot:run", false},
		{".\\mvnw.cmd -P!Cloud -pl gateway spring-boot:run", false},
		{"mvn package", false},
		{"go build .", false},
		{"go test ./...", false},
		{"npm run build", false},
		{"cargo clippy", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isStartupCommand(c.run); got != c.want {
			t.Errorf("isStartupCommand(%q) = %v, want %v", c.run, got, c.want)
		}
	}
}

func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestRunUpdateCheckDetectsNewerRelease(t *testing.T) {
	server := newUpdateTestServer(t, `{"tag_name":"v9.9.9","assets":[]}`, http.StatusOK)
	t.Setenv("PROTON_GITHUB_API", server.URL)
	t.Setenv("PROTON_REPO", "cyamoyed/proton")

	var stdout, stderr bytes.Buffer
	if err := runUpdate([]string{"--check"}, &stdout, &stderr, "v1.0.0"); err != nil {
		t.Fatalf("runUpdate() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "v9.9.9") || !strings.Contains(stdout.String(), "新版本") {
		t.Errorf("output should report the newer release:\n%s", stdout.String())
	}
}

func TestRunUpdateCheckUpToDate(t *testing.T) {
	server := newUpdateTestServer(t, `{"tag_name":"v1.0.0","assets":[]}`, http.StatusOK)
	t.Setenv("PROTON_GITHUB_API", server.URL)
	t.Setenv("PROTON_REPO", "cyamoyed/proton")

	var stdout, stderr bytes.Buffer
	if err := runUpdate([]string{"--check"}, &stdout, &stderr, "v1.0.0"); err != nil {
		t.Fatalf("runUpdate() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "已是最新版本") {
		t.Errorf("output should report up to date:\n%s", stdout.String())
	}
}

func TestRunUpdateUnknownVersionRequiresForce(t *testing.T) {
	server := newUpdateTestServer(t, `{"tag_name":"v9.9.9","assets":[]}`, http.StatusOK)
	t.Setenv("PROTON_GITHUB_API", server.URL)
	t.Setenv("PROTON_REPO", "cyamoyed/proton")

	var stdout, stderr bytes.Buffer
	if err := runUpdate([]string{"--check"}, &stdout, &stderr, "dev"); err != nil {
		t.Fatalf("runUpdate() error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "--force") {
		t.Errorf("output should point at --force:\n%s", stdout.String())
	}
}

func TestRunUpdatePrivateRepoHint(t *testing.T) {
	server := newUpdateTestServer(t, `{}`, http.StatusNotFound)
	t.Setenv("PROTON_GITHUB_API", server.URL)
	t.Setenv("PROTON_REPO", "cyamoyed/proton")

	var stdout, stderr bytes.Buffer
	err := runUpdate([]string{"--check"}, &stdout, &stderr, "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "令牌") {
		t.Fatalf("runUpdate() error = %v; want a hint about the access token", err)
	}
}

func TestRunUpdateRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := runUpdate([]string{"extra"}, &stdout, &stderr, "v1.0.0"); err == nil {
		t.Error("positional argument should fail fast")
	}
}

func newUpdateTestServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/cyamoyed/proton/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}
