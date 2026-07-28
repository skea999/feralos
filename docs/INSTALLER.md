# FeralOS Installer Architecture

## Overview

The FeralOS installer is a Go-based system installer that creates a hardened Alpine Linux system with LUKS encryption, Btrfs RAID1/single, booster initramfs, and signed Unified Kernel Image (UKI). It replaces GRUB entirely — the system boots directly from the UKI.

## Core Architecture Principles

- **Standard core, pluggable data**: Installer logic never changes; only data (YAML + Go layouts) changes
- **Phase separation**: Host operations (hostPhase) run on the host, chroot operations (chrootPhase) run inside the chroot
- **Modular design**: Each functionality is a separate module for easy review and maintenance
- **Memory safety**: Uses memory-safe alternatives where possible (uutils, micro, doas)

## Project Structure

```
src/
├── main.go                          # Installer entry point
├── hostPhase/                       # Host-side operations (writes files, NO chroot commands)
│   ├── bootstrap.go                 # Alpine bootstrap (apk.static)
│   ├── bind_mounts.go               # Virtual filesystem mounts (/dev, /proc, /sys)
│   ├── booster.go                   # booster.yaml + kernel cmdline + hooks
│   ├── chroot_preparation.go        # Write config files into chroot
│   ├── keyfile.go                   # LUKS keyfile generation + embedding
│   ├── hooks.go                     # APK hooks injection
│   └── disks/                       # Storage layouts (Go packages)
│       ├── btrfsStandard/           # Single disk, LUKS, swapfile, 25 subvolumes
│       │   ├── partitioning.go      # Disk partitioning and Btrfs setup
│       │   ├── fstab.go             # Filesystem table generation
│       │   └── luks.go              # LUKS encryption setup
│       └── btrfsStandardServer/     # 2 disks, RAID1, dual EFI, LUKS swap
│           ├── partitioning.go      # Disk partitioning and Btrfs RAID1 setup
│           ├── fstab.go             # Filesystem table generation
│           └── luks.go              # LUKS encryption setup
├── chrootPhase/                     # Chroot operations (runs INSIDE chroot)
│   ├── configurator.go              # Main orchestrator for chroot phase
│   ├── packages.go                  # Package installation (apk add)
│   ├── services.go                  # OpenRC services, aliases, symlinks, timezone
│   ├── users.go                     # User creation, groups, SSH, shell
│   ├── dmcrypt.go                   # LUKS swap keyfile + luksAddKey
│   ├── hardening.go                 # AppArmor, sysctl, chmod, sed
│   ├── uki.go                       # booster build, efi-mkuki, sbsign
│   ├── zswap.go                     # zswap compressed swap (optional)
│   ├── nix.go                       # Nix package manager (optional)
│   └── utils.go                     # getPartitionSpacer, getDiskSuffix
├── utils/                           # Shared utilities
│   ├── config/
│   │   └── config.go                # Config types + LoadConfig
│   ├── shell/
│   │   └── shell.go                 # RunCommand, RunChrootCommand, RunCommandStrict
│   ├── files/
│   │   └── files.go                 # WriteFileInChroot (creates parent dirs)
│   ├── logic/
│   │   └── logic.go                 # Dynamic rule application
│   └── createConfig/
│       ├── create_values.go         # Interactive wizard (PromptForConfig)
│       └── createValues/
│           └── main.go              # Standalone generator
└── variables/
    ├── variables.yaml               # User config template (runtime)
    └── internalRules.yaml           # Dynamic rules (packages, services, modules)
```

## Installation Flow

### Phase 1: Host Operations (hostPhase)

1. **Load Configuration** (`main.go`)
   - Parse `variables.yaml` (user config)
   - Load `internalRules.yaml` (dynamic rules)
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
   - Configure doas permissions
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
   - Install standard packages from internalRules.yaml
   - Install layout-specific packages
   - Install desktop environment packages (if selected)
   - Install video driver packages (if selected)
   - Install extra packages from user config

2. **Setup Services** (`services.go`)
   - Configure OpenRC runlevels
   - Create shell aliases (ll, sudo=doas, nano=micro)
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

## Key Components

### Disk Layouts

#### btrfsStandardServer (2 disks)
- **Partitioning**: EFI (512MB) + LUKS (rest) on each disk
- **RAID1**: Btrfs RAID1 on LUKS for root filesystem
- **Swap**: LUKS-encrypted swap partition
- **Subvolumes**: 21 subvolumes including @root, @home, @boot, @nix-*

#### btrfsStandard (1 disk)
- **Partitioning**: EFI (512MB) + LUKS (rest)
- **Single**: Btrfs single mode (no RAID)
- **Swap**: Swapfile in @swap subvolume
- **Subvolumes**: 25 subvolumes including @root, @home, @boot, @nix-*

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
- **Memory Safety**: uutils, micro, doas instead of C alternatives

## Configuration Files

### variables.yaml (User Config)
- Disk configuration (disks, layout)
- User credentials (username, password)
- System identity (hostname, timezone, locale)
- Repository settings (mirror, branch)
- Feature toggles (encryption, secureboot, TPM, hardening, zswap, nix)
- Extra packages

### internalRules.yaml (Dynamic Rules)
- Standard packages (hardcoded)
- Disk layout definitions (packages, modules)
- Desktop environment definitions (packages, services, groups)
- Video driver definitions (packages, modules)
- Package-specific rules (groups, services, modules)

## Error Handling

- **Non-interactive mode**: `SKEA_NONINTERACTIVE=1` + `SKEA_NONINTERACTIVE_DECISION=ignore|stop`
- **RunCommand**: Shows error and asks for retry/ignore/abort (interactive) or auto-continues/aborts (non-interactive)
- **RunCommandStrict**: Always returns errors (for critical steps like bootstrap)
- **Logging**: Debug mode shows commands and results (stdout/stderr)

## Testing

- **Harness**: QEMU-based testing with virtual machines
- **Artifacts**: Test outputs in tests/artifacts/ (gitignored)
- **Modes**: Install, boot, shell access for manual testing

## Build & Run

```bash
# Build installer
cd src && go build -o ../feralos_installer .

# Run installer (requires root + live environment)
sudo ./feralos_installer -config variables.yaml

# Run with non-interactive mode
SKEA_NONINTERACTIVE=1 SKEA_NONINTERACTIVE_DECISION=ignore sudo ./feralos_installer -config variables.yaml
```
