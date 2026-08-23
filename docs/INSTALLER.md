# FeralOS Installer Architecture

## Overview

The FeralOS installer is a Go-based system installer that creates a hardened Alpine Linux system with LUKS encryption, Btrfs RAID1/single, booster initramfs, and signed Unified Kernel Image (UKI). It replaces GRUB entirely — the system boots directly from the UKI.

## Core Architecture Principles

- **Standard core, pluggable data**: Installer logic never changes; only data (YAML + Go layouts) changes
- **Phase separation**: Host operations (hostPhase) run on the host, chroot operations (chrootPhase) run inside the chroot
- **Modular design**: Each functionality is a separate module for easy review and maintenance
- **Memory safety**: Uses memory-safe alternatives where possible (uutils, micro, sudo-rs)

## Project Structure

Full tree — `AGENTS.md` keeps only pointer. Details here:

```
feralos_installer/
├── src/
│   ├── main.go, go.mod, variables.yaml
│   ├── variables/ (variables.yaml, internal_rules.yaml)
│   ├── utils/
│   │   ├── config/config.go
│   │   ├── shell/execute.go          # ExecuteCmd(chroot, cmd) unified
│   │   ├── logic/logic.go            # dynamic rule application
│   │   ├── swap/swap.go              # ResolveSwapSizeMiB (auto=2×RAM)
│   │   └── createConfig/ (create_values.go, createValues/main.go)
│   ├── hostPhase/
│   │   ├── host_phase.go             # orchestrator: MountVirtualFilesystems + BootstrapAlpine + PrepareChrootEnv + InjectApkHooks
│   │   ├── internal/
│   │   │   ├── bootstrap.go
│   │   │   ├── bind_mounts.go
│   │   │   ├── chroot_env.go
│   │   │   └── hooks.go
│   │   └── disks/
│   │       ├── btrfsStandard/ (partitioning.go, fstab.go, luks.go)       # 1 disk, swapfile via swapSize
│   │       ├── btrfsRaid0/ (partitioning.go, fstab.go, luks.go)          # 2 disks RAID0, 2 LUKS swap p2 gated enableSwap
│   │       └── btrfsStandardServer/ (partitioning.go, fstab.go, luks.go) # 2 disks RAID1, always LUKS swap
│   ├── firstBoot/
│   │   ├── secureboot/main.go
│   │   └── tpm/main.go
│   └── chrootPhase/
│       ├── chroot_phase.go           # 14-step orchestrator
│       ├── internal/
│       │   ├── packages.go, firmware.go, network.go, boot.go, storage.go
│       │   ├── logging.go, misc.go, services.go, users.go, uki.go
│       │   └── ...
│       ├── features/ (hardening.go, zswap.go, zswap_server.go, nix.go)
│       └── desktops/ (niri.go, kde.go, gnome.go)
├── tests/
│   ├── fixtures/variables/ (14 templates + README — all scenarios)
│   ├── harness/ (cmd/skea-harness, internal/, variables-*.yaml deprecated shims)
│   └── artifacts/ (gitignored: cache, prepare, runs, state)
├── docs/ (PROJECT.md, INSTALLER.md, USAGE.md)
├── state/session-state.json
└── handoff/latest.md
```

## Installation Flow

### Phase 1: Host Operations (hostPhase)

1. **Load Configuration** (`main.go`)
   - Parse `variables.yaml` (user config)
    - Load `internal_rules.yaml` (dynamic rules)
   - Run interactive wizard if not unattended

2. **Apply Dynamic Rules** (`logic.go`)
   - Merge standard packages with layout-specific packages
   - Add desktop environment packages if selected
   - Add video driver packages if selected
   - Apply package-specific rules (groups, services, modules)

3. **Check Host Dependencies** (`main.go`)
   - Verify required tools: btrfs-progs, cryptsetup, curl, tar, dosfstools, rsync, efitools, sbsigntool
   - Install missing dependencies via apt

4. **Disk Partitioning & LUKS Setup** (`disks/`)
   - **btrfsStandardServer**: 2 disks, RAID1, dual EFI, LUKS swap
   - **btrfsStandard**: 1 disk, LUKS root, swapfile, 25 subvolumes
   - Create partition tables, format filesystems, setup LUKS encryption

5. **Mount Virtual Filesystems** (`bind_mounts.go`)
   - Mount /dev, /proc, /sys to chroot directory
   - Make mounts slave to prevent propagation

6. **Alpine Bootstrap** (`bootstrap.go`)
   - Download apk-tools-static
   - Initialize Alpine base system into target
   - Configure APK repositories

7. **Generate LUKS Keyfile** (`keyfile.go`)
   - Generate random 2KB keyfile via `dd if=/dev/urandom`
   - Add keyfile to all LUKS root partitions via `cryptsetup luksAddKey`
   - Copy keyfile to `/keyfile` (initramfs) and `/etc/luks-keyfile` (persistence)
   - Generate per-device kernel cmdline args: `rd.luks.key=/keyfile:luks-<uuid>`
   - **Why**: Booster requires interactive passphrase in QEMU headless; keyfile in initramfs enables automatic unlock
   - **Multi-LUKS**: With 2+ LUKS devices, booster requires per-device keyfile spec

8. **Prepare Chroot Environment** (`chroot_preparation.go`)
   - Write networking config (resolv.conf, hostname, hosts)
   - Write localization config (locale, keymap)
   - Write system limits and subuid/subgid
   - Configure APK repositories
   - Write shell aliases and environment
   - Configure sudo-rs permissions
   - Write kernel modules configuration
   - Write ZRAM and LXC service configs
   - Write bridging sysctl

8. **Write Booster Config** (`booster.go`)
   - Generate booster.yaml for initramfs
   - Write kernel command line parameters
   - Create booster hook for kernel upgrades
   - Create secureboot hook config

9. **Inject APK Hooks** (`hooks.go`)
   - Add commit hooks for automatic UKI rebuild on kernel updates

### Phase 2: Chroot Operations (chrootPhase)

1. **Install Packages** (`packages.go`)
   - Install dummy packages (APK v3 `apk mkpkg`) for memory-safe replacements
   - Install provide packages (empty dummies) to satisfy dependencies
   - Install standard packages from internal_rules.yaml
   - Install layout-specific packages
   - Install desktop environment packages (if selected)
   - Install video driver packages (if selected)
   - Install extra packages from user config

2. **Setup Services** (`services.go`)
   - Configure OpenRC runlevels
   - Create shell aliases (ll, sudo=sudo-rs, nano=micro)
   - Setup symlinks for tools
   - Configure timezone and locale

3. **Setup Users** (`users.go`)
   - Create root password
   - Create regular user with groups
   - Configure SSH authorized keys
   - Set default shell (fish)

4. **Setup Dmcrypt Swap** (`dmcrypt.go`)
   - Generate LUKS keyfile
   - Add keyfile to LUKS partition
   - Configure dmcrypt for swap

5. **Setup Zswap** (`zswap.go`)
   - Enable zswap if configured
   - Calculate vm.min_free_kbytes (1% RAM)
   - Configure zswap parameters

6. **Setup Nix** (`nix.go`)
   - Install Nix package manager (if enabled)
   - Configure Nix directories and permissions

7. **Apply Hardening** (`hardening.go`)
   - Enable AppArmor
   - Configure sysctl parameters
   - Set permissions on sensitive files
   - Apply security policies

8. **Write Final Kernel Cmdline** (`configurator.go`)
   - Append zswap and hardening arguments
   - Write to /etc/kernel/cmdline

9. **Build Signed UKI** (`uki.go`)
   - Detect CPU vendor (AMD/Intel)
   - Install microcode
   - Generate EFI keys
   - Build booster initramfs
   - Build UKI with efi-mkuki
   - Sign UKI with sbsign
   - Create fallback BOOTX64.EFI
   - Mirror to secondary EFI partition
   - Create EFI boot entries

10. **Final Upgrade** (`configurator.go`)
    - Run apk update && apk upgrade

11. **Write Vector Config** (`configurator.go`)
    - Write `/etc/vector/vector.yaml` AFTER package install (apk overwrites with demo_logs)
    - Socket source on `/run/vector/syslog.sock` (symlinked from `/dev/log`)
    - Sinks: `/var/log/messages`, `auth.log`, `daemon.log`, `kern.log`
    - `vector-setup` service (boot) creates dirs, symlink, chowns `/var/log`

## Key Components

### Disk Layouts

#### btrfsStandardServer (2 disks)
- **Partitioning**: EFI (512MB) + LUKS (rest) on each disk
- **RAID1**: Btrfs RAID1 on LUKS for root filesystem
- **Swap**: LUKS-encrypted swap partition
- **Subvolumes**: 21 subvolumes including @root, @home, @boot, @nix-*
- **Compression**: Per-subvolume via `btrfs property set` (zstd for data, none for NoCoW)

#### btrfsStandard (1 disk)
- **Partitioning**: EFI (512MB) + LUKS (rest)
- **Single**: Btrfs single mode (no RAID)
- **Swap**: Swapfile in @swap subvolume (NoCoW, NOCOMPRESS)
- **Subvolumes**: 25 subvolumes including @root, @home, @boot, @nix-*
- **Compression**: Per-subvolume via `btrfs property set` (zstd for data, none for NoCoW)

### LUKS Encryption

- **Root**: LUKS2 encryption on root partition
- **Swap**: LUKS-encrypted swap (btrfsStandardServer) or swapfile (btrfsStandard)
- **Keyfile**: Generated and stored in /etc/keys/ for swap auto-unlock
- **Password**: Same password for LUKS and user login

### Boot Process

1. **UEFI**: Loads BOOTX64.EFI from EFI partition
2. **UKI**: Unified Kernel Image contains kernel + initramfs + cmdline
3. **Booster**: Fast initramfs that loads necessary modules
4. **LUKS**: Prompts for password to unlock root partition
5. **Btrfs**: Mounts root subvolume and other subvolumes
6. **Init**: Starts OpenRC init system

### Security Features

- **AppArmor**: Mandatory access control
- **Sysctl**: Kernel hardening parameters
- **File Permissions**: Strict permissions on sensitive files
- **Secure Boot**: Signed UKI for verified boot
- **LUKS2**: Full disk encryption
- **Memory Safety**: uutils, micro, sudo-rs, vector, fish, incus instead of C alternatives

## Configuration Files

### variables.yaml (User Config)
- Disk configuration (disks, layout)
- User credentials (username, password)
- System identity (hostname, timezone, locale)
- Repository settings (mirror, branch)
- Feature toggles (encryption, secureboot, TPM, hardening, zswap, nix)
- Extra packages

### internal_rules.yaml (Dynamic Rules)
- Standard packages (hardcoded)
- Disk layout definitions (packages, modules)
- Desktop environment definitions (packages, services, groups)
- Video driver definitions (packages, modules)
- Package-specific rules (groups, services, modules)

## Error Handling

- **Non-interactive mode**: `SKEA_NONINTERACTIVE=1` + `SKEA_NONINTERACTIVE_DECISION=ignore|stop`
- **ExecuteCmd**: Unified command execution with logging, retry (3 attempts), and user feedback
- **File writes**: Done via shell commands (echo/cat) instead of direct file writes
- **Logging**: Timestamp + command, stdout/stderr, exit code, duration
- **Retry**: After 3 failures, asks user: [s]top, [i]gnore, [r]etry (interactive) or auto-continues/aborts (non-interactive)

## Testing

- **Harness**: QEMU-based testing with virtual machines
- **Artifacts**: Test outputs in tests/artifacts/ (gitignored)
- **Modes**: Install, boot, shell access for manual testing
- **Checks**: 22/22 verification checks incl. `fstab_vs_mount`, `vector_*`, `apk_packages` dumps
- **debug-run**: `tests/debug-run/` boots an existing run without reinstalling (auto-detects latest run dir)

## Build & Run

```bash
# Build installer
cd src && go build -o ../feralos_installer .

# Build first-boot programs
cd src && go build -o ../feralos_secureboot ./firstBoot/secureboot/
cd src && go build -o ../feralos_tpm ./firstBoot/tpm/

# Run installer (requires root + live environment)
sudo ./feralos_installer -config variables.yaml

# Run with non-interactive mode
SKEA_NONINTERACTIVE=1 SKEA_NONINTERACTIVE_DECISION=ignore sudo ./feralos_installer -config variables.yaml
```
