# Changelog

All notable changes to FeralOS will be documented in this file.

## [Unreleased]

### Added
- Go installer with btrfsStandardServer layout (2-disk RAID1)
- Go installer with btrfsStandard layout (single disk, swapfile)
- Dynamic rules engine (internalRules.yaml + logic.go)
- APK hooks for kernel fallback and EFI sync
- Btrfs subvolume creation with mount options
- Fstab generation per layout
- Interactive command execution with retry/ignore/abort
- Config loading from YAML
- Generator (interactive + standalone)
- QEMU-based test harness with runner + target VMs
- Non-interactive mode for automated testing
- LUKS keyfile auto-unlock for headless boot (rd.luks.key)
- Improved logging with module context and timestamps
- Vector logging pipeline (Rust) replacing rsyslog/logrotate
  - Socket on `/run/vector/syslog.sock` (symlinked from `/dev/log`)
  - `vector-setup` service (boot stage) creates dirs, symlink, chowns `/var/log`
  - Config written AFTER package install (apk overwrites with demo_logs)
  - Init-system agnostic check: `apk info | grep -q '^vector$'`
- Per-subvolume btrfs compression via `btrfs property set` (zstd for data, none for NoCoW)
- Swapfile NoCoW + NOCOMPRESS (`chattr +C` + `chattr +m`)
- `@btrbk` and `@var-lib-vector` subvolumes in both disk layouts
- btrbk backup/snapshot tool with `snapshot_dir /btrbk/snapshots`
- earlyoom in standardPackages with service in packageLogic
- uutils subpackages: groups, more, uptime (stat removed — doesn't exist)
- Harness checks: `fstab_vs_mount`, `vector_service`, `vector_logs`, `vector_running`, `vector_diag`, `apk_packages` (dumps to run dir), `sudo_installed/wheel/works`
- `tests/debug-run/` — boot existing run without reinstalling (auto-detects latest run)
- EFI keys generated AFTER package install (matches reference flow)
- `WriteSecurebootConf` skips when `/etc/uefi-keys/db.key` missing
- APK v3 dummy packages via `apk mkpkg` (replaces binary manipulation)
- `providePackages` in internalRules.yaml for empty dependency-satisfying dummies
- `util-linux-misc` dummy to resolve `uutils-coreutils-more` conflict with `incus-feature-vm`
- `less_dummy` harness check (verifies symlink chain to bat)
- `log_errors` harness check — non-regression guard scanning target-serial.log + installer.log for error/failed markers, fails the run on any match (whitelist for known innocuous lines)
- `services_status` harness check — verifies configured OpenRC services are actually STARTED via `rc-status --all` (not just `[ ok ]` at boot); excludes QEMU-only services (smartd, fsck, udev-settle); parses supervise-daemon format `[ started 00:00:36 (0) ]`
- `lsattr_nocow` harness check — verifies C flag on all 7 NoCoW subvolumes, no `m` flag, compressed subvolumes stay CoW

### Fixed
- Pointer copy bug in configurator.go, hardening.go, zswap.go (u := conf.User → u := &conf.User)
- zswap cmdline flow: now appends to KernelCmdline slice, not overwrite file
- dmcrypt skips for btrfsStandard (uses swapfile, no swap partition)
- Harness hardcoded feature toggles: added EnableSecureboot/EnableHardening/EnableZswap/EnableNix to RunConfig
- efibootmgr guarded with EFI availability check (no error on BIOS runner)
- Harness disk layout changed to btrfsStandardServer (matches RAID1 verification)
- Unreachable code in harness service.go (shift key sleep)
- booster per-device keyfile args: rd.luks.key=/keyfile:luks-<uuid> for multi-device LUKS
- Vector VRL E103: `parse_syslog(.message) ?? {}` fallback
- Vector crash: socket moved from `/dev/log` to `/run/vector/syslog.sock` (permissions)
- Vector config overwritten by apk — now written after package install
- Secure Boot hook failure: EFI keys now generated after package install
- btrbk `snapshot_dir` path (was `/mnt/btrbk/snapshots`)
- "FeralOS Server" → "FeralOS" in user-facing strings
- Swap shutdown: localmount unmounted /swap while swapfile active ("in use but fuser finds nothing") — swap moved to boot runlevel + `sed 's/before localmount/need localmount/'` in SetupServices (`before` guarantees start order only, not stop order)
- Swapfile: `btrfs property set compression none` set the `m` attribute conflicting with `chattr +C` (NixOS/nixpkgs #384047) — compression property removed from ALL NoCoW subvolumes, `chattr +C` on dir before mount + `btrfs filesystem mkswapfile`
- `swap_status` harness check: busybox swapon has no `--show` flag — now uses `swapon -s || cat /proc/swaps`
- gofmt in src (config.go comment alignment, packages.go EOF newline)
- Harness lint cleanup: errcheck, nilerr, dead code removal, goconst, shadow, gosec, gocritic, hugeParam pointer refactors (commit `6b42ff3`)

### Changed
- Migrated from monolithic shell script to modular Go architecture
- Separated host phase and chroot phase into distinct packages
- Console output via console=ttyS0,115200 in kernel cmdline
- Keyfile generation added as Step 6 in main.go (between bootstrap and chroot prep)
- All packages installed in single `apk add` (base + TPM + microcode + Nix + hardening)
- `internalRules.yaml` cleaned: only base OS in defaultRunlevels, services in packageLogic
- udev services moved to packageLogic under eudev (Alpine uses mdev by default)
- `modules` service removed (booster loads `/etc/modules` automatically)
- earlyoom moved to standardPackages
- e2fsprogs removed from standardPackages
- Package masking removed (apk `!pkg` conflicts with explicit dependencies)
- Swap service in boot runlevel (moved from default) + `need localmount` dependency
- apk `.apk-new` behavior verified: modified /etc files (swap init.d sed) survive openrc upgrades
- log_errors whitelist: `start-stop-daemon: no matching processes found` (smartd, no SMART hardware in QEMU) + `fsck.vfat: No such file or directory` (fsck, QEMU-only)

### Open Issues
- Secure Boot fix (efi-mkkeys after install) — pending harness revalidation

### Planned (Phase 2)
- Dinit init system (replace OpenRC)
- Custom FeralOS repository
- Nix package manager integration
- Full distribution release

## [0.1.0] — Shell Script

### Added
- Working shell script installer (internal/start.sh)
- Dual-disk RAID1 partitioning
- Btrfs subvolume layout (20+ subvolumes)
- Alpine bootstrap via apk-tools-static
- Basic fstab and GRUB configuration

---

*Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)*
