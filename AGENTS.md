# Project Context

**Stack**: Go 1.22 (installer) + Shell (legacy reference) + YAML config
**Project**: FeralOS — Alpine-based distribution, memory safety + security, Nix for glibc, Dinit selectable (OpenRC fallback), custom repo
**Status**: Phase 1 refining · Phase 2 shipping (niriWayle flagship desktop, Dinit selectable)

> **Primary context for AI agents.** This file keeps full project info. Detailed copies live in `docs/` (double documentation). Read this file first, then follow `→` pointers for deep dives.
## Work history

All per-round work summaries (formerly "Recent Rounds") live in [CHANGELOG.md](CHANGELOG.md) — see the "Session work log" section at the end. Keep new round summaries there; this file stays lean context.

## Workflow

**Chain (complex)**: `context/recall` → `interview/refine` → `@planner` → *approve* → `@codebase` → PARALLEL(`@review`+`@code-reviewer`+`@vulnerability-scan`+`@information-scan`) → `@debugger/tests` → `@docs/docs` → save
**Direct (simple)**: `@codebase` or specialist
State: `state/session-state.json` (**primary** — full working state, read by the agents-opencode plugin context hook; write all future updates HERE) | Handoff: `handoff/latest.md`

State-file protection: the agents-opencode installer writes `state/session-state.json` only when missing (write-once-if-missing, code-proven + live-tested 2026-11-13) — it never overwrites. Git tracks the file: any accidental change is visible as a diff and restorable with `git checkout -- state/session-state.json`.

**Installer Workflow**: Understand Requirements (`CONTEXT.md`, `ROADMAP.md`, shell ref) → Plan (`hostPhase`/`chrootPhase` modules) → Implement (Go, naming conventions) → Test locally → Test harness (QEMU) → Review → Commit (conventional)

Key Decisions: `hostPhase` writes files (NO chroot), `chrootPhase` runs commands, module isolation, data-driven YAML/Go, security first (LUKS2, Secure Boot, AppArmor)

→ Details: `docs/guides/development.md` (how to code / separation), `docs/INSTALLER.md` (flow)

## Documentation Rule (MANDATORY)

**Every code change MUST update documentation.** After you change code, update `AGENTS.md` (primary context, keeps full info) **and** the matching `docs/` copy (architecture/adr/guides/harness/intent/ideas) to keep double documentation in sync. No code without doc update. `AGENTS.md` → `docs/` is single source duplication; keep both identical in substance. Update `Docs Index` pointers if you add/move files. Harness docs → `docs/harness/` (canonical) — `tests/harness/README.md` stays pointer only.

**ADRs:** If you make an architectural decision (new pattern, isolation rule, storage layout, boot method, logging, compatibility, etc.) and no ADR exists, **write one** in `docs/adr/ADR-XXX-title.md` (next number) + update `docs/adr/README.md` index. Template: Status / Date / Context / Decision / Consequences. No decision without ADR.

→ Details: `docs/architecture/`, `docs/adr/`, `docs/guides/`, `docs/harness/`, `docs/intent/`, `docs/ideas/`

## Session Memory

Storage `.agent-memory/`, scripts `.agents/skills/session-memory/scripts/`

**The memory must be pushed.** `.agent-memory/` is tracked in git — after saving, commit and push it with the session's changes.

```bash
.agents/skills/session-memory/scripts/memory.sh context --days 14
.agents/skills/session-memory/scripts/memory.sh recall "vector" --limit 5
.agents/skills/session-memory/scripts/memory.sh stats
.agents/skills/session-memory/scripts/memory.sh save "decision" "Chose X because Y" tag1 tag2
MEMORY_IMPORTANCE=high .agents/skills/session-memory/scripts/memory.sh save "insight" "Key finding" installer
.agents/skills/session-memory/scripts/memory.sh save "summary" "Today: implemented X. Next: Z" -Tags installer go feralos
```

## Build & Test

```bash
# Build Go installer
cd src && go build -o ../feralos_installer .

# Build Go generator
cd src/utils/createConfig/createValues && go build -o ../../../../feralos_create_config .

# Build first-boot programs
cd src && go build -o ../feralos_secureboot ./firstBoot/secureboot/
cd src && go build -o ../feralos_tpm ./firstBoot/tpm/

# Build harness
cd tests/harness && go build -o skea_harness ./cmd/skea_harness/

# Run installer (requires root + live)
sudo ./feralos_installer -config variables.yaml

# Run generator
./feralos_create_config

# Run harness tests
cd tests/harness
./skea_harness prepare-runner
./skea_harness run -f ~/Progetti/feralos/src/variables/variables.yaml
./skea_harness status
./skea_harness cleanup

# Run harness with fixture templates (covers all scenarios)
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_minimal.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_raid0_noswap.yaml
./skea_harness run -f tests/fixtures/variables/btrfs_standard_swap_explicit_8g.yaml
ls tests/fixtures/variables/  # 30 fixtures, full variables.yaml template mirror (all scenarios)

# Run tests (when available)
cd src && go test ./...
```

→ Details: `docs/guides/development.md` (build/test/lint + conventions), `docs/harness/commands.md` (CLI), `docs/harness/workflow.md` (3-phase flow)

## Test Artifacts

Output to `tests/artifacts/<tool-name>/<timestamp>/`:

```text
tests/artifacts/
├── feralos_create_config/
│   └── .gitkeep
└── harness/
    ├── cache/ (base-images, keys, runner-images, web/alpine)
    ├── prepare/ (logs)
    ├── runs/ (per-install logs + disks)
    └── state/ (active.json)
```

Rules: gitignored `tests/artifacts/**/*` except `.gitkeep`, never commit outputs, harness keeps last 2 runs.

→ Details: `docs/harness/artifacts.md` (where generated, gitignored, how to inspect)

## Lint (Native Linux)

```bash
cd src && go vet ./...
cd src && golangci-lint run ./...       # includes dupl (code clone detection)
cd tests/harness && go vet ./...
cd tests/harness && golangci-lint run ./...
```

**Dupl exclusions** (intentional duplication): `src/hostPhase/disks/` (layout packages), `src/chrootPhase/desktops/` (derived desktop flows — niri.go/niri_wayle.go are intentionally duplicated, same policy as disk layouts: do NOT merge or extract shared blocks), `src/utils/firstboot/` (standalone binaries). Config: `.golangci.yml` → `dupl.threshold: 100` + `exclude-rules`. Linter set also carries `wastedassign` + `nakedret` (both clean); `revive` is DISABLED — its var-naming rule flags every MixedCaps package name (`hostPhase`, `chrootPhase`, `btrfsStandard`, `createConfig`…), which is the documented project convention ("Folders camelCase"), so its findings are false positives (rationale comment in `.golangci.yml`). Full polish task history is in the git log.

**Static analysis suite (ursine pattern)** — configs at repo root: `.ecrc`, `.gitleaks.toml`, `.gitleaksignore`, `.markdownlint-cli2.jsonc`, `.shellcheckrc`, `typos.toml`, `.yamllint.yml`, `taplo.toml`; wired into `scripts/ci.sh` after the existing checks; tool versions pinned in `Dockerfile` lint-tools (gitleaks 8.30.1, typos 1.51.1, yamllint 1.38.0, markdownlint-cli2 0.24.0 + Node 22.23.4, editorconfig-checker 4.0.2; taplo optional/not in image):
- **blocking**: `yamllint -c .yamllint.yml` over every `*.yaml`/`*.yml` in the tree except artifacts/vendor dirs (broken YAML breaks the installer); `gitleaks detect --source . --config .gitleaks.toml --no-banner --redact` (git checkout = full commit history; known historical findings fingerprinted in `.gitleaksignore`; the docker image ships no `.git` → ci.sh falls back to `gitleaks dir .` working-tree scan; the old pattern-grep scan stays as the fallback for hosts without gitleaks)
- **warn-only**: `typos --config typos.toml --format brief .`, `markdownlint-cli2` (globs/ignores in `.markdownlint-cli2.jsonc`), `ec --config .ecrc .`, `taplo lint --config taplo.toml`
- `.editorconfig` exceptions: `*.md` keeps trailing whitespace + `indent_size = unset`; `*.go` indent rules unset — gofmt is the sole Go indentation authority (editorconfig-checker cannot tell code from space-indented raw-string content)

→ Details: `docs/guides/development.md` (lint/format)

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Files | snake_case | `bind_mounts.go`, `config.go` |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` |
| Variables | camelCase | `targetDisk` |
| Functions | camelCase/PascalCase | `getMinDiskSize()`, `RunBtrfsSetup()` |
| Types | PascalCase | `Config`, `Subvolume` |

→ Details: `docs/guides/development.md` (module map + conventions)

## Architecture Principle

Standard core, pluggable data — installer logic never changes, storage layouts are Go packages, essential packages hardcoded, optional+rules in YAML, parser same code only data changes, booster replaces mkinitfs, UKI replaces GRUB, LUKS2 on RAID/Swap, Btrfs RAID1 on LUKS. **No backward compatibility — installer runs once, always fresh.**

→ Details: `docs/architecture/overview.md` (principle + tree + decisions), `docs/adr/ADR-001-go-installer.md`, `ADR-005-luks-btrfs-booster-uki.md`, `ADR-006-no-backward-compat.md`

## Entry Point & Phase Isolation

`main.go` is entry point ONLY. It collects/generates config then delegates:

- `cleanup.Cleanup(conf)` — pre-host cleanup (swapoff, unmount, LUKS close)
- `hostPhase.Run(conf)` — all host-side installation
- `chrootPhase.Run(conf)` — all chroot-side installation
- `firstboot.WriteInstallConfig(conf)` — reads cmdline, writes `/opt/feralos/config.yaml`
- `firstboot.SetupSecureboot(conf)` — compile → copy → chmod 755
- `firstboot.SetupTpm(conf)` — compile → copy → chmod 755
- `cleanup.Cleanup(conf)` — final unmount before reboot

`main` must NOT call disk layouts or chroot commands directly.

**Internal package rule**: `hostPhase/internal/*` is importable ONLY by `hostPhase` (and its subpackages). `main` cannot import it. This is the structural guarantee of host/chroot isolation.

**`hostPhase.Run()` fixed order**:

1. Disk layout switch (`hostPhase/disks/*`) by `conf.User.DiskLayout`
2. Mount virtual FS (`internal/bind_mounts.go`)
3. Alpine bootstrap (`internal/bootstrap.go`)
4. Chroot prep (`internal/chroot_env.go`)

Cleanup (before/after) and firstboot (install config, secureboot/tpm compile) are outside `hostPhase.Run()` — called directly by `main`.

→ Details: `docs/architecture/isolation.md` (entry-point + internal rule + order), `docs/INSTALLER.md#installation-flow`, `docs/adr/ADR-002-host-chroot-isolation.md`, `ADR-003-internal-package-rule.md`

## Logging Rule

**Commands: `shell.ExecuteCmd`; file writes: `shell.WriteFile`/`shell.WriteFileAppend` — in `hostPhase`/`chrootPhase`, never raw `os.*`.**

- **Commands** (installs, chmod, sed, chown, anything executed): `shell.ExecuteCmd` — logs CMD, stdout/stderr, exit, retry, `SKEA_NONINTERACTIVE` handling. Host: `shell.ExecuteCmd("", "mkdir -p /mnt/etc")`. Chroot: `shell.ExecuteCmd(chroot, "mkdir -p /etc")`.
- **File writes** (`shell.WriteFile(chroot, path, content, perm)` / `WriteFileAppend`, `src/utils/shell/write.go`): logged direct writes on the chroot mount — `[WRITE]`/`[APPEND]` line (path + size, content NEVER logged) + `[RESULT]`; MkdirAll of the parent (0755); perm forced via chmod on create (umask-independent); honors `SKEA_IGNORE_ERRORS=1` like ExecuteCmd. **The `echo '<escaped>' > path` shell pattern is forbidden for new code**: single-quote escaping of arbitrary content is fragile (one missed `'` corrupts the seed) and very large payloads can hit ARG_MAX — the escaped-echo call sites were converted to `WriteFile`. KEEP `ExecuteCmd` for: conditional appends (`grep -q … || echo … >> file` — the guard is part of the operation), writes computed at shell-runtime (e.g. awk over `/proc/meminfo`), and in-place edits (sed/awk). `WriteFileAppend` covers unconditional appends.
- `import "os"` forbidden in `hostPhase`/`chrootPhase` (exceptions: `firstBoot/` standalone, `utils/shell` impl).
- Documented exceptions (secrets must never be logged): passwd via stdin pipe (chrootPhase/internal/users.go) and cryptsetup stdin (hostPhase/disks/common/luks.go).

→ Details: `docs/architecture/logging.md`, `docs/adr/ADR-004-executecmd-logging.md`, `src/utils/shell/execute.go`, `src/utils/shell/write.go`

## Error Handling (`unattended` vs `ignoreErrors`)

**`unattended: true`** skips manual input prompts ONLY (wizard skip; unchanged semantics). **`ignoreErrors: true`** (default `false`) = best-effort install: command failures are logged (`[ERROR][IGNORED] command failed (ignoreErrors=true): ...`) and installation CONTINUES. Wiring: `LoadConfig` sets `SKEA_IGNORE_ERRORS=1` when `conf.User.IgnoreErrors`; `shell.ExecuteCmd`/`ExecuteCmdArgs` honor it after retries are exhausted (return nil error → callers continue). Used by the test harness: harness runs are ALWAYS best-effort (`LoadVariablesFile` forces `cfg.IgnoreErrors = true`, `writeHarnessVariables` writes `ignoreErrors: true`, all fixtures carry `ignoreErrors: true`) — a failing optional step must not abort the run; the verification phase judges what actually works.

→ Details: `docs/guides/variables.md` (Options table), `src/utils/config/config.go` (env wiring), `src/utils/shell/execute.go` (ignore path)

## Activatable Feature Confinement

**Rule: Every activatable feature MUST be confined to a single owner file.**

Each `Enable*` flag (and `DesktopEnvironment`) is owned by exactly ONE file under `src/chrootPhase/features/` or `src/utils/firstboot/` or `src/chrootPhase/desktops/`. All logic — packages, configuration, guards — lives in that file.

**Callers must NOT contain feature logic:**

- `packages.go` → calls `features.XxxPackages(conf)`, never contains inline feature packages
- `logic.go` → zero feature-specific logic, only generic rule application
- `chroot_phase.go` → calls `features.SetupXxx(conf)`, never contains feature configuration
- `host_phase.go` → calls `features.SetupXxx(conf)`, never contains feature compilation

**Pattern for new features:**

1. Create `src/chrootPhase/features/<name>.go` (or `src/utils/firstboot/` for compile+copy)
2. Export `XxxPackages(conf) []string` — returns APK packages needed
3. Export `SetupXxx(conf) error` — configures the feature inside chroot
4. Guard: `if !conf.User.EnableXxx { return nil }`
5. Call from `packages.go` (packages) and `chroot_phase.go` or `host_phase.go` (setup)
6. NEVER mutate flags from outside the feature file (exception: `NixPackages()` auto-enables internally)

**Current confinement map:**

| Feature | Owner file | Called from |
|---------|-----------|-------------|
| `EnableSecureboot` | `utils/firstboot/secureboot.go` | `main.go` |
| `EnableTPM` | `utils/firstboot/tpm.go` | `main.go` |
| `EnablePodman` | `chrootPhase/features/podman.go` | `packages.go` + `chroot_phase.go` |
| `EnableHardening` | `chrootPhase/features/hardening.go` | `packages.go` + `chroot_phase.go` |
| `EnableNix` | `chrootPhase/features/nix.go` | `packages.go` + `chroot_phase.go` |
| `EnableZswap` | `chrootPhase/features/zswap.go` + `zswap_server.go` | `packages.go` + `chroot_phase.go` |
| `EnableEdgeRepos` | `chrootPhase/features/edgerepos.go` | `chroot_phase.go` (step 17, chroot tail — immediately after `SetupDbusAudit`, never inside `SetupPackages`) |
| `EnableDebug` | `chrootPhase/features/hardening.go` | via `HardeningPackages()` |
| `EnableSwap` | `hostPhase/disks/*` | `host_phase.go` (disk layout layer) |
| `DesktopEnvironment` | `chrootPhase/desktops/desktops.go` | `chroot_phase.go` |

**User-session daemons = supervised services (Artix/Chimera model, user rule 2026-10-16):** EVERY user-session daemon runs as a supervised per-user dinit service (`.user`, started by the turnstile-spawned user dinit) — NOTHING is left to unsupervised D-Bus activation, autostart or daemon self-spawn. If Alpine lacks the `-user-dinit` package, one is created in feralos-aports (e.g. `oo7-user-dinit`, `xdg-desktop-portal-user-dinit`). Applies to BOTH inits (OpenRC gets the identical user-dinit stack via turnstile).

**Desktop environments supported:**
- `niri` — custom config (greetd, regreet, niri config, DMS, xdg-su) in `chrootPhase/desktops/niri.go`; session wiring = turnstile (PAM → user dinit) + `niri-session` via login-shell trick (`-s /usr/libexec/login-shell`, not in `/etc/shells`, symlink re-pointed to fish — busybox applet dispatch breaks the package's `/bin/ash` target; user `chsh` skipped) + `niri`/`niri-shutdown` user services (`niri-user-dinit`); installer enables `/etc/dinit.d/boot.d/turnstiled` + user `boot.d` stack in `/usr/lib/dinit.d/user/boot.d/`
  - Session startup: real path = greetd/regreet → `niri-session` (fixed dinit branch shipped by `niri-user-dinit`, `replaces="niri"`: full login-env setenv + dinit-monitor wait — upstream post-25.11 mechanism). greetd autologin exists ONLY as a harness test fixture (simulates the real login); the installer never writes `[initial_session]`. Harness = real-life simulation; installer = the product; harness-discovered fixes go to installer/aports, never the harness. Admin GUIs are `ursine` panels; `kcron`/`ksystemlog`/`gnome-disk-utility`/`firewalld-gui` are not installed (Flatseal stays — Ursine does not edit Flatpak session-bus policies).
- `niriWayle` — **the flagship/recommended desktop** (the only DE developed and verified as a complete daily-driver experience; the others ship for user convenience): custom config in `chrootPhase/desktops/niri_wayle.go` (niri.go derivation minus DMS), Wayle shell + Veila + Raffi + oo7; launcher TOGGLES via `raffi-toggle` (feralos-aports — `/usr/bin/raffi-toggle`: close the running raffi window via niri IPC or spawn a new detached instance via setsid; wired into the Wayle bar custom module `left-click` + the niri Mod+D bind; one-way `raffi-toggle → raffi` dep — the raffi→raffi-toggle direction would be an unbuildable build-order cycle, the installer lists both packages explicitly; abuild forbids `/usr/local` in packages — the user spec's `/usr/local/bin` path corrected to `/usr/bin` during implementation, CI run 41153877924); **window switcher** Mod+W + bar window-list module (`[[modules.custom]] id = "windows"`, niri IPC watcher, `left-click = "raffi -i 'win '"` — raffi 0.26.0 initial query; harness `wayle_session_actions` asserts both); Key Rack secrets GUI via Flathub flatpak (`app.drey.KeyRack`, NOT an apk — seahorse conflicts with oo7-server on org.freedesktop.secrets.service), portals Secret interface pinned to `oo7-portal`; oo7 login auto-unlock via PAM (`oo7_pam_unlock` harness check — authenticated PAM proof); same turnstile/login-shell session wiring. Admin GUIs are `ursine` panels; `kcron`/`ksystemlog`/`gnome-disk-utility`/`firewalld-gui` are not installed (Flatseal stays — Ursine does not edit Flatpak session-bus policies). Daily-driver apps (2026-11-10): `evince`, `file-roller`, `thunderbird`, `gnome-maps`, `gnome-firmware`, `obs-studio`, `gst-libav` (H.265), `font-noto-emoji`, `dconf`, `xdg-desktop-portal-gtk` (the `default=gtk;` generic portal backend — rules list, see the 2026-11-10 round); the gnome portal backend is shadowed by an empty 999 aports dummy (`xdg-desktop-portal-gnome` — nautilus never installs; the installer seeds `default=gtk;` + wlr screenshot/screencast into the user portals conf); Ursine 0.27 is D-Bus-free (`depends="parted shadow util-linux"` — accountsservice/udisks2 gone from the install); supervision for the new activation targets = `file-roller-user-dinit` + `evince-user-dinit`. Wayle session actions wired (installer seeds `~/.config/wayle/config.toml` — the dashboard button (far right) opens the dashboard dropdown whose lock/logout/reboot/poweroff buttons run FeralOS commands; `left-click = "dropdown:dashboard"` pinned explicitly (upstream default equals it — layered config keeps per-field defaults on partial tables, verified live incl. a real VNC click opening the panel, run `2026-10-28T00-10-10Z`); reboot via the `zz-power` NOPASSWD path; harness `wayle_session_actions`); `stasis` context-aware idle manager (supervised `stasis.user`; FeralOS system config `/etc/stasis/stasis.rune` via `-c` — veila lock → niri dpms → loginctl suspend; harness `stasis_idle`); veilad runs through the `veilad-session` env wrapper (harness `veila_daemon`); greeter session uniqueness — the installer appends `NoDisplay=true` to `/usr/share/wayland-sessions/niri.desktop` on niriWayle installs so regreet lists ONLY the niriWayle session (real-hw finding: the upstream bare `niri` session showed next to it; ReGreet 0.2.0 respects the Desktop Entry Hidden/NoDisplay fields; apk-safe — survives niri upgrades as `.apk-new`; harness `niriwayle_session_unique`). Ships the **full niri app set** (browser/office/utilities + printing/scanning support + the niri flatpaks alongside Key Rack) — user request 2026-10-19; DMS shell packages and seahorse/gnome-keyring are intentionally not copied. **Branding** (2026-11-01): installer installs the FeralOS logo as a system hicolor icon (`/usr/share/icons/hicolor/scalable/apps/feralos-logo.svg` — the Wayle launcher custom module carries `icon-name = "feralos-logo"`; Adwaita inherits hicolor) + default wallpapers from the tracked repo-root `branding/` dir (`/usr/share/wallpapers/feralos-default.png` 1080p = the regreet background + `feralos-default-3440x1440.png` ultrawide) via `desktops/branding.go` (multi-candidate repo-path lookup, same pattern as createValues templatePaths; new logged `shell.ReadFile` host-side read counterpart of WriteFile) — shared by niri.go (regreet path was a dangling `/usr/share/wallpapers/default.png`, now real); missing assets = non-fatal WARNING. README top carries the logo.
- `kde` — packages only via `internal_rules.yaml`, no custom config
- `cosmic` — packages only via `internal_rules.yaml` (uses seatd, not elogind), no custom config

**Dinit services** ship `/etc/dinit.d/config/<svc>.conf` (Artix pattern, ADR-008): service → `/usr/lib/dinit/<svc>` wrapper → sources config → exec daemon. User-editable, apk-protected; service conventions per ADR-009 + Amendment 2026-11-08 (restart = on-failure default, smooth-recovery only on resident daemons, before=login.target, pre-network ordering, dbus-wait-for readiness).

→ Details: `docs/architecture/feature_confinement.md`, `docs/guides/development.md` (how to add feature)

## Rules Tunables (stage-2 rules-ification)

**The installer is tunable by editing ONLY `internal_rules.yaml`; the mechanism stays Go.** Hardcoded Go data moved into the rules file (2026-11-07): `standardUserGroups`, `dinitSkipSessionManaged` (packages.go conversion skips), `feralosRepo.keyringUrl` (repo bootstrap), `kernel.{package,flavor}` (hooks.go EFI-sync target + uki.go `vmlinuz-<flavor>`/`booster-<flavor>.img`/`linux-<flavor>.efi` naming), `microcode.{amd,intel}.{package,image}` (packages/boot/uki), `boosterConfig.{compression,extraFiles,mountTimeout,network.dhcp}` (/etc/booster.yaml — network mirrors booster.yaml's own sub-block shape; boot.go consumer renders dhcp false→"off"/true→"on", byte-identical for the shipped values), `edgeRepositories` (@edge repo lines), `resolvNameservers` (chroot resolv.conf), `maintenanceCron.{apkAudit,trim,scrub,snapshots}` (crontab schedules), `limits.{nofileDefault,nofileUser}` (limits.conf), `nix.{channel,registryRef}` (stable pins), and per-DE `userServices` (user dinit boot.d infra units — VERBATIM `.user` names, joined into the shell loop) + `greeterGroups` (niri/niriWayle greeter `usermod -aG`). `LoadInternalRules` VALIDATES the always-consumed ones (kernel, keyringUrl, resolvNameservers, maintenanceCron, limits, boosterConfig compression/extraFiles/mountTimeout, standardUserGroups, dinitSkipSessionManaged — empty = load error); edge/nix/microcode are feature-gated (SetupEdgeRepos/SetupNix enforce their own contracts; empty microcode = none). Guard `TestRulesCarryRequiredTunables` pins the full set + `turnstiled` in the niri/niriWayle services maps + the packaged ADR-010 binaries in standardPackages. Desktop misplacements fixed behavior-preserving: login-shell trick lives in `desktops/niri.go`/`niri_wayle.go` step 4b (users.go is DE-agnostic — generic fish+chsh path runs for ALL DEs), turnstiled enablement is driven by the DE `services` map via services.go (its `/etc/dinit.d` fallback covers the turnstiled unit), `ensureIncusAdminGroup` + the `usermod -aG audio` duplicates removed (setupUserGroups creates missing rules groups; rules `groups` already carry audio), the `/var/log/ursine` belt-and-braces mkdir lives in misc.go (`setupUrsineLogDir`), and `AppendUnique` is exported from utils/logic (single shared copy). Generated output is BYTE-IDENTICAL for every DE (diff-proven per consumer).

## D-Bus Activation Rule (ADR-010)

**Rule: every D-Bus-activatable daemon is a dinit service; activation only triggers dinit.** No exceptions, no per-application special cases (user directive 2026-10-18; decision `docs/adr/ADR-010-dbus-activation-dinit.md`, mechanism + checklist `docs/architecture/dbus-activation.md`).

- **Unit name follows WHO activates the service in FeralOS (R1, amended 2026-10-25):** user (session) services = canonical bus name + `.user` (`/etc/dinit.d/user/`) — dbus/turnstile-activated on demand, never boot-enabled by us; system services WE boot-enable (an OpenRC initd exists) = exactly the OpenRC initd name (`/lib/dinit.d/<initd-name>`) — the `services` map alone drives both inits (rc-update + boot.d symlink), no corrective mapping, and `dinitServices` stays EMPTY (mechanism kept for exceptional cases only); system services dbus-activated on demand (never boot-enabled — dbus decides when they run) = canonical bus name (`/lib/dinit.d/<Name>`) — fwupd, PPD, upower, udisks2, avahi, wpa_supplicant, nm_dispatcher, accountsservice, bolt, cups-pk-helper, flatpak SystemHelper, fprintd, geoclue, kcron, the Plasma KAuth helpers. fwupd/PPD were boot-enabled before the amendment — corrected to activation-only on both inits. Aliases (e.g. `net.hadess.PowerProfiles` → `org.freedesktop.UPower.PowerProfiles`) are separate activation files mapping onto the same canonical unit.
- Bus names stay on the D-Bus side in a rename: override filenames + `Name=`, `dbus-wait-for -n` arguments, polkit action ids never change — the override `Exec=` aliases the bus name onto the unit whatever it is called.
- The effective `.service` file carries `Exec=/usr/libexec/dinit/dbus-activate <unit> -- <upstream exec>`; overrides ship in `/usr/share/dinit/dbus-system-services` (system) + `/usr/share/dinit/dbus-services` (session) and are materialized into the effective servicedirs: `/usr/local/share/dbus-1/system-services` at boot via `dbus-services-setup` (the system bus activates through the setuid launch helper, which resolves names ONLY against the standard dirs — `/etc/dbus-1/system-services` is daemon-enumerated but NOT helper-activatable on dbus 1.16.2) and `$XDG_RUNTIME_DIR/dbus-1/services` at login via the `dbus-session` wrapper; upstream files untouched. (Shim + override staging dirs FHS-migrated 2026-11-08: `/usr/libexec/dinit/` + `/usr/share/dinit/` — aports side ships both locations, stage-2 commit.)
- Shim: forwards the activation environment to dinit (single `dinitctl setenv`) then `dinitctl start` (waits) and exits; OpenRC system-bus fallback = upstream exec when no system dinit; session bus always user dinit (both inits).
- Enforcement: harness `dbus_activation_uniform` (override sources + boot materialization + unit exists; fails if anything is left in the helper-ineligible `/etc/dbus-1/system-services`) + `dbus_activation_env` (real activation proof: unit started with no `DBUS_STARTER_*` in the daemon environ, plus `setenv` forwarding via a direct shim marker run) + `dbus_activation_migration` (forward-looking retirement gate: enumerates every activation file live, fails only when dbus-broker is installed AND every target is unit-covered) + `dbus_system_activation` (system-bus coverage on EVERY DE, dinit-only).
- Adding a new activatable service: follow the checklist in `docs/architecture/dbus-activation.md` (aports mirror: `DBUS-ACTIVATION.md` in `feralos-aports`); the same doc carries the verification/troubleshooting commands (`ListActivatableNames` ≠ activatable; the `DBUS_STARTER_*` check; raw-spawn diagnosis).
- **Supervision package name = the daemon's apk package, NEVER the bus name (R1b, 2026-11-03):** `<owner-of-Exec>-user-dinit` (session) / `<owner-of-Exec>-dinit` (system dbus-only); boot-enabled initd services keep `<initd>-dinit`; non-activatable infra keeps its service name. Rationale: a bus name is an API identity, not a daemon identity — two daemons can claim one name (`org.freedesktop.secrets` = gnome-keyring on niri, oo7 on niriWayle) and one daemon can serve many activation files (plasma-workspace: 6, kio/kio5: 4 each). Resolution is mechanical: `apk info --who-owns <Exec path>`.
- **Supervision packages are NEVER listed in `internal_rules.yaml`** — `feralos-dbus-audit` installs them (see below). Go-test guard `TestInternalRulesCarryNoSupervisionPackages` (`src/utils/config/rules_guard_test.go`) enforces the allowlist (only non-activatable infra: dbus/pipewire×3/wayland-wait user units, niri-user-dinit, polkit-agent/wayvnc, dbus-session-materializer, and the override-only kglobalaccel/cosmic-applets — no upstream activation file, audit-blind).
- Enforcement: `feralos-dbus-audit` (`/usr/libexec/dinit/feralos-dbus-audit`) — install-time AUTO-INSTALLER, invoked TWICE (2026-11-11): **pass 1 = step 8a** (before the desktop setup — its apk fallback puts the supervision packages on disk so the desktop's user boot.d linking loop can link them; its boot.d WARN lines = honest not-yet-linked state) and **pass 2 = step 16**, the chroot TAIL immediately BEFORE the `@edge` install at step 17 (the edge step disables the repos and would break the audit's `apk add`; the boot.d section reports the FINAL state — any WARN = genuine regression) + boot-time oneshot (`--log-file /var/log/feralos-dbus-audit.log`, both inits, `local.target`-ordered) + CLI; scans BOTH activation inventories (session `/usr/share/dbus-1/services` + system `/usr/share/dbus-1/system-services` — same enumeration as the harness `dbus_activation_migration` check), verifies units (both naming conventions + override-aliased units, e.g. login1→elogind) and `apk add`s the supervision package per target (candidates: who-owns → bus name/providers), MISS + exit 1 = aports gap (installer: WARNING only). Details: `docs/architecture/dbus-activation.md` "Supervision audit" section.
- `dbus-services-setup` is the reference boot-time oneshot (both inits, ordered before a consumer) — reusable pattern + gotchas: `docs/architecture/boot-oneshots.md`.

## aports Source Pinning

**Rule (2026-11-08, user decision): every GitHub source in `feralos-aports` pins to a TAG archive (`archive/refs/tags/...`), NEVER a commit archive (`archive/<sha>...`).** The sha512sum is the integrity guard; the tag is the version contract — if upstream re-publishes a tag to fix a bug/CVE, the checksum FAILS = the signal to diff the new content and port the fix. Commit pins would silently keep old bytes forever; tags give version semantics + fail-loud. On a sum break: fetch the new archive → top-dir-normalized `diff -r` → legit fix = new sum + pkgrel bump + commit note; unexpected/hostile = STOP and report. Our own repos (`skea999/feralos` — e.g. tag `dbus-audit-0.1.0` for `feralos-dbus-audit`, `skea999/ursine`, `skea999/dms-window-buttons`) cut a tag per package-relevant source state instead of commit pins. Inventory 2026-11-08: 17 archive sources over 16 packages, all tag archives (dms-shell reverted r5 commit-pin → tag `v1.6.2` + `dank-qml-common` tag `v1.6.0`; dux-shell v1.6.2 re-fetch sum == the originally validated `7ccb2361…`); no release-asset URLs, no non-GitHub sources, no moving targets. Full policy + workflow: `feralos-aports` `README.md` "Source pinning policy"; copy: `docs/aports-pinning.md`.

## Compatibility

No backward compat. Breaking changes allowed without fallback (e.g., breaking changes to `internal_rules.yaml` without deprecated path). Installer is single-run.

→ Details: `docs/adr/ADR-006-no-backward-compat.md`

## FirstBoot Architecture

Activatable features (Secure Boot, TPM) use standalone binaries in `src/firstBoot/`:

- `src/firstBoot/secureboot/main.go` — autonomous: installs packages, generates EFI keys, enrolls sbctl, builds+signs UKI, writes hooks
- `src/firstBoot/tpm/main.go` — autonomous: checks TPM, installs clevis@edge + tpm2-tools, binds LUKS to TPM2, rebuilds initramfs+UKI

**Installer flow**:

1. `firstboot.SetupSecureboot(conf)` / `firstboot.SetupTpm(conf)` compile binaries → copy to `/opt/feralos/bin/feralos-<name>` (mkdir of `/opt/feralos/bin` inside `buildBinary`), then `main` prints the end-of-installer first-boot guide (feature-conditioned paths, no ERROR/WARN-adjacent words)
2. `firstboot.WriteInstallConfig(conf)` writes `/opt/feralos/config.yaml` (disk layout, LUKS UUIDs, user info)
3. Binaries are autonomous — installer does NOT run them

**3-boot harness flow**:

1. **Runner (Debian)**: installer runs, compiles binaries, writes config. swtpm always present.
2. **Target (Alpine) — first boot**: harness SSH runs `feralos-secureboot` + `feralos-tpm` (if enabled). Binaries install packages, configure features, bind LUKS to TPM2. swtpm always present.
3. **Target (Alpine) — verification**: harness checks sbctl status, clevis bindings, LUKS auto-unlock, `podman ps` (when podman is enabled). swtpm always present.

**Key rules**:

- swtpm is always passed to runner + target VMs (even if EnableTPM=false) — simplifies harness
- firstBoot binaries read `/opt/feralos/config.yaml` for disk layout + LUKS UUIDs (fallback: kernel cmdline)
- On real hardware: user runs `/opt/feralos/bin/feralos-secureboot` and `/opt/feralos/bin/feralos-tpm` manually after first boot; the paths are printed by the installer's end-of-installer guide

**Known limitation**: Secure Boot enrollment cannot be confirmed in QEMU (no real efivarfs). TPM auto-unlock CAN be confirmed and works reliably.

→ Details: `docs/architecture/firstboot.md` (hostPhase/firstBoot design), `docs/guides/first_boot.md` (real hardware steps), `docs/guides/podman.md` (btrfs driver), `docs/FIRST_BOOT.md` is deprecated → `guides/first_boot.md`

## Project Structure

Full tree → `docs/INSTALLER.md#project-structure` and `docs/architecture/overview.md#project-structure`. Pointer only here to keep file concise, but info kept (double documentation).

## Docs Index (double documentation — AGENTS.md keeps full info, copies live in `docs/`)

- **Architecture**: `docs/architecture/overview.md` (principle + tree), `isolation.md` (entry-point + internal rule + hostPhase order), `logging.md` (ExecuteCmd rule), `feature_confinement.md` (single-owner), `storage.md` (layouts: btrfsStandardServer, btrfsStandard, btrfsRaid0, btrfsUnencrypted), `firstboot.md` (secureboot/tpm 3-boot), `dbus-activation.md` (ADR-010 mechanism + new-service checklist + diagnostics), `boot-oneshots.md` (both-init boot oneshot pattern) — all double of AGENTS.md above
- **ADRs**: `docs/adr/README.md` → ADR-001 Go installer, 002 isolation, 003 internal rule, 004 ExecuteCmd, 005 LUKS+Btrfs+booster+UKI, 006 no compat, 007 dinit shutdown ordering (decisions from AGENTS.md)
- **Guides** (user guides only): `docs/guides/README.md` (index) → `getting_started.md` (real hardware), `first_boot.md` (secureboot/tpm post-install), `variables.md` (variables.yaml compilation), `niriwayle.md` (flagship niriWayle desktop — Wayle/Veila/Raffi/oo7 + PAM unlock), `niri.md` (niri + NetworkManager + wayvnc), `podman.md` (btrfs driver), `development.md` (how to code / separation)
- **Harness**: `docs/harness/README.md` (index) → `overview.md` (what it does), `workflow.md` (3-phase + 3-boot), `artifacts.md` (where generated, gitignored), `building.md` (where compiled binary goes), `development.md` (how to code), `commands.md` (CLI+MCP), `debug_run.md` (`tests/debug_run/` boot existing run, LUKS inject). `tests/harness/README.md` is pointer only.
- **Intent / Ideas**: `docs/intent/vision.md`, `docs/intent/goals.md`, `docs/intent/vocabulary.md`, `docs/ideas/backlog.md` ← `CONTEXT.md` / `ROADMAP.md`
- **Project context / inventory**: `docs/memory_safe_alternatives.md` (detailed per-package memory-safety inventory; condensed table → root `CONTEXT.md`), `docs/FERALOS_NAME.md` (name rationale), `docs/aports-pinning.md` (feralos-side copy of the aports source-pinning policy — canonical + full: `feralos-aports` `README.md` "Source pinning policy")
- **AI tooling environment** (how this repo's agents/opencode setup is installed and updated — not FeralOS product docs): `docs/agents_guide.md` (agent roster + daily workflow), `docs/global_setup.md` (global opencode/caveman/cavecrew bootstrap), `docs/update.md` (update procedures)

→ All `docs/` files are double of AGENTS.md sections above. AI agents: read AGENTS.md first for full context, then follow `→` pointers for deep dives.

## Roadmap

| Phase | Milestone | Status |
|-------|-----------|--------|
| Phase 1 | btrfsStandardServer, btrfsStandard, btrfsRaid0 | Completed |
| Phase 1 | Generator, Test harness, Nix | Completed |
| Phase 1 | btrfsUnencrypted | Completed |
| Phase 1 | niri + NetworkManager + wayvnc + firewall + podman | Completed |
| Phase 1 | niriWayle flagship desktop (Wayle + Veila + Raffi + oo7) | Completed |
| Phase 1 | Refining | In progress |
| Phase 2 | Dinit (shipped, selectable via `enableDinit`), Custom repo (live) | In progress |
| Phase 2 | Full release (Dinit as default) | Planned |

See `ROADMAP.md` for pipeline priorities. Memory-safe alternatives table → `CONTEXT.md`. Backlog → `docs/ideas/backlog.md`.

## Known Issues

All prior swap/vector/dummy issues resolved. See `CHANGELOG.md` Fixed section. Current: `log_errors` shows QEMU-only noise (`smartd`/`fsck` whitelisted; netns `mount/umount /run/netns ... Invalid argument` QEMU early-chain noise allowlisted; udevd `/bin/false` module install errors — parport/psmouse/intel_gtt/input_leds/intel_pmc_bxt — from OpenRC udev path, harmless in QEMU, nix pivot fix landed). Resolved 2026-09-30: dinit shutdown ordering — `turnstiled` pam_elogind bus error at shutdown (dinit releases dependencies at dependent stop-start, not exit) fixed via feralos-aports `stop-command` guards on `dbus`/`elogind` (ADR-007, commit `75c566a`; harness 63/63 run 2026-09-30T00-08-04Z).
