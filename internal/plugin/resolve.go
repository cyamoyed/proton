package plugin

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolveForDir returns a project-specific copy of a plugin. Built-in generic
// recommendations are narrowed to commands that the detected project supports.
func ResolveForDir(source *Plugin, dir string) *Plugin {
	resolved := *source
	resolved.Commands = append([]Command(nil), source.Commands...)
	resolved.Env = append([]EnvCheck(nil), source.Env...)
	resolved.Commands = setCommandSource(resolved.Commands, source.Source)

	switch source.Name {
	case "nodejs":
		resolveNode(&resolved, dir)
	case "python":
		resolvePython(&resolved, dir)
	case "java-gradle":
		resolveGradle(&resolved, dir)
	case "java-maven":
		resolveMaven(&resolved, dir, runtime.GOOS)
	case "cpp":
		resolveCpp(&resolved, dir)
	case "just":
		resolveJust(&resolved, dir)
	case "taskfile":
		resolveTaskfile(&resolved, dir)
	case "swiftui-ios":
		resolveSwiftUIIOS(&resolved, dir)
	}
	for i := range resolved.Commands {
		if resolved.Commands[i].Checks == nil {
			resolved.Commands[i].Checks = append([]EnvCheck(nil), resolved.Env...)
		}
	}
	return &resolved
}

// MatchesDir applies project checks that cannot be expressed by filename
// detection alone. Most plugins only need their declarative detection rules.
func MatchesDir(source *Plugin, dir string) bool {
	if source.Name == "swiftui-ios" {
		return isSwiftUIIOSProject(dir)
	}
	return true
}

type mavenProject struct {
	GroupID    string   `xml:"groupId"`
	ArtifactID string   `xml:"artifactId"`
	Name       string   `xml:"name"`
	Packaging  string   `xml:"packaging"`
	Modules    []string `xml:"modules>module"`
	Parent     struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
	} `xml:"parent"`
	Dependencies []mavenCoordinate `xml:"dependencies>dependency"`
	Build        struct {
		Plugins []mavenCoordinate `xml:"plugins>plugin"`
	} `xml:"build"`
	Profiles []mavenProfile `xml:"profiles>profile"`
}

type mavenCoordinate struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
}

type mavenProfile struct {
	ID         string   `xml:"id"`
	Modules    []string `xml:"modules>module"`
	Activation struct {
		ActiveByDefault bool `xml:"activeByDefault"`
	} `xml:"activation"`
}

func resolveMaven(p *Plugin, dir, goos string) {
	prefix, wrapped := mavenCommand(dir, goos)
	if wrapped {
		removeEnv(p, "maven")
		for i := range p.Commands {
			if strings.HasPrefix(p.Commands[i].Run, "mvn ") {
				p.Commands[i].Run = prefix + p.Commands[i].Run[3:]
			}
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "pom.xml"))
	if err != nil {
		return
	}
	project, _ := parseMavenProject(data)
	p.Name = classifyMavenProject(dir, project)

	packaging := strings.TrimSpace(project.Packaging)
	if packaging == "" {
		packaging = "jar"
	}
	reactorPrefix := prefix
	if packaging == "pom" && p.Name == "spring-boot-maven" {
		reactorPrefix += activeCloudProfileDeactivation(project.Profiles)
	}
	pluginIDs := make(map[string]bool, len(project.Build.Plugins))
	for _, buildPlugin := range project.Build.Plugins {
		pluginIDs[strings.TrimSpace(buildPlugin.ArtifactID)] = true
	}

	commands := []Command{{Name: "Clean", Run: prefix + " clean", Description: "Remove Maven build output"}}
	if packaging == "pom" && len(project.Modules) > 0 {
		buildName, testName, verifyName, installName := "Build all modules", "Test all modules", "Verify all modules", "Install all modules locally"
		if p.Name == "spring-boot-maven" {
			buildName, testName, verifyName, installName = "Build Spring Boot modules", "Test Spring Boot modules", "Verify Spring Boot modules", "Install Spring Boot modules locally"
		} else if p.Name == "spring-cloud-maven" {
			buildName, testName, verifyName, installName = "Build Spring Cloud modules", "Test Spring Cloud modules", "Verify Spring Cloud modules", "Install Spring Cloud modules locally"
		}
		commands = append(commands,
			Command{Name: buildName, Run: reactorPrefix + " package", Description: "Build the complete Maven reactor"},
			Command{Name: testName, Run: reactorPrefix + " test", Description: "Run tests across the Maven reactor"},
			Command{Name: verifyName, Run: reactorPrefix + " verify", Description: "Run all Maven verification checks"},
			Command{Name: installName, Run: reactorPrefix + " install", Description: "Install every reactor artifact in the local Maven repository"},
		)
		commands = append(commands, mavenAggregatorRunCommands(dir, project, reactorPrefix, p.Name)...)
	} else {
		packageName := "Package"
		if packaging == "jar" || packaging == "war" || packaging == "ear" {
			packageName += " " + strings.ToUpper(packaging)
		}
		commands = append(commands,
			Command{Name: "Compile", Run: prefix + " compile", Description: "Compile main Java sources"},
			Command{Name: "Run tests", Run: prefix + " test", Description: "Run Maven unit tests"},
			Command{Name: packageName, Run: prefix + " package", Description: "Create the Maven " + packaging + " artifact"},
			Command{Name: "Verify", Run: prefix + " verify", Description: "Run integration and verification checks"},
			Command{Name: "Install locally", Run: prefix + " install", Description: "Install the artifact in the local Maven repository"},
		)
	}

	if packaging != "pom" && (project.Parent.ArtifactID == "spring-boot-starter-parent" || pluginIDs["spring-boot-maven-plugin"]) {
		name, description := "Run Spring Boot", "Start the Spring Boot application"
		if p.Name == "spring-cloud-maven" {
			name, description = "Run Spring Cloud service", "Start this Spring Cloud service with Spring Boot"
		}
		commands = append(commands, Command{Name: name, Run: prefix + " spring-boot:run", Description: description})
	}
	if pluginIDs["quarkus-maven-plugin"] {
		commands = append(commands, Command{Name: "Run Quarkus dev mode", Run: prefix + " quarkus:dev", Description: "Start Quarkus in development mode"})
	}
	if pluginIDs["exec-maven-plugin"] {
		commands = append(commands, Command{Name: "Run application", Run: prefix + " exec:java", Description: "Run the configured Java main class"})
	}
	for _, profile := range project.Profiles {
		if isCloudProfile(profile.ID, profile.Modules) {
			commands = append(commands, Command{
				Name:        "Build Spring Cloud profile (" + strings.TrimSpace(profile.ID) + ")",
				Run:         prefix + " -P" + strings.TrimSpace(profile.ID) + " package",
				Description: "Build modules enabled by the Spring Cloud Maven profile",
			})
		}
	}
	commands = append(commands, Command{Name: "Dependency tree", Run: prefix + " dependency:tree", Description: "Show resolved Maven dependencies"})
	p.Commands = setCommandSource(commands, "pom.xml")
}

type runnableMavenModule struct {
	ArtifactID   string
	RelativePath string
}

func mavenAggregatorRunCommands(dir string, project mavenProject, prefix, projectType string) []Command {
	modules := findRunnableMavenModules(dir, project, projectType)
	commands := make([]Command, 0, len(modules))
	for _, module := range modules {
		framework := "Spring Boot"
		if projectType == "spring-cloud-maven" {
			framework = "Spring Cloud"
		}
		commands = append(commands, Command{
			Name:        "Run " + framework + ": " + module.ArtifactID,
			Run:         prefix + " -pl " + quoteMavenSelector(module.RelativePath) + " spring-boot:run",
			Description: "Start module " + module.RelativePath + "; install its reactor dependencies first if they are not available locally",
		})
	}
	return commands
}

func findRunnableMavenModules(rootDir string, project mavenProject, projectType string) []runnableMavenModule {
	root, err := filepath.Abs(rootDir)
	if err != nil {
		return nil
	}
	visited := make(map[string]bool)
	var runnable []runnableMavenModule

	var walk func(string, []string, int)
	walk = func(parentDir string, moduleNames []string, depth int) {
		if depth >= 8 {
			return
		}
		for _, moduleName := range moduleNames {
			moduleName = strings.TrimSpace(moduleName)
			if moduleName == "" || strings.Contains(moduleName, "${") || filepath.IsAbs(moduleName) {
				continue
			}
			moduleDir, err := filepath.Abs(filepath.Join(parentDir, filepath.FromSlash(moduleName)))
			if err != nil {
				continue
			}
			relative, err := filepath.Rel(root, moduleDir)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || visited[moduleDir] {
				continue
			}
			visited[moduleDir] = true
			data, err := os.ReadFile(filepath.Join(moduleDir, "pom.xml"))
			if err != nil {
				continue
			}
			child, _ := parseMavenProject(data)
			packaging := strings.TrimSpace(child.Packaging)
			if packaging == "" {
				packaging = "jar"
			}
			if packaging != "pom" && hasMavenPlugin(child, "spring-boot-maven-plugin") && classifyMavenProject(moduleDir, child) == projectType {
				artifactID := strings.TrimSpace(child.ArtifactID)
				if artifactID == "" {
					artifactID = filepath.Base(moduleDir)
				}
				runnable = append(runnable, runnableMavenModule{ArtifactID: artifactID, RelativePath: filepath.ToSlash(relative)})
			}
			walk(moduleDir, child.Modules, depth+1)
		}
	}

	walk(root, project.Modules, 0)
	return runnable
}

func quoteMavenSelector(selector string) string {
	if strings.ContainsAny(selector, " \t") {
		return `"` + strings.ReplaceAll(selector, `"`, `\"`) + `"`
	}
	return selector
}

func parseMavenProject(data []byte) (mavenProject, bool) {
	var project mavenProject
	if xml.Unmarshal(data, &project) == nil {
		return project, true
	}

	text := string(data)
	parent := firstXMLBlock(text, "parent")
	project.Parent.GroupID = firstXMLValue(parent, "groupId")
	project.Parent.ArtifactID = firstXMLValue(parent, "artifactId")
	withoutParent := strings.Replace(text, parent, "", 1)
	project.GroupID = firstXMLValue(withoutParent, "groupId")
	project.ArtifactID = firstXMLValue(withoutParent, "artifactId")
	project.Name = firstXMLValue(withoutParent, "name")
	project.Packaging = firstXMLValue(withoutParent, "packaging")
	project.Modules = xmlValues(firstXMLBlock(withoutParent, "modules"), "module")

	build := firstXMLBlock(withoutParent, "build")
	for _, pluginBlock := range xmlBlocks(build, "plugin") {
		project.Build.Plugins = append(project.Build.Plugins, mavenCoordinate{
			GroupID:    firstXMLValue(pluginBlock, "groupId"),
			ArtifactID: firstXMLValue(pluginBlock, "artifactId"),
		})
	}
	for _, profileBlock := range xmlBlocks(withoutParent, "profile") {
		profile := mavenProfile{
			ID:      firstXMLValue(profileBlock, "id"),
			Modules: xmlValues(firstXMLBlock(profileBlock, "modules"), "module"),
		}
		profile.Activation.ActiveByDefault = strings.EqualFold(firstXMLValue(firstXMLBlock(profileBlock, "activation"), "activeByDefault"), "true")
		project.Profiles = append(project.Profiles, profile)
	}
	return project, false
}

func classifyMavenProject(dir string, project mavenProject) string {
	kind := classifyCurrentMavenProject(project)
	if kind == "spring-cloud-maven" {
		return kind
	}

	current := filepath.Clean(dir)
	for i := 0; i < 8; i++ {
		parentDir := filepath.Dir(current)
		if parentDir == current {
			break
		}
		current = parentDir
		data, err := os.ReadFile(filepath.Join(current, "pom.xml"))
		if err != nil {
			continue
		}
		ancestor, _ := parseMavenProject(data)
		ancestorKind := classifyCurrentMavenProject(ancestor)
		if ancestorKind == "spring-cloud-maven" {
			return ancestorKind
		}
		if ancestorKind == "spring-boot-maven" {
			kind = ancestorKind
		}
	}
	return kind
}

func classifyCurrentMavenProject(project mavenProject) string {
	for _, dependency := range project.Dependencies {
		if isSpringCloudCoordinate(dependency) {
			return "spring-cloud-maven"
		}
	}
	if containsCloudToken(project.Parent.ArtifactID) {
		return "spring-cloud-maven"
	}
	cloudModules := 0
	for _, module := range project.Modules {
		if containsCloudToken(module) || containsInfrastructureToken(module) {
			cloudModules++
		}
	}
	if containsCloudToken(project.ArtifactID+" "+project.Name) && (len(project.Modules) > 0 || hasMavenPlugin(project, "spring-boot-maven-plugin") || len(project.Dependencies) > 0) {
		return "spring-cloud-maven"
	}
	if cloudModules >= 2 {
		return "spring-cloud-maven"
	}

	if project.Parent.GroupID == "org.springframework.boot" || project.Parent.ArtifactID == "spring-boot-starter-parent" || hasMavenPlugin(project, "spring-boot-maven-plugin") {
		return "spring-boot-maven"
	}
	for _, dependency := range project.Dependencies {
		if dependency.GroupID == "org.springframework.boot" || strings.HasPrefix(dependency.ArtifactID, "spring-boot-") {
			return "spring-boot-maven"
		}
	}
	return "java-maven"
}

func isSpringCloudCoordinate(coordinate mavenCoordinate) bool {
	groupID := strings.ToLower(strings.TrimSpace(coordinate.GroupID))
	artifactID := strings.ToLower(strings.TrimSpace(coordinate.ArtifactID))
	return strings.HasPrefix(groupID, "org.springframework.cloud") || groupID == "com.alibaba.cloud" ||
		strings.HasPrefix(artifactID, "spring-cloud-") || strings.Contains(artifactID, "starter-cloud") || strings.Contains(artifactID, "cloud-starter")
}

func hasMavenPlugin(project mavenProject, artifactID string) bool {
	for _, buildPlugin := range project.Build.Plugins {
		if strings.TrimSpace(buildPlugin.ArtifactID) == artifactID {
			return true
		}
	}
	return false
}

func containsCloudToken(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "spring-cloud") || strings.Contains(value, "cloud-") || strings.Contains(value, "-cloud")
}

func containsInfrastructureToken(value string) bool {
	value = strings.ToLower(value)
	return strings.Contains(value, "gateway") || strings.Contains(value, "nacos") || strings.Contains(value, "eureka") || strings.Contains(value, "sentinel")
}

func isCloudProfile(id string, modules []string) bool {
	if containsCloudToken(id) || strings.EqualFold(strings.TrimSpace(id), "cloud") {
		return true
	}
	for _, module := range modules {
		if containsCloudToken(module) {
			return true
		}
	}
	return false
}

func activeCloudProfileDeactivation(profiles []mavenProfile) string {
	var ids []string
	for _, profile := range profiles {
		if profile.Activation.ActiveByDefault && isCloudProfile(profile.ID, profile.Modules) {
			ids = append(ids, "!"+strings.TrimSpace(profile.ID))
		}
	}
	if len(ids) == 0 {
		return ""
	}
	return " -P" + strings.Join(ids, ",")
}

func xmlBlocks(text, tag string) []string {
	expression := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `(?:\s[^>]*)?>.*?</` + regexp.QuoteMeta(tag) + `\s*>`)
	return expression.FindAllString(text, -1)
}

func firstXMLBlock(text, tag string) string {
	blocks := xmlBlocks(text, tag)
	if len(blocks) == 0 {
		return ""
	}
	return blocks[0]
}

func xmlValues(text, tag string) []string {
	expression := regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `(?:\s[^>]*)?>\s*([^<]*?)\s*</` + regexp.QuoteMeta(tag) + `\s*>`)
	matches := expression.FindAllStringSubmatch(text, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, strings.TrimSpace(match[1]))
	}
	return values
}

func firstXMLValue(text, tag string) string {
	values := xmlValues(text, tag)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func mavenCommand(dir, goos string) (string, bool) {
	if goos == "windows" && exists(dir, "mvnw.cmd") {
		return ".\\mvnw.cmd", true
	}
	if exists(dir, "mvnw") {
		return "./mvnw", true
	}
	return "mvn", false
}

func removeEnv(p *Plugin, name string) {
	filtered := p.Env[:0]
	for _, check := range p.Env {
		if check.Name != name {
			filtered = append(filtered, check)
		}
	}
	p.Env = filtered
}

func resolveNode(p *Plugin, dir string) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return
	}
	var manifest struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return
	}
	manager := "npm"
	install := "npm install"
	if exists(dir, "pnpm-lock.yaml") {
		manager, install = "pnpm", "pnpm install"
	} else if exists(dir, "yarn.lock") {
		manager, install = "yarn", "yarn install"
	} else if exists(dir, "bun.lock") || exists(dir, "bun.lockb") {
		manager, install = "bun", "bun install"
	} else if exists(dir, "package-lock.json") {
		install = "npm ci"
	}
	commands := []Command{{Name: "Install dependencies", Run: install, Description: "Install dependencies using the detected package manager"}}
	preferred := []struct{ script, label string }{{"dev", "Start dev server"}, {"start", "Start"}, {"build", "Build"}, {"test", "Run tests"}, {"lint", "Lint"}, {"format", "Format"}}
	seen := map[string]bool{}
	for _, item := range preferred {
		if _, ok := manifest.Scripts[item.script]; ok {
			commands = append(commands, Command{Name: item.label, Run: manager + " run " + item.script, Description: "package.json script: " + item.script})
			seen[item.script] = true
		}
	}
	remaining := make([]string, 0, len(manifest.Scripts))
	for script := range manifest.Scripts {
		if !seen[script] {
			remaining = append(remaining, script)
		}
	}
	sort.Strings(remaining)
	for _, script := range remaining {
		commands = append(commands, Command{Name: script, Run: manager + " run " + script, Description: "package.json script: " + script})
	}
	p.Commands = setCommandSource(commands, "package.json")
	for i := range p.Env {
		if p.Env[i].Name == "npm" {
			p.Env[i].Name = manager
			p.Env[i].Check = manager + " --version"
		}
	}
}

func resolvePython(p *Plugin, dir string) {
	commands := make([]Command, 0, len(p.Commands))
	if exists(dir, "requirements.txt") {
		commands = append(commands, Command{Name: "Install dependencies", Run: "python -m pip install -r requirements.txt"})
	} else if exists(dir, "Pipfile") {
		commands = append(commands, Command{Name: "Install dependencies", Run: "pipenv install"})
	} else if exists(dir, "pyproject.toml") {
		commands = append(commands, Command{Name: "Install project", Run: "python -m pip install -e ."})
	}
	if exists(dir, "main.py") {
		commands = append(commands, Command{Name: "Run", Run: "python main.py"})
	}
	if exists(dir, "pytest.ini") || exists(dir, "conftest.py") || exists(dir, "tests") || exists(dir, "test") {
		commands = append(commands, Command{Name: "Run tests", Run: "python -m pytest"})
	}
	if len(commands) > 0 {
		p.Commands = setCommandSource(commands, "Python project files")
	}
}

func resolveGradle(p *Plugin, dir string) {
	wrapper := "gradle"
	if runtime.GOOS == "windows" && exists(dir, "gradlew.bat") {
		wrapper = ".\\gradlew.bat"
	} else if exists(dir, "gradlew") {
		wrapper = "./gradlew"
	}
	for i := range p.Commands {
		if len(p.Commands[i].Run) >= 6 && p.Commands[i].Run[:6] == "gradle" {
			p.Commands[i].Run = wrapper + p.Commands[i].Run[6:]
		}
	}
	if wrapper != "gradle" {
		for i := range p.Env {
			if p.Env[i].Name == "gradle" {
				p.Env = append(p.Env[:i], p.Env[i+1:]...)
				break
			}
		}
	}
	source := "build.gradle"
	if exists(dir, "build.gradle.kts") {
		source = "build.gradle.kts"
	}
	p.Commands = setCommandSource(p.Commands, source)
}

func resolveCpp(p *Plugin, dir string) {
	commands := []Command{}
	if exists(dir, "CMakeLists.txt") {
		commands = append(commands, Command{Name: "Build (CMake)", Run: "cmake -S . -B build && cmake --build build", Source: "CMakeLists.txt"})
	}
	if exists(dir, "meson.build") {
		commands = append(commands, Command{Name: "Build (Meson)", Run: "meson setup build && meson compile -C build", Source: "meson.build"})
	}
	if len(commands) > 0 {
		p.Commands = commands
	}
}

var justRecipePattern = regexp.MustCompile(`^([[:alnum:]_-]+)(?:[ \t]+[^:]*)?[ \t]*:(?:[^=].*)?$`)

func resolveJust(p *Plugin, dir string) {
	name := firstExisting(dir, "justfile", "Justfile", ".justfile")
	if name == "" {
		return
	}
	file, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return
	}
	defer file.Close()

	var commands []Command
	description := ""
	private := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "#!") {
			description = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
			continue
		}
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			private = strings.Contains(trimmed, "private")
			continue
		}
		if len(line) != len(strings.TrimLeft(line, " \t")) {
			description = ""
			continue
		}
		match := justRecipePattern.FindStringSubmatch(line)
		if len(match) != 2 {
			description = ""
			private = false
			continue
		}
		recipe := match[1]
		if !private {
			commands = append(commands, Command{
				Name:        recipe,
				Run:         "just " + shellQuote(recipe),
				Description: description,
				Source:      name,
			})
		}
		description = ""
		private = false
	}
	if len(commands) > 0 {
		p.Commands = commands
	}
}

func resolveTaskfile(p *Plugin, dir string) {
	name := firstExisting(dir, "Taskfile.yml", "Taskfile.yaml", "taskfile.yml", "taskfile.yaml")
	if name == "" {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return
	}
	var manifest struct {
		Tasks map[string]yaml.Node `yaml:"tasks"`
	}
	if yaml.Unmarshal(data, &manifest) != nil {
		return
	}
	names := make([]string, 0, len(manifest.Tasks))
	for taskName := range manifest.Tasks {
		names = append(names, taskName)
	}
	sort.Strings(names)
	commands := make([]Command, 0, len(names))
	for _, taskName := range names {
		var metadata struct {
			Description string `yaml:"desc"`
			Internal    bool   `yaml:"internal"`
		}
		node := manifest.Tasks[taskName]
		_ = node.Decode(&metadata)
		if metadata.Internal {
			continue
		}
		commands = append(commands, Command{
			Name:        taskName,
			Run:         "task " + shellQuote(taskName),
			Description: metadata.Description,
			Source:      name,
		})
	}
	if len(commands) > 0 {
		p.Commands = commands
	}
}

func firstExisting(dir string, names ...string) string {
	for _, name := range names {
		if exists(dir, name) {
			return name
		}
	}
	return ""
}

type xcodeScheme struct {
	name     string
	hasTests bool
	runnable bool
}

var (
	iosPlatformsPattern  = regexp.MustCompile(`(?m)SUPPORTED_PLATFORMS\s*=\s*"?[^;\n]*\biphoneos\b`)
	swiftUIImportPattern = regexp.MustCompile(`(?m)^\s*(?:@[_[:alnum:]]+\s+)?import\s+SwiftUI\b`)
)

func resolveSwiftUIIOS(p *Plugin, dir string) {
	projects := xcodeProjects(dir)
	if len(projects) == 0 {
		return
	}
	project := projects[0]
	for _, candidate := range projects {
		if isIOSXcodeProject(candidate) {
			project = candidate
			break
		}
	}

	containerFlag, containerPath := "-project", filepath.Base(project)
	if workspace := preferredXcodeWorkspace(dir, project); workspace != "" {
		containerFlag, containerPath = "-workspace", filepath.Base(workspace)
	}
	container := containerFlag + " " + shellQuote(containerPath)
	openPath := shellQuote(containerPath)

	schemes := xcodeSchemes(project, filepath.Join(dir, containerPath))
	if len(schemes) == 0 {
		schemes = []xcodeScheme{{
			name:     strings.TrimSuffix(filepath.Base(project), ".xcodeproj"),
			hasTests: xcodeProjectHasTests(project),
			runnable: xcodeProjectHasApp(project),
		}}
	}

	commands := []Command{
		{Name: "Open in Xcode", Run: "open " + openPath, Description: "Open the detected project in Xcode"},
		{Name: "List schemes", Run: "xcodebuild " + container + " -list", Description: "Show the schemes and targets available to xcodebuild"},
	}
	for _, scheme := range schemes {
		labelSuffix := ""
		if len(schemes) > 1 {
			labelSuffix = " (" + scheme.name + ")"
		}
		base := "xcodebuild " + container + " -scheme " + shellQuote(scheme.name)
		commands = append(commands, Command{
			Name:        "Build for iOS Simulator" + labelSuffix,
			Run:         base + " -configuration Debug -destination " + shellQuote("generic/platform=iOS Simulator") + " build",
			Description: "Compile the SwiftUI app without launching the iOS Simulator",
		})
		if scheme.runnable {
			commands = append(commands, Command{
				Name:        "Run in iOS Simulator" + labelSuffix,
				Run:         simulatorRunCommand(base),
				Description: "Boot an available iOS Simulator, build and install the app, then launch it",
			})
		}
		if scheme.hasTests {
			commands = append(commands, Command{
				Name:        "Run tests" + labelSuffix,
				Run:         simulatorSelectionCommand() + " && " + base + ` -destination "platform=iOS Simulator,id=$simulator_id" test`,
				Description: "Run the scheme's tests on the first available iOS Simulator",
			})
		}
	}
	p.Commands = setCommandSource(commands, filepath.Base(project))
}

func setCommandSource(commands []Command, source string) []Command {
	for i := range commands {
		if commands[i].Source == "" {
			commands[i].Source = source
		}
	}
	return commands
}

func isSwiftUIIOSProject(dir string) bool {
	projects := xcodeProjects(dir)
	if len(projects) == 0 {
		return false
	}
	hasIOSProject := false
	for _, project := range projects {
		if isIOSXcodeProject(project) {
			hasIOSProject = true
			break
		}
	}
	return hasIOSProject && directoryImportsSwiftUI(dir)
}

func xcodeProjects(dir string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.xcodeproj"))
	projects := matches[:0]
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil && info.IsDir() {
			projects = append(projects, match)
		}
	}
	sort.Strings(projects)
	return projects
}

func isIOSXcodeProject(project string) bool {
	data, err := os.ReadFile(filepath.Join(project, "project.pbxproj"))
	if err != nil {
		return false
	}
	contents := string(data)
	return strings.Contains(contents, "SDKROOT = iphoneos") ||
		strings.Contains(contents, "IPHONEOS_DEPLOYMENT_TARGET") ||
		iosPlatformsPattern.MatchString(contents)
}

func directoryImportsSwiftUI(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() && path != dir {
			name := entry.Name()
			if shouldSkipSwiftScanDirectory(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".swift" {
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil && swiftUIImportPattern.Match(data) {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func shouldSkipSwiftScanDirectory(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".xcodeproj") || strings.HasSuffix(name, ".xcworkspace") {
		return true
	}
	switch name {
	case "Pods", "Carthage", "DerivedData", "node_modules", "vendor", "build":
		return true
	}
	return false
}

func preferredXcodeWorkspace(dir, project string) string {
	workspaces, _ := filepath.Glob(filepath.Join(dir, "*.xcworkspace"))
	sort.Strings(workspaces)
	projectName := strings.TrimSuffix(filepath.Base(project), ".xcodeproj")
	for _, workspace := range workspaces {
		if strings.TrimSuffix(filepath.Base(workspace), ".xcworkspace") == projectName {
			return workspace
		}
	}
	for _, workspace := range workspaces {
		data, err := os.ReadFile(filepath.Join(workspace, "contents.xcworkspacedata"))
		if err == nil && strings.Contains(string(data), filepath.Base(project)) {
			return workspace
		}
	}
	return ""
}

func xcodeSchemes(project, container string) []xcodeScheme {
	patterns := []string{
		filepath.Join(project, "xcshareddata", "xcschemes", "*.xcscheme"),
		filepath.Join(project, "xcuserdata", "*", "xcschemes", "*.xcscheme"),
	}
	if strings.HasSuffix(container, ".xcworkspace") {
		patterns = append(patterns,
			filepath.Join(container, "xcshareddata", "xcschemes", "*.xcscheme"),
			filepath.Join(container, "xcuserdata", "*", "xcschemes", "*.xcscheme"),
		)
	}
	byName := make(map[string]xcodeScheme)
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			name := strings.TrimSuffix(filepath.Base(match), ".xcscheme")
			data, _ := os.ReadFile(match)
			contents := string(data)
			scheme := byName[name]
			scheme.name = name
			scheme.hasTests = scheme.hasTests || strings.Contains(contents, "<TestableReference")
			scheme.runnable = scheme.runnable || strings.Contains(contents, "<BuildableProductRunnable") || strings.Contains(contents, ".app&quot;")
			byName[name] = scheme
		}
	}
	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	schemes := make([]xcodeScheme, 0, len(names))
	for _, name := range names {
		schemes = append(schemes, byName[name])
	}
	return schemes
}

func xcodeProjectHasTests(project string) bool {
	data, err := os.ReadFile(filepath.Join(project, "project.pbxproj"))
	if err != nil {
		return false
	}
	contents := string(data)
	return strings.Contains(contents, "com.apple.product-type.bundle.unit-test") ||
		strings.Contains(contents, "com.apple.product-type.bundle.ui-testing")
}

func xcodeProjectHasApp(project string) bool {
	data, err := os.ReadFile(filepath.Join(project, "project.pbxproj"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "com.apple.product-type.application")
}

func simulatorRunCommand(base string) string {
	destination := ` -configuration Debug -destination "platform=iOS Simulator,id=$simulator_id"`
	buildSettings := base + destination + " -showBuildSettings"
	return simulatorSelectionCommand() +
		` && (xcrun simctl boot "$simulator_id" >/dev/null 2>&1 || true)` +
		` && open -a Simulator` +
		` && xcrun simctl bootstatus "$simulator_id" -b` +
		" && " + base + destination + " build" +
		` && build_settings="$(` + buildSettings + `)"` +
		` && app_path="$(printf '%s\n' "$build_settings" | awk -F ' = ' '/ TARGET_BUILD_DIR = / { build_dir=$2 } / WRAPPER_NAME = .*\.app$/ { print build_dir "/" $2; exit }')"` +
		` && bundle_id="$(printf '%s\n' "$build_settings" | awk -F ' = ' '/ PRODUCT_BUNDLE_IDENTIFIER = / { bundle_id=$2 } / WRAPPER_NAME = .*\.app$/ { print bundle_id; exit }')"` +
		` && test -d "$app_path" && test -n "$bundle_id"` +
		` && xcrun simctl install "$simulator_id" "$app_path"` +
		` && xcrun simctl launch "$simulator_id" "$bundle_id"`
}

func simulatorSelectionCommand() string {
	return `simulator_id="$(xcrun simctl list devices available | awk -F '[()]' '/^-- iOS / { ios=1; next } /^-- / { ios=0 } ios && /\(Booted\)/ { print $2; found=1; exit } ios && /\(Shutdown\)/ && !fallback { fallback=$2 } END { if (!found && fallback) print fallback }')" && test -n "$simulator_id"`
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
