# Proton Roadmap

## Product thesis

Proton is a zero-configuration task discovery and execution layer for unfamiliar repositories and polyglot monorepos. It should complement existing task runners and IDE configurations by discovering their tasks, exposing provenance, and presenting one interface to people and automation.

The TUI is one client of the task catalog. The catalog and non-interactive CLI are the product foundation.

## P0: Reliable foundation

- [x] Use one canonical repository and Go module path.
- [x] Support the actual minimum Go version.
- [x] Load `.proton.yaml` from the requested scan directory.
- [x] Resolve relative plugin paths from the declaring configuration file.
- [x] Reuse one discovery pipeline across the TUI and CLI.
- [x] Provide deterministic task IDs and command provenance.
- [x] Add `proton list`, `proton list --json`, `proton run`, and `--dry-run`.
- [x] Require `--yes` for high-risk non-interactive tasks.
- [x] Preserve deterministic ordering for manifest-defined tasks.
- [x] Verify tests, the race detector, vet, and all release targets.

## P1: Distribution and trust

- [x] Expose the task catalog and execution over an MCP server (`proton mcp`, stdio) with the same high-risk approval gate as the CLI.
- [x] Produce SHA-256 checksums for every release artifact.
- [x] Self-update from GitHub Releases (`proton update`) with SHA-256 verification, atomic replacement, and private-repo token support.
- [x] Build release binaries without CGO and with trimmed filesystem paths.
- [x] Verify modules and scan reachable vulnerabilities in CI.
- [ ] Sign and notarize macOS artifacts.
- [ ] Sign Windows artifacts.
- [ ] Publish Homebrew and Scoop packages; evaluate Winget after the name is finalized.
- [ ] Generate an SBOM and signed build provenance.
- [ ] Decide on a distinct, searchable product name before broad public launch.

## P1: Accuracy and performance

- [x] Run independent environment checks concurrently while preserving display order.
- [x] Track required tools per task so unrelated missing runtimes do not block execution in polyglot projects.
- [x] Include source, confidence, required tools, and risk in the JSON catalog.
- [x] Discover public tasks from Justfiles and Taskfiles.
- [ ] Discover tasks from mise configuration, Makefiles, and runnable README blocks before using inferred defaults.
- [ ] Add first-class uv, Poetry, PDM, Go workspace, CMake Presets, and Gradle task discovery.
- [ ] Label every command as declared, adapted, or inferred.
- [ ] Add configurable confidence thresholds and an option to hide inferred tasks.
- [ ] Cache scans with file-change invalidation for large monorepos.
- [ ] Version and document the plugin schema.

## P2: Quality benchmark

- [ ] Create a manifest of at least 30 real public repositories covering all supported ecosystems.
- [ ] Add expected project and task snapshots for each repository.
- [ ] Measure precision, recall, scan latency, and first-successful-command rate.
- [ ] Run the benchmark in CI without executing untrusted repository commands.
- [ ] Require at least 80% task precision before adding another ecosystem.

## P2: Four-week user validation

- Recruit 10–20 CLI-first developers who regularly open unfamiliar repositories or use coding agents.
- Measure time from clone to first successful task, correction rate, weekly reuse, and commands launched per active user.
- Interview users who stop using Proton and record the replacement workflow.
- Continue broad investment only if task precision is at least 80%, at least 30% of testers reuse Proton in week two, and at least three teams request continued use.

## Commercialization gate

Do not build paid features until the retention gate is met. If it is met, validate team-oriented capabilities such as shared task policy, trusted plugin distribution, onboarding catalogs, and audit history. Keep local discovery and execution open source.

## Explicit non-goals for the current phases

- Replacing full IDE run/debug configurations.
- Creating another general-purpose task definition language.
- Executing inferred deployment or destructive commands without explicit project configuration.
- Adding ecosystems before the existing adapters meet the benchmark threshold.
