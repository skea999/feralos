# FeralOS Installer Architecture

## Overview

The FeralOS installer is a Go-based system installer that creates a hardened Alpine Linux system with LUKS encryption, Btrfs RAID1/single, booster initramfs, and signed Unified Kernel Image (UKI). It replaces GRUB entirely — the system boots directly from the UKI. The init system is selectable: **dinit** (`enableDinit`, the configuration template default) with **OpenRC co-installed as fallback** — a dinit install must remain bootable under OpenRC.

## Core Architecture Principles

- **Standard core, pluggable data**: Installer logic never changes; only data (YAML + Go layouts) changes
- **Phase separation**: Host operations (hostPhase) run on the host, chroot operations (chrootPhase) run inside the chroot
- **Modular design**: Each functionality is a separate module for easy review and maintenance
- **Memory safety**: Uses memory-safe alternatives where possible (uutils, micro, sudo-rs)

## Project Structure

Full tree — `AGENTS.md` keeps only pointer. Details here:

```text
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
│       ├── features/ (hardening.go, zswap.go, zswap_server.go, nix.go, podman.go, edgerepos.go)
│       └── desktops/ (niri.go, niri_wayle.go, desktops.go)
├── tests/
│   ├── fixtures/variables/ (30 fixtures, full template mirror + README — all scenarios)
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

```text
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
      - **btrfsStandard**: 1 disk, LUKS root, swapfile, 24 subvolumes
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
   - Dinit path: install the dinit core (dinit, dinit-chimera, getty-dinit, networking-dinit, dinit-boot-log, klogd-dinit, dinit-shutdown, busybox-loadkmap-dinit) and convert each installed `-openrc` subpackage to its `-dinit` equivalent (skips per `dinitReplacedByChimera` + `dinitSkipServices` in internal_rules.yaml)

2. **Setup Services** (`services.go`)
   - OpenRC path: enable services via `rc-update add <svc> <runlevel>` from the rules `services` map — runs ALWAYS (never conditional), so a dinit install stays OpenRC-bootable (the links are inert under dinit)
   - Dinit path (enableDinit): create `boot.d` symlinks in `/etc/dinit.d/boot.d/` (admin dir processed by `boot` via waits-for) → target resolved in `/lib/dinit.d/<svc>` first, then `/etc/dinit.d/<svc>` (the turnstile unit ships in `/etc/dinit.d`)
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
- **Subvolumes**: 24 subvolumes including @root, @home, @boot, @nix-*
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
6. **Init**: dinit via `init=/usr/libexec/dinit/init` (enableDinit — removing `init=` boots OpenRC again, no point of no return) or OpenRC fallback

## Init System — dinit (selectable) + OpenRC fallback

`enableDinit` (variables.yaml, template default) switches the init system. OpenRC is
ALWAYS co-installed and its enablement is never conditional: every `rc-update` link
is created on both inits (inert under dinit), so a dinit install remains bootable
under OpenRC. The switch is the kernel cmdline `init=` — no point of no return.

### Boot suite

- **dinit** from Alpine community (0.21.0, built with `--shutdown-prefix=dinit-`)
- **dinit-chimera** upstream suite: 52 services + compiled helpers (early chain:
  pseudo-fs → `early-fs-fstab` → `early-devd` → `early-root-rw` → `pre-local` →
  `local.target`, then `boot` → `system` → `login.target`, `network.target`,
  `pre-network.target`)
- FeralOS glue packages (feralos-aports): `dinit-devd` (eudev hook, mandatory),
  `dinit-cryptdisks` (optional), `getty@`, `networking-dinit`, `dinit-boot-log`
  (mirrors the PID1 trail to `/var/log/dinit/boot.log`), `klogd-dinit` (kernel
  ring → `/dev/log` for vector), `dinit-shutdown` (see below), `busybox-loadkmap-dinit`
- User sessions: **turnstile** (chimera project) spawns a per-user dinit at login;
  session apps (pipewire, dbus, DMS/wayle, portals…) are supervised per-user
  services (`.user` units in `/etc/dinit.d/user/`, boot.d stack under
  `/usr/lib/dinit.d/user/boot.d/`)

### Packages convention

- `-dinit` packages SHIP service files only (`/lib/dinit.d/<svc>`); enablement is
  rules-driven — the `services` map in internal_rules.yaml drives BOTH inits
  (OpenRC: `rc-update add`; dinit: `boot.d` symlink). There is no per-package
  auto-enable.
- During install, every installed `-openrc` subpackage is converted to its `-dinit`
  equivalent (individually, so one missing `-dinit` doesn't block others). Bases
  listed in `dinitReplacedByChimera` are skipped (service already provided by the
  dinit-chimera suite); `dinitSkipServices` covers services that must never be
  converted/enabled (session services, NM-managed, units shipping in main packages).

### Service file layout (ADR-008)

- Unit: `/lib/dinit.d/<svc>` (vendor dir; dinit-chimera overrides the scan dir to
  `/lib/dinit.d` — stock Alpine dinit does NOT scan it)
- Wrapper: `/usr/lib/dinit/<svc>` — POSIX-sh wrapper that sources
  `/etc/dinit.d/config/<svc>.conf` (`[ -r ] && . …`) then execs the daemon with
  `${VARS}` — user-editable config, apk-protected unit (Artix `/etc/conf.d`
  convention restored; defaults match Alpine conf.d)
- Enablement: `/etc/dinit.d/boot.d/<svc>` symlink (admin dir, `boot` waits-for) —
  created by the installer services map (dinitctl `--offline enable` crashes on
  Alpine dinit 0.21.0, so the identical symlinks are created directly)

### Service conventions (ADR-009)

- Daemons: NO `restart = true` (Artix/Chimera never restart) — `smooth-recovery`
  only; scripted one-shots explicit `restart = false` (exceptions: getty keeps
  restart + `termsignal=HUP`; idle-exit daemons like geoclue/fprintd set
  `smooth-recovery = false` so a legitimate exit stops the unit until the next
  activation)
- 33+ services carry `before = login.target` (Artix+Chimera universal convention)
- Network ordering: firewalld/nftables/iptables before `pre-network.target`;
  NetworkManager depends pre-network + before `network.target`; avahi/rsync depend
  networking; apparmor before `local.target` (profiles load BEFORE local services);
  fuse after `early-root-rw.target`
- A dinit unit with `logfile=` under a dedicated fstab subvol (e.g. `@var-log`)
  MUST carry `depends-on = local.target` — subvol mounts complete in the early
  chain, any no-dep unit races them
- Bus-name readiness via `dbus-wait-for` (feralos-aports, Artix dinit-rc helper)

### Shutdown ordering (ADR-007)

- dinit handles only SIGINT=reboot / SIGTERM=halt / SIGQUIT=now — busybox
  `poweroff` (SIGUSR2) is IGNORED; without `dinit-shutdown` PID 1 can never
  finalize any shutdown
- `dinit-shutdown` ships `dinit-poweroff`/`dinit-reboot`/`dinit-halt` + the
  finalizer; the installer wires conventional commands (Artix/Chimera style):
  `poweroff`/`halt`/`shutdown` symlinks + `reboot`/`soft-reboot` wrappers in
  `/usr/bin`; daemon rolls back services then execs `/usr/sbin/dinit-shutdown --system -p/-r/-h`
- elogind execs the ABSOLUTE `/sbin/poweroff` (busybox) after its hooks — the
  installer writes `/etc/elogind/system-shutdown/50-feralos-dinit.sh` (execs the
  dinit binaries per verb BEFORE the busybox no-op); `stop-command` guards on
  `dbus`/`elogind` keep the release order clean at shutdown

### D-Bus activation (ADR-010)

Every D-Bus-activatable daemon is a dinit service; activation only triggers dinit —
nothing dbus-activatable may run unsupervised:

- Unit naming follows who activates the service: user (session) services = canonical
  bus name + `.user`; system services WE boot-enable (initd exists) = the OpenRC
  initd name; system services dbus-activated on demand = canonical bus name
- The effective `.service` file carries
  `Exec=/usr/libexec/feralos/dbus-activate <unit> -- <upstream exec>` — the shim
  forwards the activation env to dinit (`dinitctl setenv`) then `dinitctl start`
  (waits); OpenRC system-bus fallback = raw upstream exec; session bus always user
  dinit (both inits)
- System overrides ship in `/usr/share/feralos/dbus-system-services` and are
  materialized into `/usr/local/share/dbus-1/system-services` at boot by
  `dbus-services-setup` (the setuid launch helper resolves names ONLY against the
  standard dirs; `/etc/dbus-1/system-services` is daemon-enumerated but NOT
  helper-activatable); session overrides ship in `/usr/share/feralos/dbus-services`
  and are materialized at login by the `dbus-session` wrapper
- Full mechanism + new-service checklist: `docs/architecture/dbus-activation.md`


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
- Desktop environment definitions (packages, services, groups, userServices, flatpakPackages)
- Video driver definitions (packages, modules)
- Package-specific rules (groups, services, modules)
- Dinit rules: `defaultRunlevelsDinit.boot` (base boot.d list), `dinitSkipServices` (never convert/enable), `dinitReplacedByChimera` (provided by the chimera suite), `userServices` (per-user boot.d stack)

## Error Handling

- **Non-interactive mode**: `SKEA_NONINTERACTIVE=1` + `SKEA_NONINTERACTIVE_DECISION=ignore|stop`
- **ExecuteCmd**: Unified command execution with logging, retry (3 attempts), and user feedback
- **File writes**: `shell.WriteFile`/`shell.WriteFileAppend` — logged direct writes on the chroot mount (ADR-004); the `echo '<escaped>' > path` shell pattern is forbidden for new code
- **Logging**: Timestamp + command, stdout/stderr, exit code, duration
- **Retry**: After 3 failures, asks user: [s]top, [i]gnore, [r]etry (interactive) or auto-continues/aborts (non-interactive); `ignoreErrors: true` (harness always sets it) = best-effort install, failures logged `[ERROR][IGNORED]` and install continues

## Testing

- **Harness**: QEMU-based testing with virtual machines
- **Artifacts**: Test outputs in tests/artifacts/ (gitignored)
- **Modes**: Install, boot, shell access for manual testing
- **Checks**: per-DE verification suites, init-aware (dinit vs OpenRC) — up to 113 checks on the flagship niri desktop; includes `fstab_vs_mount`, `vector_*`, `apk_packages` dumps, dinit runtime/user-session checks
- **Log hygiene**: `log_errors` fails on any non-allowlisted ERROR/WARNING in installer.log + serial; QEMU-only noise is allowlisted (`smartd`/`fsck`, netns mount/umount early chain, udevd `/bin/false` module installs)
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
