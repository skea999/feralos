# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: Skea — new Linux distribution based on Alpine Linux
**Focus**: Memory safety, security, modern components
**Status**: Installer development (Phase 1)

## Distribution Vision

Skea is a new Linux distribution based on Alpine Linux, focused on memory safety and security. It replaces traditional C components with modern alternatives (uutils, micro, doas). For software not in Alpine repos or requiring glibc, **Nix** is used. After the installer is stable, **Dinit** will replace OpenRC, and a **custom repository** will be created for Skea services.

## Workflow

**Chain** (complex): context/recall → interview/refine → @planner → approve → @codebase → PARALLEL(@review+@code-reviewer+@vulnerability-scan+@information-scan) → @debugger/tests → @docs/docs → save.

**Direct** (simple): @codebase or specialist direct.

State: `state/session-state.json`.

## Session

```powershell
# Start
.agents\skills\session-memory\scripts\memory.ps1 context
.agents\skills\session-memory\scripts\memory.ps1 recall "project" --limit 10
.agents\skills\session-memory\scripts\memory.ps1 stats

# End
.agents\skills\session-memory\scripts\memory.ps1 save "summary" "Today: X, next: Y" -Tags @("installer","go","skea")
```

### Final verification

```powershell
.agents\skills\session-memory\scripts\memory.ps1 context --days 14
.agents\skills\session-memory\scripts\memory.ps1 stats
.agents\skills\session-memory\scripts\memory.ps1 recall "project" --limit 5
```

## Git hooks (first session only)

```bash
git config core.hooksPath .githooks && chmod +x .githooks/pre-commit
```

Pre-commit: secrets scan + lint-staged (post-stack).

## Build & Test

```bash
# Build Go installer
cd src && go build -o ../alpine-installer .

# Build Go generator
cd src/generator && go build -o ../../generator .

# Run installer (requires root + live environment)
sudo ./alpine-installer -config variables.yaml

# Run generator (interactive or standalone)
./generator

# Run tests (when available)
cd src && go test ./...
```

## Lint (WSL / Native)

Development happens on Windows but lint/build runs on Linux. Use WSL on Windows, native on Linux.

### WSL Setup (one-time)

```bash
wsl -d Alpine -u root -- sh -c "apk add --no-cache go golangci-lint"
```

### Run Lint

**Windows (WSL):**
Convert the project path from Windows to WSL format (`C:\` → `/mnt/c/`, `\` → `/`), then:
```powershell
wsl -d Alpine -u root -- sh -c "cd <project-path>/src && GOFLAGS=-buildvcs=false golangci-lint run ./..."
```

**Linux (native):**
```bash
cd src && GOFLAGS=-buildvcs=false golangci-lint run ./...
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk`, `minSizeBytes` |
| Functions | camelCase (unexported), PascalCase (exported) | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |
| Constants | UPPER_SNAKE | — |

## Architecture Principle

**Standard core, pluggable data.**

- Installer logic never changes
- Storage layouts are Go packages
- Essential packages hardcoded in Go
- Optional packages + rules in YAML
- Parser always same code, only data changes

## Agents

### User-created
- `researcher`
- `information-scan`
- `vulnerability-scan`

## Skills

- `.opencode/skills/` — agents-opencode pkg
- `.agents/skills/` — skills.sh CLI

### User-modified (no auto-update)
- `session-memory`
- `using-agent-skills`

### User-created
None.

External skills:
- `customize-opencode` → built-in

## Docs

- `docs/PROJECT.md` — distribution vision, architecture, milestones
- `CONTEXT.md` — domain vocabulary (memory safety, Nix, Dinit)
- `ROADMAP.md` — milestones and pipeline
- `CHANGELOG.md` — version history

## Project Structure

```
skeaAlpineInstaller/
├── src/                          # Go installer (primary)
│   ├── main.go                   # Installer entry point
│   ├── go.mod                    # Go module definition
│   ├── variables.yaml            # User config (runtime)
│   ├── variables/
│   │   ├── variables.yaml        # Template with defaults
│   │   └── internalRules.yaml    # Dynamic rules
│   ├── installer/
│   │   ├── utility/              # Config, exec, files, logic
│   │   │   ├── config.go
│   │   │   ├── exec.go
│   │   │   ├── files.go
│   │   │   ├── logic.go
│   │   │   ├── make_variables.go
│   │   │   └── makeVariables/
│   │   │       └── main.go       # Standalone generator
│   │   ├── hostPhase/            # Host-side operations
│   │   │   ├── disks/            # Storage layouts (Go packages)
│   │   │   │   ├── btrfsStandardServer/
│   │   │   │   │   ├── partitioning.go
│   │   │   │   │   └── fstab.go
│   │   │   │   └── btrfsStandard/
│   │   │   │       ├── partitioning.go
│   │   │   │       └── fstab.go
│   │   │   ├── bootstrap.go
│   │   │   ├── bind_mounts.go
│   │   │   ├── chroot_preparation.go
│   │   │   └── hooks.go
│   │   └── chrootPhase/          # Chroot configuration
│   │       └── configurator.go
├── internal/                     # Shell scripts (reference)
│   ├── start.sh
│   ├── variables.sh
│   └── chroot.sh
├── docs/
├── state/
├── handoff/
└── tests/
```

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | Go installer: btrfsStandardServer | In Progress |
| Phase 1 | Go installer: btrfsStandard | Planned |
| Phase 1 | Generator (interactive + standalone) | Planned |
| Phase 1 | Test suite + CI | Planned |
| Phase 2 | Dinit init system | Planned |
| Phase 2 | Custom Skea repository | Planned |
| Phase 2 | Nix integration | Planned |
| Phase 2 | Full distribution release | Planned |

## Memory-Safe Alternatives

| Traditional (C) | Skea Alternative | Language |
|-----------------|------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | doas | C (minimal) |
| openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |

---

*Updated: 2026-07-23*
