---
name: proton-runner
description: Use this skill when an AI coding agent needs to start, inspect, test, build, or run an unfamiliar local project by using the already-built or installed proton CLI. This skill explains how Codex, Claude Code, and similar agents should use proton as a project task launcher without needing to understand the target project's build system first.
---

# Proton Runner

## Purpose

Proton discovers runnable tasks in local repositories and polyglot monorepos. Use it as the first command-discovery layer before guessing how to start, test, build, or lint an unfamiliar project.

## Prefer MCP When Available

If an MCP client connects a `proton` server (command `proton`, args `["mcp"]`), use its tools instead of the CLI — they return structured results without text parsing:

1. `list_tasks` — the full task catalog with stable IDs (same schema as `proton list --json`).
2. `run_task` with `dry_run: true` — review the command, directory, risk, and required tools.
3. `run_task` — execute; pass `approved: true` only for high-risk/confirm tasks after the user explicitly agrees, and `timeout_seconds` (default 120) to bound long-running commands. Long-running dev servers are killed at the timeout with their captured output returned.
4. `check_env` — verify required tools before running.

Everything below applies when no MCP server is available.

## Find Proton

Prefer the installed executable:

```bash
command -v proton
proton --version
```

If `proton` is not on `PATH`, check for a local built binary near the current repository:

```bash
test -x ./build/proton && ./build/proton --version
```

On Windows:

```powershell
Get-Command proton
proton --version
```

If a local Windows build exists:

```powershell
.\build\proton.exe --version
```

Use `proton` in examples below. If only a local binary exists, replace `proton` with `./build/proton` or `.\build\proton.exe`.

## Discover Tasks

Run Proton from the project root, or pass the target project directory explicitly.

Human-friendly task list:

```bash
proton list .
```

Machine-readable task catalog for agents:

```bash
proton list --json .
```

For large monorepos, limit scan depth when useful:

```bash
proton list --json --depth 1 .
proton list --json --no-recursive .
```

The JSON output includes a `schema_version`, the scanned root, and detected tasks. Use each task's exact `id` when executing; do not guess or partially match task IDs.

## Run Tasks Safely

Always inspect a task before running it when the command is not already obvious:

```bash
proton run --dry-run <task-id> .
```

Run the selected task:

```bash
proton run <task-id> .
```

Tasks marked `risk: high` or `confirm: true` require explicit approval:

```bash
proton run --yes <task-id> .
```

Do not use `--yes` unless the user explicitly asked to run the command and you have reviewed the dry-run output.

## Interactive Mode

For a human-operated terminal UI:

```bash
proton .
```

Use interactive mode when the user wants to choose a command manually. For autonomous agent workflows, prefer the MCP tools, or `proton list --json` plus `proton run --dry-run` without MCP.

## Recommended Agent Workflow

1. From the target repository root, run `proton list --json .`.
2. Parse the task catalog and identify the task matching the user's goal, such as dev server, test, build, lint, format, or install.
3. Run `proton run --dry-run <task-id> .` and review the command, directory, source, required tools, and risk metadata.
4. If the task is low risk and matches the user's request, run `proton run <task-id> .`.
5. If the task requires confirmation or has high risk, ask the user before running it with `--yes`.
6. If no suitable task appears, report that Proton did not detect one and fall back to inspecting the repository manually.

## Common Goals

Use detected task names and metadata rather than hard-coded assumptions. Typical matches include:

- Start or develop: choose tasks named like `dev`, `start`, `serve`, `run`, `boot`, or framework-specific run commands.
- Test: choose tasks named like `test`, `unit test`, `integration test`, or `run tests`.
- Build: choose tasks named like `build`, `package`, `compile`, or framework-specific build commands.
- Lint or check: choose tasks named like `lint`, `check`, `vet`, `clippy`, or `typecheck`.
- Install dependencies: choose tasks named like `install`, `deps`, or package-manager install commands.

If several tasks look plausible, prefer the narrowest task that satisfies the user's request.

## Reporting

- Say whether Proton was found on `PATH` or via a local binary.
- Mention the selected task ID and the command shown by `--dry-run`.
- If execution fails because a required tool is missing, report the missing tool and stop.
- If Proton detects no suitable task, say so clearly before using manual repository inspection.
