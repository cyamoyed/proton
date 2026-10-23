# Proton

English | [简体中文](README.md)
[![Release](https://img.shields.io/github/v/release/cyamoyed/proton)](https://github.com/cyamoyed/proton/releases/latest)
[![License](https://img.shields.io/github/license/cyamoyed/proton)](LICENSE)

> Zero-configuration task discovery and execution for unfamiliar repositories and polyglot monorepos.

Proton detects project types, existing tasks, and runtime environments, then exposes relevant commands through an interactive terminal or a machine-readable CLI. Developers can start services, test, and build quickly, while coding agents can consume the same local task catalog.

## Contents

- [Why Proton](#why-proton)
- [Features](#features)
- [Quick start](#quick-start)
- [Installation](#installation)
- [Building from source](#building-from-source)
- [Usage](#usage)
- [MCP integration](#mcp-integration)
- [Supported project types](#supported-project-types)
- [Configuration](#configuration)
- [Roadmap](ROADMAP.md)
- [Contributing](#contributing)
- [License](#license)

## Why Proton

As AI-assisted development becomes part of everyday engineering workflows, tasks such as understanding, generating, modifying, and troubleshooting code no longer require a full heavyweight IDE at all times. For frequent operations such as starting services, running tests, and building applications, developers often need only an accurate and convenient command entry point.

Proton was created for this purpose. It identifies project structures and runtime environments, then brings common development commands into a lightweight terminal interface. Developers can start and manage projects without continuously running resource-intensive IDEs such as IntelliJ IDEA. Proton is not intended to replace every IDE capability; it provides a focused, resource-efficient alternative for AI-era workflows, reducing memory pressure and leaving more system resources available for applications, containers, and AI tools.

## Features

- **Context-aware detection**: Scans and groups technologies by project directory instead of showing duplicate projects
- **Relevant commands**: Reads project files to show commands the project actually supports, including package-manager and wrapper detection
- **Interactive TUI**: Navigate projects and commands with keyboard shortcuts
- **Environment checks**: Verify required tools before running commands
- **Plugin system**: Add custom project types through YAML plugins
- **Automation interface**: List tasks as text or JSON and run them non-interactively by stable task ID
- **MCP server**: `proton mcp` starts a stdio MCP server so AI agents can discover and run tasks directly
- **Cross-platform**: Supports Windows, macOS, and Linux

## Quick start

On macOS / Linux, install with a single command (for Windows see [Installation](#installation)), then open a new terminal:

```bash
curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash
```

```bash
cd /path/to/project
proton
```

Proton detects the project, checks required tools, and lists available commands. Use the arrow keys to select a command and press `Enter` to run it.

## Installation

> Note: on macOS / Linux the `proton-*` release assets are command-line programs. Do not double-click them in Finder / your file manager (it will report an invalid text encoding); run them in a terminal as shown below.

### Windows

1. Open the [latest Proton release](https://github.com/cyamoyed/proton/releases/latest).
2. Download `proton-installer-windows-amd64.exe` for most PCs, or the `arm64` build for Windows on ARM.
3. Run the installer. It displays progress while copying Proton and updating PATH; administrator permission is not required.
4. Open a new terminal and verify the installation:

```powershell
proton --version
```

Proton installs to `%LOCALAPPDATA%\Proton\bin\proton.exe` and adds that directory to the current user's PATH.

When launched by double-clicking, the installer keeps the result visible until you press Enter. Installation and uninstallation update the Windows user environment directly without starting PowerShell.

### macOS

Run one command in a terminal; the install script detects the chip automatically (Apple Silicon or Intel):

```bash
curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash
```

If the repository is private or downloads are unavailable, get `install.sh` and the matching `proton-darwin-*` asset from the [latest release](https://github.com/cyamoyed/proton/releases/latest) (or a GitHub Actions build artifact), then run:

```bash
sh install.sh ~/Downloads/proton-darwin-arm64   # Apple Silicon; use amd64 on Intel Macs
```

`install.sh` also works without arguments: it first looks for a platform-matching `proton-darwin-*` binary next to itself, and only downloads from the GitHub Release when none is found. Open a new terminal afterwards and verify:

```bash
proton --version
```

### Linux

Same as macOS:

```bash
curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash
```

or download `install.sh` plus the matching `proton-linux-*` asset and run:

```bash
sh install.sh ~/Downloads/proton-linux-amd64   # use arm64 on ARM devices
```

On macOS and Linux, Proton installs to `~/.local/bin/proton`. If that directory is not in PATH, the install script adds it to the active shell profile. Open a new terminal after installation.

### Uninstall

```bash
proton uninstall
```

### Developer installation

Developers with Go installed can also use:

```bash
go install github.com/cyamoyed/proton/cmd/proton@latest
```

## Building from source

Building requires the Go version declared in `go.mod` or a compatible newer version.

### Windows

Build Proton for the current Windows architecture:

```powershell
.\make.cmd build
```

This produces:

```text
build\proton.exe
build\proton-installer.exe
```

Build portable binaries and self-installers for every supported target:

```powershell
.\make.cmd build-all
```

### macOS and Linux

Build for the current platform:

```bash
make build
```

Build every supported target:

```bash
make build-all
```

### Build directly with Go

Build only the CLI without the project scripts:

```bash
go build -o build/proton ./cmd/proton
```

On Windows:

```powershell
go build -o build\proton.exe .\cmd\proton
```

Supply a release version with linker flags:

```powershell
go build -ldflags "-s -w -X main.version=v1.0.0" -o build\proton-installer.exe .\cmd\proton
```

### Verification

Run before publishing:

```bash
go test ./...
go vet ./...
```

## Usage

1. Navigate to a project directory or pass a directory to Proton.
2. Run `proton`, or run `proton /path/to/project`.
3. Use the arrow keys to navigate projects and commands.
4. Press `Tab` to switch between the Commands and Environment panels.
5. Press `Enter` to execute the selected command.
6. Press `q` or `Ctrl+C` to quit.

### CLI options

```text
proton [options] [directory]
proton list [options] [directory]
proton run [options] <task-id> [directory]
proton mcp
proton install
proton uninstall

  -depth int
        maximum subdirectory scan depth (default 3; 0 scans only the target directory)
  -no-recursive
        scan only the target directory
  -version
        print the version and exit
```

Run `proton --help` to display the current options. Examples:

```bash
proton .
proton --depth 1 ../workspace
proton --no-recursive /path/to/project
proton list --json /path/to/project
proton run --dry-run root:运行测试 /path/to/project
proton run root:运行测试 /path/to/project
```

`proton list` shows each task's ID, shell command, and source. Add `--json` for a machine-readable catalog with a `schema_version`, suitable for scripts and AI agents. `proton run` requires an exact task ID. Tasks marked high-risk or requiring confirmation also require `--yes` for non-interactive execution.

## MCP integration

`proton mcp` starts a Model Context Protocol server over stdio, so MCP-capable clients such as Claude Desktop, Cursor, or Codex can use Proton without parsing CLI output. Add it to any MCP client configuration:

```json
{
  "mcpServers": {
    "proton": {
      "command": "proton",
      "args": ["mcp"]
    }
  }
}
```

The server exposes three tools:

| Tool | Description |
|------|-------------|
| `list_tasks` | Returns the same task catalog as `proton list --json`, with stable task IDs, commands, risk levels, and required tools |
| `run_task` | Runs a task by its exact ID; `dry_run` reviews the command first, `approved` approves high-risk tasks, and `timeout_seconds` bounds execution (default 120s, clamped to 1-600; the whole process tree is killed at the timeout and captured output is still returned) |
| `check_env` | Checks whether the tools required by discovered tasks are installed, with detected versions |

Tasks marked high-risk or `confirm: true` require an explicit `approved: true`, mirroring the CLI `--yes` gate. Commands run through the system shell in the task's project directory with the project's `.env` loaded; combined stdout/stderr is returned with the result (capped at 1 MiB, truncated beyond that).

## Supported project types

| Type | Detection files |
|------|-----------------|
| Node.js | `package.json` |
| Python | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile` |
| Go | `go.mod` |
| Rust | `Cargo.toml` |
| Java | `pom.xml`, `build.gradle`, `build.gradle.kts` |
| C++ | `CMakeLists.txt`, `meson.build` |
| just | `justfile`, `Justfile`, `.justfile` |
| Task | `Taskfile.yml`, `Taskfile.yaml`, and lowercase variants |
| SwiftUI (iOS, macOS only) | `*.xcodeproj`, verified through iOS build settings and SwiftUI source imports |

## Configuration

### Global configuration

Create `~/.proton/config.yaml`:

```yaml
plugins:
  - /path/to/custom-plugin.yaml
```

### Project configuration

Create `.proton.yaml` in the project root:

```yaml
plugins:
  - /path/to/project-specific-plugin.yaml
```

Relative plugin paths are resolved from the directory containing the configuration file. Proton reads the target project's `.proton.yaml` even when invoked as `proton /path/to/project` from another directory.

### Custom plugins

Create a YAML file to define a project type. Plugin commands run through the system shell, so only install and use trusted plugin files:

```yaml
name: my-project
version: 1.0.0
detect:
  files:
    - my-config.json
  dirs:
    - src
commands:
  - name: Build
    description: Build the application
    run: npm run build
  - name: Test
    run: npm test
  - name: Deploy production
    description: Deploy the current revision to production
    run: ./scripts/deploy.sh
    risk: high
    confirm: true
env:
  - name: Node.js
    check: node --version
    required: true
```

Command metadata is optional. Set `confirm: true` or `risk: high` for commands that must not execute with a single keypress. Proton treats plugin files as trusted shell configuration, so review third-party plugins before installing them.

`detect.files` and `detect.dirs` support the `*`, `?`, and `[...]` filename glob patterns.

Built-in plugins adapt their recommendations to the detected project:

- Node.js reads `package.json` scripts and detects npm, pnpm, Yarn, or Bun from lock files.
- Python recommends install, run, and test commands only when supporting files or directories exist.
- Maven reads `pom.xml` and local parent reactors; distinguishes JAR, WAR, multi-module, Spring Boot, Spring Cloud, Quarkus, and Exec applications; shows different project types and command labels for Boot and Cloud; and prefers `mvnw` or `mvnw.cmd`.
- Maven aggregators recursively discover runnable modules that declare `spring-boot-maven-plugin` and expose reactor-relative `-pl` run commands on the root page; default-active Cloud profiles are disabled for normal Boot commands.
- Gradle prefers `gradlew` or `gradlew.bat` when a wrapper is available.
- C++ selects CMake and/or Meson commands from the actual build files.
- Justfiles and Taskfiles are read as declared task sources, excluding private or internal tasks.
- SwiftUI iOS projects detect Xcode projects, workspaces, and schemes. Proton can open Xcode, list schemes, perform a build-only operation, or build, install, and launch the app in iOS Simulator. Projects with test targets can also run tests on an automatically selected simulator.

## License

[MIT](LICENSE)
