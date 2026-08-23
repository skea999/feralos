# FeralOS Installer Usage Guide

## Overview

FeralOS is a hardened Alpine Linux distribution with LUKS encryption, Btrfs RAID1/single, booster initramfs, and signed Unified Kernel Image (UKI). This guide explains how to use the installer.

## Prerequisites

### Hardware Requirements
- **CPU**: x86_64 processor (AMD or Intel)
- **RAM**: 2GB minimum (4GB recommended)
- **Storage**: 
  - btrfsStandardServer: 2 disks (any size, will be partitioned)
  - btrfsStandard: 1 disk (any size, will be partitioned)
- **Network**: Internet connection for package downloads

### Software Requirements
- **Live Environment**: Linux Mint or Debian (running from USB)
- **Root Access**: Required for installation
- **Boot Mode**: UEFI (not Legacy BIOS)

## Quick Start

### Option 1: Automated Setup (Recommended)

1. Boot from Linux Mint or Debian live USB
2. Open terminal and run:

```bash
sudo bash <(curl -fsSL https://raw.githubusercontent.com/skem/skeaAlpineInstaller/latest-stable/setup.sh)
```

3. Follow the interactive prompts
4. Wait for installation to complete
5. Reboot and remove live USB

### Option 2: Manual Installation

1. Boot from Linux Mint or Debian live USB
2. Open terminal and run:

```bash
# Install dependencies
sudo apt update
sudo apt install -y git golang curl tar dosfstools rsync efitools sbsigntool btrfs-progs cryptsetup

# Clone repository
git clone --branch latest-stable https://github.com/skem/skeaAlpineInstaller.git
cd skeaAlpineInstaller/src

# Build installer
go build -o ../feralos_installer .

# Configure installer
cd ..
sudo nano src/variables/variables.yaml

# Run installer
sudo ./feralos_installer -config src/variables/variables.yaml
```

## Configuration

### variables.yaml

The installer is configured via `src/variables/variables.yaml`. Here's a complete example:

```yaml
variables:
  # Disk Configuration
  disks:
    - "/dev/sda"
    - "/dev/sdb"

  # User and Security
  userName: "skea999"
  password: "Alpine.1"

  # System Identity
  hostname: "skea-server"
  timezone: "Europe/Rome"
  locale: "it"
  keymap: "it"

  # Repository and Architecture
  mirror: "https://dl-cdn.alpinelinux.org/alpine"
  arch: "x86_64"
  branch: "latest-stable"

  # Layout
  diskLayout: "btrfsStandardServer"  # or "btrfsStandard"
  desktopEnvironment: "none"         # or "kde", "niri"
  videoDriver: "none"                # or "amdgpu", "nvidia", "intel"

  # SSH (optional)
  sshAuthorizedKey: ""

  # Feature Toggles
  enableEncryption: true
  enableSecureboot: true
  enableTPM: true
  enableHardening: true
  enableZswap: true
  enableNix: true
  enableDebug: false

  # Extra Packages (optional)
  extraPackages:
    - incus-feature
    - incus-feature-vm
    - incus-feature-client
    - incus-feature-utils
```

### Configuration Options

#### Disk Configuration
- **disks**: List of target disks (e.g., `["/dev/sda", "/dev/sdb"]`)
- **diskLayout**: Storage layout
  - `btrfsStandardServer`: 2 disks, RAID1, dual EFI, LUKS swap
  - `btrfsStandard`: 1 disk, LUKS root, swapfile, 25 subvolumes

#### User and Security
- **userName**: Regular user account name
- **password**: Root and user password (same for both)
- **sshAuthorizedKey**: SSH public key for passwordless login

#### System Identity
- **hostname**: System hostname
- **timezone**: Timezone (e.g., `Europe/Rome`, `America/New_York`)
- **locale**: Language locale (e.g., `it`, `en`, `de`, `fr`)
- **keymap**: Console keymap (e.g., `it`, `us`, `de`, `fr`)

#### Repository and Architecture
- **mirror**: Alpine package mirror URL
- **arch**: System architecture (usually `x86_64`)
- **branch**: Alpine release branch (e.g., `latest-stable`, `v3.24`)

#### Desktop Environment
- **desktopEnvironment**: Desktop environment to install
  - `none`: No desktop (server)
  - `kde`: KDE Plasma desktop
  - `niri**: Niri Wayland compositor (with greetd, foot, pipewire)

#### Video Driver
- **videoDriver**: Graphics driver to install
  - `none`: No specific driver
  - `amdgpu**: AMD GPU driver
  - `nvidia`: NVIDIA proprietary driver
  - `intel`: Intel GPU driver

#### Feature Toggles
- **enableEncryption**: Enable LUKS2 disk encryption (recommended: true)
- **enableSecureboot**: Enable Secure Boot with signed UKI (recommended: true)
- **enableTPM**: Enable TPM2 auto-unlock (recommended: true)
- **enableHardening**: Enable security hardening (recommended: true)
- **enableZswap**: Enable zswap compressed swap (recommended: true)
- **enableNix**: Enable Nix package manager (recommended: true)
- **enableDebug**: Enable debug logging (recommended: false)

#### Extra Packages
- **extraPackages**: Additional packages to install (triggers dynamic rules)

## Disk Layouts

### btrfsStandardServer (2 disks)

**Use case**: Server with redundant storage

**Partitioning**:
- EFI partition (512MB) on each disk
- LUKS partition (rest of disk) on each disk
- Btrfs RAID1 on LUKS for root filesystem
- LUKS-encrypted swap partition

**Subvolumes** (21):
- @root, @home, @boot, @etc, @var
- @var-log, @var-tmp, @var-opt, @var-spool
- @tmp, @swap
- @nix-store, @nix-var-nix, @nix-var-log
- @nix-var-nix-build
- @snapshots
- @docker, @incus
- @srv

**Advantages**:
- Redundant storage (disk failure protection)
- Dual EFI partitions (backup boot)
- LUKS-encrypted swap

### btrfsStandard (1 disk)

**Use case**: Desktop or single-disk server

**Partitioning**:
- EFI partition (512MB)
- LUKS partition (rest of disk)
- Btrfs single mode for root filesystem
- Swapfile in @swap subvolume

**Subvolumes** (25):
- @root, @home, @boot, @etc, @var
- @var-log, @var-tmp, @var-opt, @var-spool
- @tmp, @swap
- @nix-store, @nix-var-nix, @nix-var-log
- @nix-var-nix-build
- @snapshots
- @docker, @incus
- @srv
- @cache, @crash

**Advantages**:
- Simpler setup (single disk)
- No RAID overhead
- Swapfile (easier to resize)

## Security Features

### LUKS2 Encryption
- **Root partition**: Encrypted with LUKS2
- **Swap**: Encrypted (btrfsStandardServer) or swapfile (btrfsStandard)
- **Keyfile**: Generated for swap auto-unlock
- **Password**: Same for LUKS and user login

### Secure Boot
- **UKI**: Unified Kernel Image signed with custom keys
- **Fallback**: BOOTX64.EFI for recovery
- **Dual EFI**: Backup EFI partition (btrfsStandardServer)

### Hardening
- **AppArmor**: Mandatory access control
- **Sysctl**: Kernel hardening parameters
- **File Permissions**: Strict permissions on sensitive files
- **Modules**: Secure kernel modules loaded

### Memory Safety
- **uutils**: Rust-based coreutils replacement
- **micro**: Go-based text editor
- **doas**: Minimal sudo replacement
- **ripgrep**: Rust-based grep replacement
- **fd**: Rust-based find replacement

## Post-Installation

### First Boot
1. Remove live USB
2. System boots from EFI partition
3. Enter LUKS password when prompted
4. System starts OpenRC init system
5. Login with username and password

### Initial Setup
```bash
# Update system
sudo apk update && sudo apk upgrade

# Check services
sudo rc-status

# Check disk usage
df -h

# Check btrfs filesystem
sudo btrfs filesystem usage /

# Check LUKS status
sudo cryptsetup status /dev/mapper/luks-*
```

### Common Tasks
```bash
# Add new user
sudo adduser newuser
sudo usermod -aG wheel newuser

# Install package
sudo apk add package-name

# Enable service
sudo rc-update add service-name default

# Start service
sudo rc-service service-name start

# Check logs
sudo tail -f /var/log/messages
```

## Troubleshooting

### Boot Issues
1. **No boot menu**: Check EFI partition and BOOTX64.EFI
2. **Kernel panic**: Check kernel cmdline in /etc/kernel/cmdline
3. **LUKS prompt not appearing**: Check booster initramfs

### Installation Issues
1. **Bootstrap fails**: Check network connection and mirror URL
2. **Package installation fails**: Check APK repositories
3. **LUKS setup fails**: Check disk availability and password

### Performance Issues
1. **Slow boot**: Check booster initramfs generation
2. **High memory usage**: Check zswap configuration
3. **Disk I/O**: Check btrfs compression and mount options

## Advanced Usage

### Non-Interactive Installation
```bash
# Set environment variables
export SKEA_NONINTERACTIVE=1
export SKEA_NONINTERACTIVE_DECISION=ignore  # or "stop"

# Run installer
sudo ./feralos_installer -config src/variables/variables.yaml
```

### Debug Mode
```bash
# Enable debug logging
sudo ./feralos_installer -config src/variables/variables.yaml
# Edit variables.yaml and set enableDebug: true
```

### Custom Mirror
```bash
# Use local mirror
sudo ./feralos_installer -config src/variables/variables.yaml
# Edit variables.yaml and set mirror: "http://local-mirror/alpine"
```

### Testing with QEMU
```bash
# Run harness
cd tests/harness
go run ./cmd/skea_harness run --keep-target-running

# Access target
ssh -p 2222 skea@localhost
```

## Uninstallation

To uninstall FeralOS:
1. Boot from live USB
2. Run installer with different configuration
3. Or manually partition and format disks

**Warning**: This will erase all data on target disks!

## Support

- **Issues**: GitHub Issues
- **Documentation**: docs/ directory
- **Source Code**: src/ directory
