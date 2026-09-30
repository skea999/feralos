# Handoff

**Active state**: `state/session_state.json`
**Milestone log**: `CHANGELOG.md`

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern alternatives (uutils, micro, vector). For software not in Alpine repos or requiring glibc, **Nix** is used. **Dinit** replaces OpenRC (complete, 47/47 verify). A **custom Alpine-style repository** for FeralOS packages is live on GitHub Pages.

## Current Focus

**Logging Round 2 (2026-09-30)**: sink split (`sudo.log`, `firewalld.log`), vista JSON `/var/log/feralos.json.log`, `internal_logs` nel flusso centrale, API Vector locale **solo con `enableDebug`** (127.0.0.1:8686), sorgente incus **condizionale** (solo se pacchetto installato; webmin saltato — log in tmpfs), retention logrotate 30/14/7. 3 check nuovi (`json_log`, `vector_api`, `vector_validate`). Harness **70/70** (run 2026-09-30T23-09-27Z, json 523 righe). Config pre-validata in docker (2 fix VRL 0.55 trovati prima del run).

**Logging Round 1 (2026-09-30)**: kernel ring finalmente nei file — nuovo `klogd-dinit` (busybox klogd → /dev/log → vector `kern.log` + snapshot `/var/log/dmesg`), dinit-only (l'OpenRC klogd richiede `logger`, sostituito da vector). Vector: rimossi i duplicati da rotazione (`*.log.1` + copytruncate) e disk buffer sui 4 sink (min 256 MiB). Harness: 4 check nuovi — **67/67** (run 2026-09-30T22-52-14Z, kern.log 56 righe, 0 bus error). feralos-aports `f2842dc` (CI green).

**Artix-parity audit (2026-09-30, round 2)**: 49 servizi dinit confrontati con Artix — 18 pacchetti aggiornati in **un solo push** (feralos-aports `90197ed`, CI green): seatd readiness (`-g seat -n 3` + pipefd:3), nftables stop flush, iptables `-w 3`, sddm+elogind, cosmic-greeter+seatd, avahi/cupsd/dnsmasq+dbus, turnstiled log in `/var/log/dinit/`, `smooth-recovery` dove Artix lo ha; rimossi `pipewire-dinit`/`wireplumber-dinit` (mai installati). Harness niri **63/63** (run 2026-09-30T22-04-51Z).

**dinit shutdown ordering (2026-09-30)**: dependency stop race fixed at the source — dinit releases a dependency when dependents *begin* stopping, so at shutdown `dbus`/`elogind` could die while `turnstiled` was still closing PAM sessions ("Failed to connect to system bus"). feralos-aports `dbus-dinit` r6 + `elogind-dinit` r5 add `stop-command` guards (wait bounded for `turnstiled` via `pidof`, then stop the daemon; `stop-timeout=30` failsafe; no-op without turnstile) — commit `75c566a`, ADR-007. Harness `log_errors` allowlist removed (no masking); niri+dinit **63/63** (run 2026-09-30T00-08-04Z, 0 bus errors, `turnstiled` stops before `dbus`/`elogind` in both shutdowns).

**Dinit — COMPLETE** (47/47 verify, 2026-09-20). Modello Alpine/Artix: pacchetti `-dinit` ship-only, enablement via regole. Servizi: chronyd, sshd, incusd, smartd, nix-daemon, mdev, incus-agent + networking-dinit (ifup -a). Log: `log-type=file` → `/var/log/dinit/<nome>.log`, trail PID1 → `/run/dinit-boot.log` (wrapper legge da `/proc/cmdline`) → `/var/log/dinit/boot.log` (tail -F), vector ingests, logrotate. aports Step 7 COMPLETE. Harness: verify 47/47 su btrfs_standard_dinit.

**Desktops dinit**: KDE (`sddm-dinit`), Cosmic (`cosmic-greeter-dinit` + `seatd-dinit`), Niri (`greetd-dinit` + `elogind-dinit` + `polkit-elogind-dinit`) — tutti testati con harness.

**Dinit shutdown + harness phases (2026-09-25)**: installer ships `dinit-shutdown` + wires conventional commands — `/usr/bin/{poweroff,halt,shutdown}` symlinks + `/usr/bin/{reboot,soft-reboot}` wrappers (busybox `poweroff` sends SIGUSR2 — a no-op under dinit; `dinitctl shutdown` only HALTs). Harness: numbered run phases (`[harness] phase N/9: …` → stdout + `harness.log`) and init-aware `powerOffTarget()` (`enableDinit` → `/usr/sbin/dinit-poweroff`; OpenRC → `poweroff`) with 5-min fail-not-hang bound; new verify check `dinit_shutdown_commands`. Verified: dinit **48/48** (2m41s), OpenRC **39/39** (2m32s), live reboot/poweroff test. Milestone log moved from AGENTS.md bottom to `CHANGELOG.md` (`## Milestone Log`).

**Harness check truthfulness (2026-09-25)**: dinit verify checks made falsifiable — 8 of 9 `dinit_*` checks ended in `&& echo ok || echo fail` (the fallback `echo` always exits 0 → PASS forever), with 3 broken commands: busybox `ps` has no `-p` (→ `/proc/1/comm`), bare `dinitctl status` requires `<service-name>` (→ `dinitctl list`), `dinit_feralos_repo` grepped `/etc/apk/repositories` (keyring installs `/etc/apk/repositories.d/feralos.list`). All now `COND && echo '<ok>' || { echo '<fail>'; exit 1; }`; verified `btrfs_standard_dinit` **48/48** (run 2026-09-24T23-42-02Z).

**Installer**: go vet/lint/build clean. Branch `developDinit` published su gitea (275 commit). History scrubbed: 0 italiano, 0 `/home/skem` in entrambi i repo.

Phase 1: Go installer complete + fixtures library + AGENTS compress + githooks vet/lint. Highlights (latest runs **32/32**):

- **btrfsRaid0 layout** (`48ad7bc`): pure RAID0, EFI both disks, 2 LUKS swap p2 (`swapSize/2`) gated `enableSwap`, root p3 remainder, `swap_service` conditional (fixes latent bug: swap never activated on RAID0)
- **enableSwap + swapSize** (generic): `enableSwapfile`→`enableSwap`, `swapSize` `auto`=2×RAM via `utils/swap.ResolveSwapSizeMiB` or `8G`/`4096M`; `btrfsStandardServer` unchanged
- **Fixtures library** (`tests/fixtures/variables/` 14 templates): btrfsStandard/Server/Raid0 × swap on/off × swapSize auto/8G × secureboot/tpm/hardening isolate × niri × full-stack. README matrix + Tier1 CI (minimal+noswap). Old harness `variables-*.yaml` kept `DEPRECATED` → new fixtures. Verified: raid0 32/32, standard 32/32 (7.66GiB), noswap 32/32, server 32/32 — 128/128
- **Fixtures validation 2026-08-23**: `btrfsRaid0-noswap` 32/32 (header-only swap_status, RAID0 pure), `btrfsStandard`/`btrfsRaid0` `8G` explicit 32/32 (8192M file / 4177916 KiB per partition)
- **AGENTS compress**: 596→145 lines, structure moved to `docs/INSTALLER.md#project-structure` (pointer only), history truncated → `CHANGELOG.md`
- **Githooks vet/lint**: staged `*.go` → `go vet ./...` (src/harness) + `golangci-lint` (`$HOME/go/bin` fallback), fixed `grep --` leading dash bug, set `core.hooksPath=.githooks`
- **Swapfile/NoCoW fixes** — `btrfs filesystem mkswapfile` + `chattr +C` on @swap dir before mount, compression property removed from ALL NoCoW (NODATACOW implies no compression), `lsattr_nocow` harness check
- **Swap shutdown FIXED** — swap moved to boot runlevel + `need localmount` (was `before`), apk `.apk-new` survives upgrades
- **`log_errors` + `services_status`** — non-regression guards (whitelist smartd/fsck, parse supervise-daemon)
- **niri DE** — `nixPackages` in internal_rules, `chrootPhase/desktops/niri.go`, DMS via stable nixpkgs, **37/37 PASS**; tests/harness checks + virtio-gpu-pci; sudoers `includedir` enabled; nix.conf cleaned; `hwclock`? no
- **Dummy packages** — APK v3 `apk mkpkg` symlinks; BusyBox grep (not GNU/rg)

## What's Working

- Shell script installer (`internal/start.sh`) — full reference implementation
- Go installer: config loading, dynamic rules, host phase, chroot phase
- **btrfsStandardServer**: Dual-disk RAID1 layout with Btrfs subvolumes (21 subvolumes)
- **btrfsStandard**: Single-disk layout with swapfile (25 subvolumes)
- LUKS2 encryption on root and swap partitions
- **LUKS keyfile auto-unlock** — embedded in initramfs via `rd.luks.key=/keyfile:luks-<uuid>`
- booster initramfs (replaces mkinitfs)
- Signed UKI (Unified Kernel Image) for Secure Boot
- Serial console output (`console=ttyS0,115200`)
- efibootmgr guarded for BIOS runners
- APK hooks for kernel fallback and EFI sync
- Interactive wizard and standalone generator
- QEMU-based test harness with feature toggles
- Non-interactive mode for automated testing
- **Clevis TPM2 binding** — empty PCR policy (TPM identity only)
- **Vector logging** — replaces rsyslog (Rust, memory-safe); logrotate re-added for file rotation
  - Socket: `/run/vector/syslog.sock` (symlinked from `/dev/log`)
  - Sinks: `/var/log/messages`, `auth.log`, `daemon.log`, `kern.log`
  - Dinit services: `log-type=file` → `/var/log/dinit/<name>.log` + PID1 trail → `/var/log/dinit/boot.log`
  - Vector `dinit_services` file source ingests dinit logs
  - Logrotate: `/etc/logrotate.d/feralos` (copytruncate + delaycompress, daily, rotate 7)
  - Service: `vector-setup` (OpenRC) + `vector` (boot) + `dinit-boot-log` (dinit, mirrors PID1 trail)
- **btrbk** — btrfs backup/snapshot tool with subvolume `@btrbk`
- **turnstile** — session tracker for desktop environments
- **internal_rules.yaml cleaned** — only base OS in defaultRunlevels, services in packageLogic
- **APK v3 dummy packages** — `apk mkpkg` creates valid packages, installed before main `apk add`
- **providePackages** — empty dummies for dependency satisfaction (e.g., `util-linux-misc`)
- **uutils-coreutils-more** — provides `/usr/bin/more`, dummy `util-linux-misc` satisfies `incus-feature-vm`
- **Swapfile working** — `btrfs filesystem mkswapfile --size 8g` with `chattr +C` on @swap dir before mount
  - btrfsStandardServer: no swapfile (dedicated LUKS swap partitions)
- **NoCoW subvolumes clean** — no compression property anywhere (NODATACOW implies no compression)
  - `chattr +C` on dir is the only mechanism; all 7 NoCoW subvolumes verified by `lsattr_nocow` harness check
  - Compressed subvolumes verified to stay CoW (no C flag)

## What's Next

### Phase 1 (Installer)
1. btrfsStandardUnencrypted layout (planned)
2. Test on real hardware with 2x NVMe
3. Test with partial features (Secure Boot only, TPM only)
4. Add more unit tests for verify checks

### Phase 2 (Distribution)
1. ~~Dinit init system (replace OpenRC)~~ ✅ DONE
2. Custom FeralOS repository — live on GitHub Pages
3. Full distribution release

## Architecture Principle

**Standard core, pluggable data.**

- Installer logic never changes
- Storage layouts are Go packages
- Essential packages hardcoded in Go
- Optional packages + rules in YAML
- Parser always same code, only data changes

## Key Files

| File | Purpose |
|------|---------|
| `src/main.go` | Installer entry point |
| `src/utils/config/config.go` | Config types + LoadConfig |
| `src/utils/shell/execute.go` | Unified ExecuteCmd (retry, logging) |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internal_rules.yaml` | Dynamic rules (packages, services) |
| `src/hostPhase/` | Host-side operations (orchestrator + internal/) |
| `src/hostPhase/disks/` | Storage layouts (Go packages) |
| `src/chrootPhase/` | Chroot operations (orchestrator + internal/ + features/) |
| `src/firstBoot/` | Standalone first-boot programs (secureboot, tpm) |
| `setup.sh` | Automated setup script |
| `tests/harness/` | QEMU-based test harness |
| `tests/debug_run/` | Debug tool for existing runs |
| `docs/INSTALLER.md` | Installer architecture documentation |
| `docs/USAGE.md` | Usage guide and configuration |
| `CONTEXT.md` | Domain vocabulary |

## Decisions Made

- **Vector socket**: `/run/vector/syslog.sock` (not `/dev/log` — permissions)
- **vector-setup**: boot stage (not sysinit — /var/log mounted by localmount)
- **Vector config**: Written AFTER package install (apk overwrites with demo_logs)
- **Vector check**: `apk info | grep -q '^vector$'` (init-system agnostic)
- **defaultRunlevels**: Only base OS services, rest in packageLogic
- **udev services**: In packageLogic under eudev (Alpine uses mdev by default)
- **modules service**: Removed (booster loads /etc/modules automatically)
- **btrbk snapshot_dir**: `/btrbk/snapshots` (not `/mnt/btrbk/snapshots`)
- **less package**: Solved via dummy package symlink (bat replaces less)
- **booster** instead of mkinitfs for initramfs
- **Signed UKI** instead of GRUB for boot
- **LUKS2** encryption on RAID and Swap partitions
- **Btrfs RAID1** on LUKS (not mdadm)
- **Phase separation**: hostPhase (writes files) vs chrootPhase (runs commands)
- **Ripgrep NOT a drop-in for grep**: rg doesn't support POSIX BRE (`\+`), breaks `efi-mkuki`
- **GNU grep removed**: BusyBox grep provides POSIX BRE, zero reverse dependencies
- **Dummy packages**: Symlink approach (APK content, not wrapper scripts)
- **Swapfile**: `btrfs filesystem mkswapfile` + `chattr +C` on dir before mount (Debian guide)
- **NoCoW**: no compression property — `chattr +C` on dir only (NODATACOW implies no compression, `m` attribute conflicts with C)
- **Swap service**: boot runlevel + `need localmount` (sed replaces `before localmount` — start order only, not stop order). apk preserves modified /etc files as `.apk-new` — the sed survives upgrades
- **Log errors check**: harness `log_errors` scans target-serial.log + installer.log for error/failed markers, fails run on any match (non-regression guard, whitelist for known innocuous lines incl. QEMU-only smartd/fsck)
- **Services active check**: harness `services_status` verifies configured services are actually started via `rc-status --all`; excludes QEMU-only services (smartd, fsck, udev-settle); parses supervise-daemon format
- **niri DE**: data in internal_rules (`nixPackages` field, auto-enables Nix) + config logic in `chrootPhase/desktops/niri.go` (kde/gnome stay data-only)
- **Nix stable**: registry pinned to nixos-26.05 (default maps nixpkgs→unstable); sandbox disabled only for chroot install; TMPDIR on nix build subvolume; NIXPKGS_ALLOW_* env vars (not nix.conf — unknown settings)
- **Seat provider**: elogind (not seatd); no render group (Alpine uses video); no turnstile (edge only)
- **sudoers**: `@includedir /etc/sudoers.d` enabled (Alpine default comments it out — wheel sudo was denied)
- **fstab swap**: `/dev/mapper/swap-*` (not `LABEL=swap-*` — busybox `swapon -a` can't resolve LABEL, literal path only)
- **dmcrypt keyfile**: written directly into chroot after subvol mounts (no /tmp + copy, mappers still open, `apk` preserves as `.apk-new`)
- **enableSwap + swapSize**: generic toggle (`auto`=2×RAM via `utils/swap.ResolveSwapSizeMiB`), `swapSize` total split across partitions, `btrfsRaid0` swap on p2 root on p3 remainder, `btrfsStandardServer` unchanged (always swap)
- **AGENTS compress**: pointer-only structure + history → `CHANGELOG.md` + `git log` + `.agent-memory` (<250 lines)
- **githooks**: staged Go → `go vet` + `golangci-lint` (`$HOME/go/bin` fallback), `grep --` fix for `-----BEGIN`

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language | Drop-in? |
|-----------------|------------------|----------|----------|
| coreutils | uutils-coreutils | Rust | No |
| vim/nano | micro | Go | No |
| sudo | sudo-rs | Rust | No |
| rsyslog/logrotate | vector | Rust | No |
| bash/zsh | fish | Rust | No |
| lxd | incus | Go | No |
| openrc | dinit | C++/Alpine | ✅ DONE |
| grep (GNU) | BusyBox grep | POSIX BRE | Yes (built-in) |
| find (GNU) | fd | Rust | No (different syntax) |
| less | bat | Rust | Yes (symlink) |

---

*See `state/session_state.json` for current goals, decisions, risks, and next actions.*
*See `AGENTS.md` for project context and milestone history.*
*See `docs/PROJECT.md` for full distribution vision.*
