package plugin

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveNodeUsesManifestScriptsAndPackageManager(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"scripts":{"dev":"vite","custom":"echo ok"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	source := &Plugin{Name: "nodejs", Commands: []Command{{Name: "Build", Run: "npm run build"}}, Env: []EnvCheck{{Name: "npm", Check: "npm --version"}}}
	resolved := ResolveForDir(source, dir)
	joined := ""
	for _, command := range resolved.Commands {
		joined += command.Run + "\n"
	}
	if strings.Contains(joined, "build") || !strings.Contains(joined, "pnpm run dev") || !strings.Contains(joined, "pnpm run custom") {
		t.Fatalf("unexpected resolved commands:\n%s", joined)
	}
	if resolved.Env[0].Name != "pnpm" {
		t.Fatalf("environment = %+v, want pnpm", resolved.Env)
	}
}

func TestResolveCppMatchesBuildSystem(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "meson.build"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveForDir(&Plugin{Name: "cpp", Commands: []Command{{Name: "Build", Run: "make"}}}, dir)
	if len(resolved.Commands) != 1 || !strings.Contains(resolved.Commands[0].Run, "meson") {
		t.Fatalf("commands = %+v, want Meson only", resolved.Commands)
	}
}

func TestResolveJustUsesDeclaredRecipes(t *testing.T) {
	dir := t.TempDir()
	contents := `set shell := ["bash", "-cu"]

# Compile the application
build target="all":
	go build ./...

[private]
helper:
	echo hidden

test: build
	go test ./...
`
	if err := os.WriteFile(filepath.Join(dir, "justfile"), []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveForDir(&Plugin{Name: "just", Commands: []Command{{Name: "List", Run: "just --list"}}}, dir)
	if len(resolved.Commands) != 2 {
		t.Fatalf("commands = %+v, want build and test", resolved.Commands)
	}
	if resolved.Commands[0].Name != "build" || resolved.Commands[0].Description != "Compile the application" || resolved.Commands[0].Source != "justfile" {
		t.Fatalf("build command = %+v", resolved.Commands[0])
	}
	if resolved.Commands[1].Name != "test" || resolved.Commands[1].Run != "just 'test'" {
		t.Fatalf("test command = %+v", resolved.Commands[1])
	}
}

func TestResolveTaskfileUsesSortedPublicTasks(t *testing.T) {
	dir := t.TempDir()
	contents := `version: '3'
tasks:
  test:
    desc: Run the tests
    cmds: [go test ./...]
  internal-helper:
    internal: true
    cmds: [echo hidden]
  build:
    cmds: [go build ./...]
`
	if err := os.WriteFile(filepath.Join(dir, "Taskfile.yml"), []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveForDir(&Plugin{Name: "taskfile", Commands: []Command{{Name: "List", Run: "task --list"}}}, dir)
	if len(resolved.Commands) != 2 {
		t.Fatalf("commands = %+v, want two public tasks", resolved.Commands)
	}
	if resolved.Commands[0].Name != "build" || resolved.Commands[1].Name != "test" {
		t.Fatalf("commands are not sorted: %+v", resolved.Commands)
	}
	if resolved.Commands[1].Description != "Run the tests" || resolved.Commands[1].Source != "Taskfile.yml" {
		t.Fatalf("test command = %+v", resolved.Commands[1])
	}
}

func TestMatchesDirRequiresSwiftUIAndIOS(t *testing.T) {
	plugin := &Plugin{Name: "swiftui-ios"}

	iosSwiftUI := t.TempDir()
	writeTestXcodeProject(t, iosSwiftUI, "Sample", "SDKROOT = iphoneos;")
	if err := os.WriteFile(filepath.Join(iosSwiftUI, "SampleApp.swift"), []byte("import SwiftUI\n@main struct SampleApp: App {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if !MatchesDir(plugin, iosSwiftUI) {
		t.Fatal("SwiftUI iOS project was not matched")
	}

	uiKit := t.TempDir()
	writeTestXcodeProject(t, uiKit, "UIKitApp", "IPHONEOS_DEPLOYMENT_TARGET = 18.0;")
	if err := os.WriteFile(filepath.Join(uiKit, "AppDelegate.swift"), []byte("import UIKit"), 0644); err != nil {
		t.Fatal(err)
	}
	if MatchesDir(plugin, uiKit) {
		t.Fatal("UIKit-only project was matched as SwiftUI")
	}

	macOS := t.TempDir()
	writeTestXcodeProject(t, macOS, "MacApp", "SDKROOT = macosx;")
	if err := os.WriteFile(filepath.Join(macOS, "MacApp.swift"), []byte("import SwiftUI"), 0644); err != nil {
		t.Fatal(err)
	}
	if MatchesDir(plugin, macOS) {
		t.Fatal("macOS-only SwiftUI project was matched as iOS")
	}
}

func TestResolveSwiftUIIOSUsesWorkspaceSchemeAndAvailableSimulator(t *testing.T) {
	dir := t.TempDir()
	project := writeTestXcodeProject(t, dir, "Sample App", "IPHONEOS_DEPLOYMENT_TARGET = 18.0;\ncom.apple.product-type.application\ncom.apple.product-type.bundle.unit-test")
	if err := os.WriteFile(filepath.Join(dir, "SampleApp.swift"), []byte("import SwiftUI"), 0644); err != nil {
		t.Fatal(err)
	}
	schemeDir := filepath.Join(project, "xcshareddata", "xcschemes")
	if err := os.MkdirAll(schemeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemeDir, "Sample App.xcscheme"), []byte("<Scheme><LaunchAction><BuildableProductRunnable/></LaunchAction><TestAction><TestableReference/></TestAction></Scheme>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Sample App.xcworkspace"), 0755); err != nil {
		t.Fatal(err)
	}

	resolved := ResolveForDir(&Plugin{Name: "swiftui-ios", Commands: []Command{{Name: "placeholder"}}}, dir)
	if !hasCommandRun(resolved.Commands, "open 'Sample App.xcworkspace'") {
		t.Fatalf("commands = %+v, want workspace open command", resolved.Commands)
	}
	build := "xcodebuild -workspace 'Sample App.xcworkspace' -scheme 'Sample App' -configuration Debug -destination 'generic/platform=iOS Simulator' build"
	if !hasCommandRun(resolved.Commands, build) {
		t.Fatalf("commands = %+v, want %q", resolved.Commands, build)
	}
	run := commandRunByName(resolved.Commands, "在 iOS 模拟器中运行")
	for _, part := range []string{
		`xcrun simctl boot "$simulator_id"`,
		"open -a Simulator",
		`xcrun simctl bootstatus "$simulator_id" -b`,
		`build_settings="$(xcodebuild -workspace 'Sample App.xcworkspace'`,
		`xcrun simctl install "$simulator_id" "$app_path"`,
		`xcrun simctl launch "$simulator_id" "$bundle_id"`,
	} {
		if !strings.Contains(run, part) {
			t.Errorf("run command = %q, want part %q", run, part)
		}
	}
	if runtime.GOOS != "windows" {
		if output, err := exec.Command("/bin/sh", "-n", "-c", run).CombinedOutput(); err != nil {
			t.Fatalf("run command has invalid shell syntax: %v\n%s\n%s", err, output, run)
		}
	}
	testRun := commandRunByName(resolved.Commands, "运行测试")
	if !strings.Contains(testRun, "xcrun simctl list devices available") || !strings.Contains(testRun, `-destination "platform=iOS Simulator,id=$simulator_id" test`) {
		t.Fatalf("test command = %q, want dynamic available simulator", testRun)
	}
}

func TestResolveSwiftUIIOSOmitsTestsWithoutTestTarget(t *testing.T) {
	dir := t.TempDir()
	writeTestXcodeProject(t, dir, "Demo", "SUPPORTED_PLATFORMS = \"iphoneos iphonesimulator\";\ncom.apple.product-type.application")
	resolved := ResolveForDir(&Plugin{Name: "swiftui-ios", Commands: []Command{{Name: "placeholder"}}}, dir)
	if hasCommandName(resolved.Commands, "运行测试") {
		t.Fatalf("commands = %+v, test command should require a test target", resolved.Commands)
	}
	if !hasCommandRun(resolved.Commands, "xcodebuild -project 'Demo.xcodeproj' -scheme 'Demo' -configuration Debug -destination 'generic/platform=iOS Simulator' build") {
		t.Fatalf("commands = %+v, want project build command", resolved.Commands)
	}
	if !hasCommandName(resolved.Commands, "在 iOS 模拟器中运行") {
		t.Fatalf("commands = %+v, want simulator run command for app target", resolved.Commands)
	}
}

func writeTestXcodeProject(t *testing.T, dir, name, pbxproj string) string {
	t.Helper()
	project := filepath.Join(dir, name+".xcodeproj")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte(pbxproj), 0644); err != nil {
		t.Fatal(err)
	}
	return project
}

func TestResolvePythonPrefersPythonOnPath(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"requirements.txt", "main.py"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatal(err)
	}
	pathWithInterpreters(t, "python", "python3")

	resolved := ResolveForDir(pythonPlugin(), dir)

	for _, want := range []string{
		"python -m pip install -r requirements.txt",
		"python main.py",
		"python -m pytest",
	} {
		if !hasCommandRun(resolved.Commands, want) {
			t.Errorf("commands %+v do not include %q", resolved.Commands, want)
		}
	}
	if env := envCheckByName(resolved.Env, "python"); env == nil || env.Check != "python --version" {
		t.Fatalf("python env check = %+v", resolved.Env)
	}
	if env := envCheckByName(resolved.Env, "pip"); env == nil || env.Check != "python -m pip --version" {
		t.Fatalf("pip env check = %+v", resolved.Env)
	}
}

func TestResolvePythonFallsBackToPython3(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"requirements.txt", "main.py"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "tests"), 0755); err != nil {
		t.Fatal(err)
	}
	pathWithInterpreters(t, "python3")

	resolved := ResolveForDir(pythonPlugin(), dir)

	for _, want := range []string{
		"python3 -m pip install -r requirements.txt",
		"python3 main.py",
		"python3 -m pytest",
	} {
		if !hasCommandRun(resolved.Commands, want) {
			t.Errorf("commands %+v do not include %q", resolved.Commands, want)
		}
	}
	if env := envCheckByName(resolved.Env, "python3"); env == nil || env.Check != "python3 --version" {
		t.Fatalf("python env check = %+v", resolved.Env)
	}
	if env := envCheckByName(resolved.Env, "pip"); env == nil || env.Check != "python3 -m pip --version" {
		t.Fatalf("pip env check = %+v", resolved.Env)
	}
	if envCheckByName(resolved.Env, "python") != nil {
		t.Fatalf("stale python env check remains: %+v", resolved.Env)
	}
}

func TestResolvePythonDefaultsToPythonWithoutInterpreterOnPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.py"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	pathWithInterpreters(t)

	resolved := ResolveForDir(pythonPlugin(), dir)

	if !hasCommandRun(resolved.Commands, "python main.py") {
		t.Fatalf("commands = %+v, want default python interpreter", resolved.Commands)
	}
	if env := envCheckByName(resolved.Env, "python"); env == nil || env.Check != "python --version" {
		t.Fatalf("python env check = %+v", resolved.Env)
	}
}

func TestResolvePythonAdaptsGenericCommands(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	pathWithInterpreters(t, "python3")

	resolved := ResolveForDir(pythonPlugin(), dir)

	for _, want := range []string{
		"python3 -m pip install -r requirements.txt",
		"python3 main.py",
		"python3 -m pytest",
	} {
		if !hasCommandRun(resolved.Commands, want) {
			t.Errorf("commands %+v do not include %q", resolved.Commands, want)
		}
	}
}

// pathWithInterpreters points PATH at a temp directory containing an
// executable marker file for each given interpreter name and returns the
// directory. Nothing is executed; only LookPath visibility matters.
func pathWithInterpreters(t *testing.T, names ...string) string {
	t.Helper()
	bin := t.TempDir()
	for _, name := range names {
		file := name
		if runtime.GOOS == "windows" {
			file += ".exe"
		}
		path := filepath.Join(bin, file)
		if err := os.WriteFile(path, nil, 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	return bin
}

func pythonPlugin() *Plugin {
	return &Plugin{
		Name: "python",
		Commands: []Command{
			{Name: "安装依赖", Run: "pip install -r requirements.txt"},
			{Name: "运行", Run: "python main.py"},
			{Name: "运行测试", Run: "pytest"},
		},
		Env: []EnvCheck{
			{Name: "python", Check: "python --version", Required: true},
			{Name: "pip", Check: "pip --version", Required: true},
		},
	}
}

func envCheckByName(checks []EnvCheck, name string) *EnvCheck {
	for i := range checks {
		if checks[i].Name == name {
			return &checks[i]
		}
	}
	return nil
}

func TestResolveMavenProjectTypes(t *testing.T) {
	tests := []struct {
		name     string
		pom      string
		wantRuns []string
		wantName string
		wantType string
	}{
		{
			name:     "jar",
			pom:      `<project><modelVersion>4.0.0</modelVersion></project>`,
			wantRuns: []string{"mvn compile", "mvn test", "mvn package", "mvn verify", "mvn install", "mvn dependency:tree"},
			wantName: "打包 JAR",
			wantType: "java-maven",
		},
		{
			name:     "war",
			pom:      `<project><packaging>war</packaging></project>`,
			wantRuns: []string{"mvn package"},
			wantName: "打包 WAR",
			wantType: "java-maven",
		},
		{
			name:     "multi-module",
			pom:      `<project><packaging>pom</packaging><modules><module>api</module><module>app</module></modules></project>`,
			wantRuns: []string{"mvn package", "mvn test", "mvn verify", "mvn install"},
			wantName: "构建全部模块",
			wantType: "java-maven",
		},
		{
			name: "spring-boot",
			pom: `<project><parent><artifactId>spring-boot-starter-parent</artifactId></parent>` +
				`<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
			wantRuns: []string{"mvn spring-boot:run"},
			wantName: "运行 Spring Boot",
			wantType: "spring-boot-maven",
		},
		{
			name: "plugin applications",
			pom: `<project><build><plugins>` +
				`<plugin><artifactId>quarkus-maven-plugin</artifactId></plugin>` +
				`<plugin><artifactId>exec-maven-plugin</artifactId></plugin>` +
				`</plugins></build></project>`,
			wantRuns: []string{"mvn quarkus:dev", "mvn exec:java"},
			wantName: "运行应用",
			wantType: "java-maven",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(test.pom), 0644); err != nil {
				t.Fatal(err)
			}
			resolved := ResolveForDir(mavenPlugin(), dir)
			for _, want := range test.wantRuns {
				if !hasCommandRun(resolved.Commands, want) {
					t.Errorf("commands %+v do not include %q", resolved.Commands, want)
				}
			}
			if !hasCommandName(resolved.Commands, test.wantName) {
				t.Errorf("commands %+v do not include name %q", resolved.Commands, test.wantName)
			}
			if resolved.Name != test.wantType {
				t.Errorf("type = %q, want %q", resolved.Name, test.wantType)
			}
		})
	}
}

func TestResolveMavenDistinguishesBootRootFromCloudReactor(t *testing.T) {
	tests := []struct {
		name         string
		pom          string
		wantType     string
		wantCommand  string
		wantCloudRun bool
	}{
		{
			name: "boot root with managed Cloud BOM and Cloud profile",
			pom: `<project><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId></parent>` +
				`<artifactId>platform-parent</artifactId><packaging>pom</packaging><modules><module>core</module><module>application</module></modules>` +
				`<dependencyManagement><dependencies><dependency><groupId>org.springframework.cloud</groupId><artifactId>spring-cloud-dependencies</artifactId></dependency></dependencies></dependencyManagement>` +
				`<profiles><profile><id>SpringCloud</id><modules><module>server-cloud</module></modules></profile></profiles></project>`,
			wantType:    "spring-boot-maven",
			wantCommand: "构建 Spring Boot 模块",
		},
		{
			name: "Cloud reactor identified by structure",
			pom: `<project><parent><artifactId>platform-parent</artifactId></parent><artifactId>server-cloud</artifactId><name>SERVER CLOUD</name>` +
				`<packaging>pom</packaging><modules><module>cloud-gateway</module><module>cloud-nacos</module><module>system-cloud-start</module></modules></project>`,
			wantType:    "spring-cloud-maven",
			wantCommand: "构建 Spring Cloud 模块",
		},
		{
			name: "Cloud service identified by direct custom starter",
			pom: `<project><parent><artifactId>platform-parent</artifactId></parent><artifactId>account-service</artifactId>` +
				`<dependencies><dependency><groupId>example</groupId><artifactId>example-boot-starter-cloud</artifactId></dependency></dependencies>` +
				`<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
			wantType:     "spring-cloud-maven",
			wantCommand:  "运行 Spring Cloud 服务",
			wantCloudRun: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(test.pom), 0644); err != nil {
				t.Fatal(err)
			}
			resolved := ResolveForDir(mavenPlugin(), dir)
			if resolved.Name != test.wantType {
				t.Fatalf("type = %q, want %q", resolved.Name, test.wantType)
			}
			if !hasCommandName(resolved.Commands, test.wantCommand) {
				t.Fatalf("commands %+v do not include %q", resolved.Commands, test.wantCommand)
			}
			if test.wantCloudRun && !hasCommandRun(resolved.Commands, "mvn spring-boot:run") {
				t.Fatalf("Cloud service commands = %+v", resolved.Commands)
			}
		})
	}
}

func TestResolveMavenClassifiesMalformedCloudPom(t *testing.T) {
	dir := t.TempDir()
	pom := `<project><parent><artifactId>server-cloud</artifactId></parent><artifactId>system-cloud-start</artifactId>` +
		`<description>broken legacy text?/description>` +
		`<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
	resolved := ResolveForDir(mavenPlugin(), dir)
	if resolved.Name != "spring-cloud-maven" || !hasCommandName(resolved.Commands, "运行 Spring Cloud 服务") {
		t.Fatalf("malformed Cloud POM resolved as %+v", resolved)
	}
}

func TestResolveMavenInheritsFrameworkFromLocalReactor(t *testing.T) {
	root := t.TempDir()
	rootPom := `<project><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId></parent>` +
		`<artifactId>platform</artifactId><packaging>pom</packaging><modules><module>normal</module><module>server-cloud</module></modules></project>`
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(rootPom), 0644); err != nil {
		t.Fatal(err)
	}
	normal := filepath.Join(root, "normal", "library")
	cloud := filepath.Join(root, "server-cloud", "service")
	if err := os.MkdirAll(normal, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cloud, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "normal", "pom.xml"), []byte(`<project><artifactId>normal</artifactId><packaging>pom</packaging><modules><module>library</module></modules></project>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(normal, "pom.xml"), []byte(`<project><artifactId>library</artifactId></project>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "server-cloud", "pom.xml"), []byte(`<project><artifactId>server-cloud</artifactId><packaging>pom</packaging><modules><module>service</module></modules></project>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloud, "pom.xml"), []byte(`<project><artifactId>service</artifactId></project>`), 0644); err != nil {
		t.Fatal(err)
	}
	if got := ResolveForDir(mavenPlugin(), normal).Name; got != "spring-boot-maven" {
		t.Fatalf("normal module type = %q", got)
	}
	if got := ResolveForDir(mavenPlugin(), cloud).Name; got != "spring-cloud-maven" {
		t.Fatalf("Cloud module type = %q", got)
	}
}

func TestResolveMavenAggregatorAddsRunnableDescendants(t *testing.T) {
	root := t.TempDir()
	writeTestPom(t, root, `<project><parent><groupId>org.springframework.boot</groupId><artifactId>spring-boot-starter-parent</artifactId></parent>`+
		`<artifactId>platform</artifactId><packaging>pom</packaging><modules><module>components</module></modules>`+
		`<profiles><profile><id>SpringCloud</id><activation><activeByDefault>true</activeByDefault></activation><modules><module>server-cloud</module></modules></profile></profiles></project>`)
	components := filepath.Join(root, "components")
	service := filepath.Join(components, "system-start")
	cloud := filepath.Join(root, "server-cloud")
	if err := os.MkdirAll(service, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cloud, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestPom(t, components, `<project><artifactId>components</artifactId><packaging>pom</packaging><modules><module>system-start</module></modules></project>`)
	writeTestPom(t, service, `<project><artifactId>system-start</artifactId><build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`)
	writeTestPom(t, cloud, `<project><artifactId>server-cloud</artifactId><build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`)

	resolved := ResolveForDir(mavenPlugin(), root)
	if !hasCommandRun(resolved.Commands, "mvn -P!SpringCloud -pl components/system-start spring-boot:run") {
		t.Fatalf("aggregator commands = %+v", resolved.Commands)
	}
	if !hasCommandRun(resolved.Commands, "mvn -P!SpringCloud package") {
		t.Fatalf("default-active Cloud profile was not disabled for the Boot build: %+v", resolved.Commands)
	}
	if hasCommandRun(resolved.Commands, "mvn -pl server-cloud spring-boot:run") {
		t.Fatalf("profile-only Cloud module leaked into Boot commands: %+v", resolved.Commands)
	}
}

func TestResolveMavenCloudAggregatorAddsEachRunnableService(t *testing.T) {
	root := t.TempDir()
	writeTestPom(t, root, `<project><artifactId>server-cloud</artifactId><packaging>pom</packaging><modules><module>gateway</module><module>visual</module></modules></project>`)
	gateway := filepath.Join(root, "gateway")
	visual := filepath.Join(root, "visual")
	monitor := filepath.Join(visual, "monitor")
	for _, dir := range []string{gateway, monitor} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestPom(t, gateway, `<project><parent><artifactId>server-cloud</artifactId></parent><artifactId>gateway</artifactId><build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`)
	writeTestPom(t, visual, `<project><parent><artifactId>server-cloud</artifactId></parent><artifactId>visual</artifactId><packaging>pom</packaging><modules><module>monitor</module></modules></project>`)
	writeTestPom(t, monitor, `<project><parent><artifactId>visual-cloud</artifactId></parent><artifactId>monitor</artifactId><build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`)

	resolved := ResolveForDir(mavenPlugin(), root)
	for _, run := range []string{"mvn -pl gateway spring-boot:run", "mvn -pl visual/monitor spring-boot:run"} {
		if !hasCommandRun(resolved.Commands, run) {
			t.Errorf("Cloud aggregator commands %+v do not include %q", resolved.Commands, run)
		}
	}
}

func writeTestPom(t *testing.T, dir, pom string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveMavenUsesPlatformWrapperWithoutGlobalMaven(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`<project/>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mvnw.cmd"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	p := mavenPlugin()
	resolveMaven(p, dir, "windows")
	if !hasCommandRun(p.Commands, `.\mvnw.cmd package`) {
		t.Fatalf("commands = %+v, want Windows Maven wrapper", p.Commands)
	}
	for _, check := range p.Env {
		if check.Name == "maven" {
			t.Fatalf("wrapper project still requires global Maven: %+v", p.Env)
		}
	}
}

func TestResolveMavenUsesUnixWrapper(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(`<project/>`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mvnw"), nil, 0755); err != nil {
		t.Fatal(err)
	}
	p := mavenPlugin()
	resolveMaven(p, dir, "linux")
	if !hasCommandRun(p.Commands, `./mvnw test`) {
		t.Fatalf("commands = %+v, want Unix Maven wrapper", p.Commands)
	}
}

func mavenPlugin() *Plugin {
	return &Plugin{
		Name:     "java-maven",
		Commands: []Command{{Name: "Package", Run: "mvn package"}},
		Env: []EnvCheck{
			{Name: "java", Check: "java -version", Required: true},
			{Name: "maven", Check: "mvn --version", Required: true},
		},
	}
}

func hasCommandRun(commands []Command, run string) bool {
	for _, command := range commands {
		if command.Run == run {
			return true
		}
	}
	return false
}

func hasCommandName(commands []Command, name string) bool {
	for _, command := range commands {
		if command.Name == name {
			return true
		}
	}
	return false
}

func commandRunByName(commands []Command, name string) string {
	for _, command := range commands {
		if command.Name == name {
			return command.Run
		}
	}
	return ""
}
