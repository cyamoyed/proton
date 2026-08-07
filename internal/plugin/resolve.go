package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	case "cpp":
		resolveCpp(&resolved, dir)
	}
	return &resolved
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
