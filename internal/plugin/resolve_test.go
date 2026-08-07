package plugin

import (
	"os"
	"path/filepath"
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
			wantName: "Package JAR",
			wantType: "java-maven",
		},
		{
			name:     "war",
			pom:      `<project><packaging>war</packaging></project>`,
			wantRuns: []string{"mvn package"},
			wantName: "Package WAR",
			wantType: "java-maven",
		},
		{
			name:     "multi-module",
			pom:      `<project><packaging>pom</packaging><modules><module>api</module><module>app</module></modules></project>`,
			wantRuns: []string{"mvn package", "mvn test", "mvn verify", "mvn install"},
			wantName: "Build all modules",
			wantType: "java-maven",
		},
		{
			name: "spring-boot",
			pom: `<project><parent><artifactId>spring-boot-starter-parent</artifactId></parent>` +
				`<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
			wantRuns: []string{"mvn spring-boot:run"},
			wantName: "Run Spring Boot",
			wantType: "spring-boot-maven",
		},
		{
			name: "plugin applications",
			pom: `<project><build><plugins>` +
				`<plugin><artifactId>quarkus-maven-plugin</artifactId></plugin>` +
				`<plugin><artifactId>exec-maven-plugin</artifactId></plugin>` +
				`</plugins></build></project>`,
			wantRuns: []string{"mvn quarkus:dev", "mvn exec:java"},
			wantName: "Run application",
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
			wantCommand: "Build Spring Boot modules",
		},
		{
			name: "Cloud reactor identified by structure",
			pom: `<project><parent><artifactId>platform-parent</artifactId></parent><artifactId>server-cloud</artifactId><name>SERVER CLOUD</name>` +
				`<packaging>pom</packaging><modules><module>cloud-gateway</module><module>cloud-nacos</module><module>system-cloud-start</module></modules></project>`,
			wantType:    "spring-cloud-maven",
			wantCommand: "Build Spring Cloud modules",
		},
		{
			name: "Cloud service identified by direct custom starter",
			pom: `<project><parent><artifactId>platform-parent</artifactId></parent><artifactId>account-service</artifactId>` +
				`<dependencies><dependency><groupId>example</groupId><artifactId>example-boot-starter-cloud</artifactId></dependency></dependencies>` +
				`<build><plugins><plugin><artifactId>spring-boot-maven-plugin</artifactId></plugin></plugins></build></project>`,
			wantType:     "spring-cloud-maven",
			wantCommand:  "Run Spring Cloud service",
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
	if resolved.Name != "spring-cloud-maven" || !hasCommandName(resolved.Commands, "Run Spring Cloud service") {
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
