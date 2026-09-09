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
│   │   ├── cleanup/cleanup.go        # Cleanup — pre/post host cleanup (swapoff, unmount, LUKS close)
│   │   ├── firstboot/
│   │   │   ├── install_config.go     # WriteInstallConfig → /opt/feralos/config.yaml
│   │   │   ├── secureboot.go         # SetupSecureboot — compile → copy → chmod 755
│   │   │   └── tpm.go                # SetupTpm — compile → copy → chmod 755
│   │   └── createConfig/ (create_values.go, createValues/main.go)
│   ├── hostPhase/
│   │   ├── host_phase.go             # orchestrator: disks→mount→bootstrap→chroot prep
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
│       └── desktops/ (niri.go, desktops.go)
├── tests/
│   ├── fixtures/variables/ (14 templates + README — all scenarios)
│   ├── harness/ (cmd/skea_harness, internal/, variables-*.yaml deprecated shims)
│   └── artifacts/ (gitignored: cache, prepare, runs, state)
├── docs/ (PROJECT.md, INSTALLER.md, guides/, architecture/, adr/, harness/)
├── state/session_state.json
└── handoff/latest.md
```

## Installation Flow

### Entry Point & Phase Isolation

`main.go` is the **entry point only**. It performs configuration collection
and generation, then **delegates** all installation work to the phase
packages. It must NOT interact with disks, cleanup, or the chroot directly.

```
setup.sh (real hardware + harness runner)
  ├─ apt update + apt install golang-go + btrfs-progs/cryptsetup/... (host deps, before build)
  └─ go mod tidy + go build feralos_installer

main.go (installer entry, no host deps)
  ├─ Load variables.yaml + internal_rules.yaml
  ├─ Run wizard (unattended-aware)
  ├─ Apply dynamic rules (logic.ApplyRules)
  └─ Delegate:
       cleanup.Cleanup(conf)         // pre-host cleanup (swapoff, unmount, LUKS close)
       hostPhase.Run(conf)           // host-side: disks→mount→bootstrap→chroot prep
       chrootPhase.Run(conf)         // chroot-side: packages, services, boot, UKI
       firstboot.WriteInstallConfig  // reads cmdline → /opt/feralos/config.yaml
       firstboot.SetupSecureboot     // compile + copy feralos_secureboot
       firstboot.SetupTpm            // compile + copy feralos_tpm
       cleanup.Cleanup(conf)         // final unmount before reboot
```

**Isolation contract:**
- `main` collects/generates config then delegates to five entry points.
- `hostPhase.Run()` owns: disk layout → mount → Alpine bootstrap → chroot prep (steps 1–4 only).
- `chrootPhase.Run()` owns everything executed inside the target chroot (packages, services, boot, UKI).
- `firstboot.*` owns: install config write, secureboot/tpm binary compilation and copy. Runs AFTER chroot (needs cmdline from chrootPhase).
- `cleanup.Cleanup()` runs twice: once before `hostPhase` (clean state) and once at the end (final unmount).
- `hostPhase/internal/*` holds host-side helpers (bind mounts, bootstrap, chroot env). Because it is an `internal` package, only `hostPhase` (and its subpackages) may import it — `main` reaches it exclusively through `hostPhase.Run()`. This is the structural guarantee of host/chroot isolation.

### Phase 1: Host Operations (`hostPhase.Run`)

Invoked by `main` after config generation. Order is fixed:

1. **Disk Partitioning & LUKS Setup** (`hostPhase/disks/`)
   - Selected by `conf.User.DiskLayout` inside `hostPhase.Run()`:
      - **btrfsStandardServer**: 2 disks, RAID1, dual EFI, LUKS swap
      - **btrfsStandard**: 1 disk, LUKS root, swapfile, 25 subvolumes
      - **btrfsRaid0**: 2 disks, RAID0, dual EFI, LUKS swap
      - **btrfsUnencrypted**: 1 disk, no LUKS, direct btrfs, same subvolumes as btrfsStandard
   - Create partition tables, format filesystems, setup LUKS encryption

2. **Mount Virtual Filesystems** (`hostPhase/internal/bind_mounts.go`)
   - Mount /dev, /proc, /sys to chroot directory
   - Make mounts slave to prevent propagation

3. **Alpine Bootstrap** (`hostPhase/internal/bootstrap.go`)
   - Download apk-tools-static
   - Initialize Alpine base system into target
   - Configure APK repositories

4. **Prepare Chroot Environment** (`hostPhase/internal/chroot_env.go`)
   - Write networking config (resolv.conf, hostname, hosts)
   - Write localization config (locale, keymap)
   - Write system limits and subuid/subgid
   - Configure APK repositories
   - Write shell aliases and environment
   - Configure sudo-rs permissions
   - Write kernel modules configuration
   - Write ZRAM and LXC service configs
   - Write bridging sysctl

### Phase 1.5: FirstBoot Setup (`firstboot.*`)

Runs AFTER chrootPhase completes (needs kernel cmdline from chroot).

1. **Write Install Config** (`utils/firstboot/install_config.go`)
   - Reads `/etc/kernel/cmdline` from chroot
   - Persists disk layout, LUKS UUIDs, user info to `/opt/feralos/config.yaml`
   - For autonomous firstBoot binaries.

2. **Setup Secureboot** (`utils/firstboot/secureboot.go`)
   - Compile `feralos_secureboot` binary
   - Copy to `/opt/feralos_secureboot`, chmod 755

3. **Setup TPM** (`utils/firstboot/tpm.go`)
   - Compile `feralos_tpm` binary
   - Copy to `/opt/feralos_tpm`, chmod 755

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
- **debug-run**: `tests/debug_run/` boots an existing run without reinstalling (auto-detects latest run dir)

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
