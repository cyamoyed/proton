# Proton

English | [简体中文](README.md)
[![Release](https://img.shields.io/github/v/release/cyamoyed/proton)](https://github.com/cyamoyed/proton/releases/latest)
[![License](https://img.shields.io/github/license/cyamoyed/proton)](LICENSE)

> A lightweight project launcher for AI-assisted development workflows.

Proton detects project types and runtime environments, then provides relevant commands through an interactive terminal interface. Start services, run tests, and build applications without keeping a heavyweight IDE open.

## Contents

- [Why Proton](#why-proton)
- [Features](#features)
- [Quick start](#quick-start)
- [Installation](#installation)
- [Building from source](#building-from-source)
- [Usage](#usage)
- [Supported project types](#supported-project-types)
- [Configuration](#configuration)
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
- **Cross-platform**: Supports Windows, macOS, and Linux

## Quick start

Download and run the appropriate installer from the [latest release](https://github.com/cyamoyed/proton/releases/latest), then open a new terminal:

```bash
cd /path/to/project
proton
```

Proton detects the project, checks required tools, and lists available commands. Use the arrow keys to select a command and press `Enter` to run it.

## Installation

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

Download the matching `proton-installer-darwin-*` file from the [latest release](https://github.com/cyamoyed/proton/releases/latest), then run:

```bash
chmod +x ~/Downloads/proton-installer-darwin-arm64
~/Downloads/proton-installer-darwin-arm64
```

Use `amd64` for Intel Macs and `arm64` for Apple Silicon.

### Linux

Download the matching `proton-installer-linux-*` file from the [latest release](https://github.com/cyamoyed/proton/releases/latest), then run:

```bash
chmod +x ~/Downloads/proton-installer-linux-amd64
~/Downloads/proton-installer-linux-amd64
```

On macOS and Linux, Proton installs to `~/.local/bin/proton`. If necessary, the installer adds that directory to the active shell profile. Open a new terminal after installation.

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
```

## Supported project types

| Type | Detection files |
|------|-----------------|
| Node.js | `package.json` |
| Python | `requirements.txt`, `pyproject.toml`, `setup.py`, `Pipfile` |
| Go | `go.mod` |
| Rust | `Cargo.toml` |
| Java | `pom.xml`, `build.gradle`, `build.gradle.kts` |
| C++ | `CMakeLists.txt`, `meson.build` |

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

Built-in plugins adapt their recommendations to the detected project:

- Node.js reads `package.json` scripts and detects npm, pnpm, Yarn, or Bun from lock files.
- Python recommends install, run, and test commands only when supporting files or directories exist.
- Maven reads `pom.xml` and local parent reactors; distinguishes JAR, WAR, multi-module, Spring Boot, Spring Cloud, Quarkus, and Exec applications; shows different project types and command labels for Boot and Cloud; and prefers `mvnw` or `mvnw.cmd`.
- Maven aggregators recursively discover runnable modules that declare `spring-boot-maven-plugin` and expose reactor-relative `-pl` run commands on the root page; default-active Cloud profiles are disabled for normal Boot commands.
- Gradle prefers `gradlew` or `gradlew.bat` when a wrapper is available.
- C++ selects CMake and/or Meson commands from the actual build files.

## License

[MIT](LICENSE)
