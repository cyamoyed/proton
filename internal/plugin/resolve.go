package plugin

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// ResolveForDir returns a project-specific copy of a plugin. Built-in generic
// recommendations are narrowed to commands that the detected project supports.
func ResolveForDir(source *Plugin, dir string) *Plugin {
	resolved := *source
	resolved.Commands = append([]Command(nil), source.Commands...)
	resolved.Env = append([]EnvCheck(nil), source.Env...)

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
	}
	return &resolved
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
	p.Commands = commands
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
	for script := range manifest.Scripts {
		if !seen[script] {
			commands = append(commands, Command{Name: script, Run: manager + " run " + script, Description: "package.json script: " + script})
		}
	}
	p.Commands = commands
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
		p.Commands = commands
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
}

func resolveCpp(p *Plugin, dir string) {
	commands := []Command{}
	if exists(dir, "CMakeLists.txt") {
		commands = append(commands, Command{Name: "Build (CMake)", Run: "cmake -S . -B build && cmake --build build"})
	}
	if exists(dir, "meson.build") {
		commands = append(commands, Command{Name: "Build (Meson)", Run: "meson setup build && meson compile -C build"})
	}
	if len(commands) > 0 {
		p.Commands = commands
	}
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
