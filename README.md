# Proton

[English](README_EN.md) | 简体中文
[![Release](https://img.shields.io/github/v/release/cyamoyed/proton)](https://github.com/cyamoyed/proton/releases/latest)
[![License](https://img.shields.io/github/license/cyamoyed/proton)](LICENSE)

> 面向陌生仓库和多语言 Monorepo 的零配置任务发现与执行工具。

Proton 自动识别项目类型、现有任务和运行环境，并通过交互式终端或机器可读 CLI 提供当前项目实际可用的命令。它既适合开发者快速启动服务、测试和构建，也可以作为 AI Agent 的本地任务目录。

## 目录

- [为什么选择 Proton](#为什么选择-proton)
- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [安装](#安装)
- [从源码构建](#从源码构建)
- [使用方法](#使用方法)
- [支持的项目类型](#支持的项目类型)
- [配置](#配置)
- [开发路线图](ROADMAP.md)
- [参与贡献](#参与贡献)
- [许可证](#许可证)

## 为什么选择 Proton

随着 AI 辅助开发逐渐融入日常工作流，代码理解、生成、修改和问题排查等任务不再始终依赖完整的重量级 IDE。对于启动服务、运行测试、执行构建等高频操作，开发者通常只需要准确、便捷的命令入口。

Proton 因此而诞生：通过识别项目结构和运行环境，将常用开发命令集中到轻量级终端界面中，让开发者无需持续运行 IntelliJ IDEA 等资源占用较高的 IDE，也能快速启动和管理项目。它并非试图取代所有 IDE 能力，而是为 AI 时代的开发工作流提供一个更轻量、更专注且资源友好的选择，从而降低内存占用，并将更多系统资源留给应用程序、容器和 AI 工具。

## 功能特性

- **上下文感知检测**：按照项目目录扫描并聚合技术栈，避免将同一个多技术栈项目重复展示
- **精准命令推荐**：读取项目文件，只显示项目实际支持的命令，包括包管理器和构建 Wrapper 检测
- **交互式终端界面**：使用键盘快捷键浏览项目和执行命令
- **环境检查**：执行命令前检查必需的开发工具是否已经安装
- **插件系统**：通过 YAML 插件扩展并添加自定义项目类型
- **自动化接口**：以文本或 JSON 列出任务，并通过稳定任务 ID 非交互执行
- **跨平台**：支持 Windows、macOS 和 Linux

## 快速开始

macOS / Linux 在终端执行一行命令即可安装（Windows 见 [安装](#安装)），然后重新打开终端：

```bash
curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash
```

```bash
cd /path/to/project
proton
```

Proton 会自动检测项目、检查必需工具并列出可执行命令。使用方向键选择命令，按 `Enter` 执行。

## 安装

> 注意：macOS / Linux 的 `proton-*` 发布附件都是命令行程序，不能在访达 / 文件管理器中双击运行（会提示"文本编码不适用"），请按下述方式在终端中执行。

### Windows

1. 打开 [Proton 最新版本](https://github.com/cyamoyed/proton/releases/latest)。
2. 大多数 Windows 电脑下载 `proton-installer-windows-amd64.exe`；Windows on ARM 设备下载 `arm64` 版本。
3. 运行安装程序。安装程序会显示复制文件和更新 PATH 的进度，无需管理员权限。
4. 打开一个新的终端并验证安装：

```powershell
proton --version
```

Proton 会安装到 `%LOCALAPPDATA%\Proton\bin\proton.exe`，并自动将该目录加入当前用户的 PATH。

双击启动安装程序时，安装结果会一直显示，直到按下 Enter。安装和卸载会直接更新 Windows 用户环境变量，不会启动 PowerShell。

### macOS

在终端执行一行命令，安装脚本会自动识别芯片类型（Apple Silicon / Intel）：

```bash
curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash
```

如果仓库私有或无法在线下载，从 [最新版本](https://github.com/cyamoyed/proton/releases/latest)（或 GitHub Actions 构建产物）下载 `install.sh` 和对应的 `proton-darwin-*` 附件，然后在终端执行：

```bash
sh install.sh ~/Downloads/proton-darwin-arm64   # Apple Silicon；Intel Mac 使用 amd64
```

`install.sh` 也可以不带参数运行：它会自动在脚本所在目录寻找与本机平台匹配的 `proton-darwin-*` 二进制，找不到时再从 GitHub Release 下载。安装完成后重新打开终端并验证：

```bash
proton --version
```

### Linux

安装方式与 macOS 相同：

```bash
curl -fsSL https://github.com/cyamoyed/proton/releases/latest/download/install.sh | bash
```

或下载 `install.sh` 与对应的 `proton-linux-*` 附件后执行：

```bash
sh install.sh ~/Downloads/proton-linux-amd64   # ARM 设备使用 arm64
```

在 macOS 和 Linux 上，Proton 会安装到 `~/.local/bin/proton`。如果 PATH 中没有该目录，安装脚本会更新当前 Shell 对应的配置文件。安装完成后请重新打开终端。

### 卸载

```bash
proton uninstall
```

### 开发者安装

已经安装 Go 的开发者也可以执行：

```bash
go install github.com/cyamoyed/proton/cmd/proton@latest
```

## 从源码构建

构建项目需要使用 `go.mod` 中声明的 Go 版本或兼容的更高版本。

### Windows

为当前 Windows 架构构建 Proton：

```powershell
.\make.cmd build
```

生成以下文件：

```text
build\proton.exe
build\proton-installer.exe
```

构建所有操作系统和架构的免安装程序及自安装程序：

```powershell
.\make.cmd build-all
```

### macOS 和 Linux

为当前平台构建：

```bash
make build
```

构建所有支持的平台：

```bash
make build-all
```

### 直接使用 Go 构建

不使用项目构建脚本，只构建 CLI：

```bash
go build -o build/proton ./cmd/proton
```

Windows：

```powershell
go build -o build\proton.exe .\cmd\proton
```

通过链接参数写入发布版本号：

```powershell
go build -ldflags "-s -w -X main.version=v1.0.0" -o build\proton-installer.exe .\cmd\proton
```

### 构建验证

发布前运行：

```bash
go test ./...
go vet ./...
```

## 使用方法

1. 进入项目目录，或者在命令中指定项目目录。
2. 运行 `proton`，也可以运行 `proton /path/to/project`。
3. 使用方向键在项目和命令之间移动。
4. 按 `Tab` 在“命令”和“环境”面板之间切换。
5. 按 `Enter` 执行选中的命令。
6. 按 `q` 或 `Ctrl+C` 退出。

### CLI 参数

```text
proton [选项] [目录]
proton list [选项] [目录]
proton run [选项] <任务 ID> [目录]
proton install
proton uninstall

  -depth int
        最大子目录扫描深度（默认 3；设置为 0 时仅扫描目标目录）
  -no-recursive
        仅扫描目标目录，不扫描子目录
  -version
        输出版本号并退出
```

运行 `proton --help` 查看当前支持的参数。例如：

```bash
proton .
proton --depth 1 ../workspace
proton --no-recursive /path/to/project
proton list --json /path/to/project
proton run --dry-run root:运行测试 /path/to/project
proton run root:运行测试 /path/to/project
```

`proton list` 按项目分组、一行一条地显示任务：启动命令排在最前并标记 `▶`，每个任务一行（名称 + Shell 命令）。任务 ID 由组头的「ID 前缀」加行尾 `(ID: ...)` 拼接而成（行尾仅在短 ID 与任务名不同时显示），例如 `proton run jeecg-server-cloud/jeecg-visual/jeecg-cloud-xxljob:运行-spring-cloud-服务`。列表只包含含启动类（应用启动入口）的模块——公共库、聚合根等无启动入口的模块自动隐藏（若扫描范围内没有任何可启动模块，则列出全部）。`--json` 始终输出完整目录，带有 `schema_version`，适合脚本和 AI Agent 使用。`proton run` 只接受完整任务 ID。对于标记为高风险或需要确认的任务，非交互执行还必须传入 `--yes`。

## 支持的项目类型

| 类型 | 检测文件 |
|------|----------|
| Node.js | `package.json` |
| Python | `requirements.txt`、`pyproject.toml`、`setup.py`、`Pipfile` |
| Go | `go.mod` |
| Rust | `Cargo.toml` |
| Java | `pom.xml`、`build.gradle`、`build.gradle.kts` |
| C++ | `CMakeLists.txt`、`meson.build` |
| just | `justfile`、`Justfile`、`.justfile` |
| Task | `Taskfile.yml`、`Taskfile.yaml` 及小写变体 |
| SwiftUI (iOS，仅 macOS) | `*.xcodeproj`，并校验 iOS 构建设置和 SwiftUI 源码 |

## 配置

### 全局配置

创建 `~/.proton/config.yaml`：

```yaml
plugins:
  - /path/to/custom-plugin.yaml
```

### 项目配置

在项目根目录创建 `.proton.yaml`：

```yaml
plugins:
  - /path/to/project-specific-plugin.yaml
```

相对插件路径以声明它的配置文件所在目录为基准。即使从其他目录运行 `proton /path/to/project`，Proton 也会读取目标项目中的 `.proton.yaml`。

### 自定义插件

创建一个 YAML 文件来定义新的项目类型。插件命令会通过系统 Shell 执行，因此请只安装和使用可信的插件文件：

```yaml
name: my-project
version: 1.0.0
detect:
  files:
    - my-config.json
  dirs:
    - src
commands:
  - name: 构建
    description: 构建应用程序
    run: npm run build
  - name: 测试
    run: npm test
  - name: 部署生产环境
    description: 将当前版本部署到生产环境
    run: ./scripts/deploy.sh
    risk: high
    confirm: true
env:
  - name: Node.js
    check: node --version
    required: true
```

命令元数据是可选的。如果命令不能通过一次按键直接执行，可以设置 `confirm: true` 或 `risk: high`。Proton 会将插件文件视为可信 Shell 配置，请在安装第三方插件前检查其内容。

`detect.files` 和 `detect.dirs` 支持 `*`、`?` 和 `[...]` 文件名通配符。

内置插件会根据检测到的项目进一步调整推荐命令：

- Node.js 读取 `package.json` 中的脚本，并通过锁文件识别 npm、pnpm、Yarn 或 Bun。
- Python 仅在对应入口、依赖或测试文件存在时推荐安装、运行和测试命令。
- Maven 读取 `pom.xml` 和本地父 Reactor，区分 JAR、WAR、多模块、Spring Boot、Spring Cloud、Quarkus 和 Exec 应用；Spring Boot 与 Spring Cloud 会显示不同的项目类型和命令名称，并优先使用 `mvnw` 或 `mvnw.cmd`。
- Maven 聚合器会递归发现声明 `spring-boot-maven-plugin` 的可运行模块，在根页面提供基于 `-pl` 的模块启动命令；默认激活的 Cloud Profile 会在普通 Boot 命令中自动停用。
- 存在 Gradle Wrapper 时优先使用 `gradlew` 或 `gradlew.bat`。
- C++ 根据实际构建文件选择 CMake 和/或 Meson 命令。
- Justfile 和 Taskfile 直接读取已声明的公开任务，并忽略私有或内部任务。
- SwiftUI iOS 项目会识别 Xcode Project、Workspace 和 Scheme，提供打开 Xcode、列出 Scheme、纯构建以及在 iOS Simulator 中安装并启动 App 的命令；项目包含测试 Target 时还可以自动选择可用模拟器运行测试。

## 许可证

[MIT](LICENSE)
