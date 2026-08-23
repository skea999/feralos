# Changelog

All notable changes to FeralOS will be documented in this file.

## [Unreleased]

### Added
- Go installer with btrfsStandardServer layout (2-disk RAID1)
- Go installer with btrfsStandard layout (single disk, swapfile)
- Dynamic rules engine (internal_rules.yaml + logic.go)
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
- `providePackages` in internal_rules.yaml for empty dependency-satisfying dummies
- `util-linux-misc` dummy to resolve `uutils-coreutils-more` conflict with `incus-feature-vm`
- `less_dummy` harness check (verifies symlink chain to bat)
- `log_errors` harness check — non-regression guard scanning target-serial.log + installer.log for error/failed markers, fails the run on any match (whitelist for known innocuous lines)
- `services_status` harness check — verifies configured OpenRC services are actually STARTED via `rc-status --all` (not just `[ ok ]` at boot); excludes QEMU-only services (smartd, fsck, udev-settle); parses supervise-daemon format `[ started 00:00:36 (0) ]`
- `lsattr_nocow` harness check — verifies C flag on all 7 NoCoW subvolumes, no `m` flag, compressed subvolumes stay CoW
- **niri desktop environment** — greetd + regreet + niri + wayvnc + DMS:
  - `nixPackages` field in internal_rules (same behavior as apk — data selects install, auto-enables Nix)
  - `chrootPhase/desktops/niri.go`: greetd config, greeter.sh, regreet.toml, niri configs (greeter + user), wayvnc config, xdg-su wrapper, OpenRC dms service
  - DMS (DankMaterialShell) + quickshell from stable nixpkgs channel (nixos-26.05)
  - Harness checks: niri_config_valid, greetd_config, niri_greeter, wayvnc_installed, dms_installed
- **btrfsRaid0 layout** — dual-disk pure RAID0 (data+metadata), EFI on both disks, 2 LUKS swap partitions (partition 2, swapSize/2 each) gated by `enableSwap`, root on partition 3 (remainder — no size formulas)
- **`enableSwap` + `swapSize`** — generic swap toggle replacing `enableSwapfile`: btrfsStandard keeps swapfile, btrfsRaid0 uses LUKS swap partitions, btrfsStandardServer unchanged (always LUKS swap). `swapSize` default `auto` = 2× RAM (via `utils/swap.ResolveSwapSizeMiB`), or explicit `8G`/`4096M`

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
- niri greetd crash: missing `wayland-libs-server` (niri dlopens libwayland-server.so.0 at runtime → NoWaylandLib panic) + greeter not in `greetd` group (couldn't traverse /var/lib/greetd 750)
- nix build in chroot: sandbox pivot_root EINVAL — `--option sandbox false` chroot-only (trusted-users = root @wheel), TMPDIR on @nix-var-nix-build subvolume (/tmp is noexec+0755)
- nix stable channel: default flake registry maps nixpkgs→unstable — `nix registry add nixpkgs github:NixOS/nixpkgs/nixos-26.05`
- sudoers: Alpine default `/etc/sudoers` has `@includedir` commented out → `/etc/sudoers.d/wheel` ineffective (wheel sudo denied) — enabled with sed + new `sudoers_includedir` harness check
- nix.conf: removed `allow-unfree`/`allow-insecure` (unknown settings in Alpine's nix build — warning noise; NIXPKGS_ALLOW_* env vars carry it permanently)
- btrfsStandardServer LUKS swap never activated: dmcrypt keyfile+conf.d written in hostPhase BEFORE the btrfs root was mounted at /mnt → files lost in host /mnt. Fixed: keyfile written directly into chroot after subvol mounts (step 8a) + fstab swap `LABEL=swap-*` → `/dev/mapper/swap-*` (busybox `swapon -a` can't resolve LABEL in fstab) + `swap_status` check now truthful
- btrfsRaid0 swap never activated (latent bug): swap service was never added to the runlevel — now conditional on `enableSwap`

### Changed
- Migrated from monolithic shell script to modular Go architecture
- Separated host phase and chroot phase into distinct packages
- Console output via console=ttyS0,115200 in kernel cmdline
- Keyfile generation added as Step 6 in main.go (between bootstrap and chroot prep)
- All packages installed in single `apk add` (base + TPM + microcode + Nix + hardening)
- `internal_rules.yaml` cleaned: only base OS in defaultRunlevels, services in packageLogic
- udev services moved to packageLogic under eudev (Alpine uses mdev by default)
- `modules` service removed (booster loads `/etc/modules` automatically)
- earlyoom moved to standardPackages
- e2fsprogs removed from standardPackages
- Package masking removed (apk `!pkg` conflicts with explicit dependencies)
- Swap service in boot runlevel (moved from default) + `need localmount` dependency
- apk `.apk-new` behavior verified: modified /etc files (swap init.d sed) survive openrc upgrades
- log_errors whitelist: `start-stop-daemon: no matching processes found` (smartd, no SMART hardware in QEMU) + `fsck.vfat: No such file or directory` (fsck, QEMU-only)
- `enableSwapfile` → `enableSwap` (generic swap toggle) + new `swapSize` variable (default `auto` = 2× RAM)

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
