# Variables Reference

> **User guide** — compilation of `variables.yaml`. Template: `src/variables/variables.yaml`. Fixtures (19 templates): `tests/fixtures/variables/` (see `tests/fixtures/variables/README.md`). Processed by `utils/logic.ApplyRules` + `internal_rules.yaml`.

Full example (current template):

```yaml
variables:
  unattended: true
  ignoreErrors: false               # true = log errors and continue (harness mode); false = abort on first error
  disks:
    - "/dev/sda"
    - "/dev/sdb"
  userName: "skea999"
  password: "Alpine.1"
  hostname: "skea-server"
  timezone: "Europe/Rome"
  locale: "it"
  keymap: "it"
  mirror: "https://dl-cdn.alpinelinux.org/alpine"
  arch: "x86_64"                    # x86_64 only (FeralOS custom repo is x86_64)
  branch: "latest-stable"           # Options: latest-stable, edge, 3.22
  diskLayout: "btrfsStandard"       # Options: btrfsStandardServer, btrfsStandard, btrfsRaid0, btrfsUnencrypted
  desktopEnvironment: "none"        # none | kde | niri | cosmic
  videoDriver: "none"               # none | amdgpu | nvidia | intel | virtio (QEMU: mesa-dri-gallium + mesa-vulkan-virtio)
  sshAuthorizedKey: ""
  enableSecureboot: false
  enableTPM: false
  enableHardening: false
  enableZswap: false
  enableNix: false
  enableSwap: true
  swapSize: auto # auto = 2×RAM (via utils/swap), or explicit 8G / 4096M / 4096K
  enableDebug: false
  enablePodman: false
  enableWebmin: false
  webminListenAll: false            # true = listen on all interfaces, false = localhost only
  enableDinit: true                 # true = dinit init system, false = OpenRC
  extraPackages:
    - incus-feature
    - incus-feature-vm
    - incus-feature-client
    - incus-feature-utils
    - qemu-system-x86_64
```

## Options

### Disk Configuration

| Key | Type | Description |
|-----|------|-------------|
| `disks` | `[]string` | Target disks (`/dev/sda`, `/dev/nvme0n1`). `btrfsStandard`: 1 disk, `btrfsStandardServer`/`btrfsRaid0`: 2 disks. All data erased. |
| `diskLayout` | `string` | `btrfsStandard` (1 disk, LUKS root + swapfile), `btrfsStandardServer` (2 disks, RAID1, LUKS swap), `btrfsRaid0` (2 disks, RAID0, LUKS swap), `btrfsUnencrypted` (1 disk, no encryption, btrfs root, swapfile — VMs/testing). See `docs/architecture/storage.md`. |

### User and Security

| Key | Type | Description |
|-----|------|-------------|
| `userName` | `string` | Regular user, added to `wheel` (sudo-rs). |
| `password` | `string` | Root + user password, also LUKS passphrase. |
| `sshAuthorizedKey` | `string` | SSH public key for passwordless login (`/home/<user>/.ssh/authorized_keys`). |

### System Identity

| Key | Type | Description |
|-----|------|-------------|
| `hostname` | `string` | System hostname (`/etc/hostname`, `/etc/hosts`). |
| `timezone` | `string` | `Europe/Rome`, `America/New_York`, etc. |
| `locale` | `string` | `it`, `en`, `de`, etc. (`it` expanded to `it_IT.UTF-8` — bare `it` crashes `conmon` `setlocale` on musl) |
| `keymap` | `string` | Console keymap `it`, `us`, etc. |

### Repository and Architecture

| Key | Type | Description |
|-----|------|-------------|
| `mirror` | `string` | Alpine mirror `https://dl-cdn.alpinelinux.org/alpine`. |
| `arch` | `string` | `x86_64` (only tested). |
| `branch` | `string` | `latest-stable`, `v3.19`, `edge`, etc. |
| `unattended` | `bool` | `true` skips wizard, uses file as-is. `false` runs interactive `createConfig`. Skips manual input ONLY — does not affect error handling. |
| `ignoreErrors` | `bool` | `true` = best-effort install: command failures are logged (`[ERROR][IGNORED]` in `shell.ExecuteCmd`/`ExecuteCmdArgs`) and installation CONTINUES (sets `SKEA_IGNORE_ERRORS=1` at config load). `false` (default) = abort on first error. Used by the test harness (always forced `true` — the verification phase judges what actually works). |

### Desktop and Video

| Key | Type | Description |
|-----|------|-------------|
| `desktopEnvironment` | `string` | `none` (server), `kde`, `niri`, `cosmic`. `niri` pulls `greetd`+`elogind`+`xdg-user-dirs`/`firewalld` plus 20 APK packages (cosmic-files/edit/screenshot/player/store/settings, okular, eog, cups, cups-filters, avahi, system-config-printer, libreoffice, galculator, firefox, firewalld-gui, gnome-firmware, thunderbird, gnome-disk-utility, kcron, ksystemlog, cosmic-settings) niri pulls dms-shell+quickshell+session stack as APK (feralos-aports); **NetworkManager** network service (`networkmanager`+`-cli`+`-tui`+`networkmanager-dinit`, `/etc/network/interfaces` lo-only, `10-manage.conf managed=true`, plugdev/netdev groups, WireGuard via NM, wpa_supplicant); **wayvnc** remote desktop (`wayvnc`+`wayvnc-user-dinit`, user dinit, loopback 127.0.0.1:5900 default — RustDesk/flatpak removed); scanner stack (`sane`/`sane-backends`/`sane-utils`/`sane-airscan`/`simple-scan`) + `qt6-qtmultimedia`/`i2c-tools`/`wtype` (DMS optdepends); polkit+cupsd+networkmanager services; audio+plugdev+netdev groups; no nix dependency (`enableNix` is now an independent feature). `cosmic` uses `seatd` (not elogind) + 19 APK packages (cosmic-session/term/greeter/edit/player/store/settings/screenshot/icons/initial-setup + elogind dependencies replaced with seatd). See `internal_rules.yaml:desktopEnvironments.niri`, `docs/guides/niri.md`. |
| `videoDriver` | `string` | `none`, `amdgpu`, `nvidia`, `intel`, `virtio` (QEMU: `mesa-dri-gallium` + `mesa-vulkan-virtio`) — adds `mesa`, `vulkan`, `xf86-*` (rules-driven via `internal_rules.yaml:videoDrivers`). |

### Feature Toggles

| Key | Type | Description |
|-----|------|-------------|
| `enableSecureboot` | `bool` | Compile `feralos_secureboot` → `/opt/feralos_secureboot`, enroll sbctl + sign UKI. Run manually on hardware, harness auto. |
| `enableTPM` | `bool` | Compile `feralos_tpm` → `/opt/feralos_tpm`, bind LUKS → TPM2 via clevis. Always passes swtpm to QEMU. |
| `enableHardening` | `bool` | AppArmor, sysctl, permissions. Includes `enableDebug` false path. |
| `enableZswap` | `bool` | Zswap compressed swap, `vm.min_free_kbytes` 1% RAM. Server always, others toggle. |
| `enableNix` | `bool` | Nix package manager, `/nix` subvolumes, daemon. Build sandbox: upstream default (`/tmp`, exec since round 48 — no workaround). `trusted-users = root @wheel` (NixOS default — wheel is root-equivalent). |
| `enableSwap` | `bool` | `btrfsStandard`/`btrfsUnencrypted`/`btrfsRaid0`: enable swap (file or LUKS partitions). `btrfsStandardServer`: always true (LUKS swap). `swapSize` only matters when true. |
| `swapSize` | `string` | `auto` (2×RAM) or explicit `8G`/`4096M`/`4096K`. Split `/2` per disk on `btrfsRaid0`. |
| `enableDebug` | `bool` | **Test-only flag** — real hardware must stay `false` (unsafe: weakens hardening). `true`: `loglevel=7`, `dmesg_restrict=0`, permissive printk/io_uring/memfd values (via `DebugOverrides`) + `console=ttyS0,115200` appended to kernel cmdline (serial console for harness boot watch + LUKS passphrase automation). `false`: hardened values, no serial console. All harness fixtures run with `true`. |
| `enablePodman` | `bool` | Podman rootless containers, subuid/subgid, `podman`+`cgroups` services, `btrfs` storage driver (`sed` on `storage.conf`), `iptables`+`podman-compose`+`podman-docker`+`fuse-overlayfs`. |
| `enableWebmin` | `bool` | Webmin **podman-based** web admin panel (requires podman). Standalone binary `src/firstBoot/webmin/main.go` (verifies podman, seeds config, `podman compose up -d`), image `demosthenesk/webmin:noble` (was `pschatzmann` dead), compose `read_only`+`tmpfs` (`/tmp /run /var/log /var/webmin`)+`cap_drop ALL`, seed `/etc/webmin` via `webmin-seed` container (bind mount shadow), oneshot `webmin-ensure`, `changepass.pl`+`chpasswd`. Modules: firewall, packages, btrfs, smart, proc, cron, logs, custom commands. `---` divider in compose docs. |
| `flatpakPackages` | `[]string` | Flatpak apps — set via `variables.yaml` only (niri no longer sets a default; RustDesk removed). Triggers `flatpak` APK + `SetupFlatpak` (flathub remote + `flatpak install -y`). `xdg-user-dirs` + `/lib/modules` before apk required. |
| `webminListenAll` | `bool` | `false`: localhost:10000 only. `true`: 0.0.0.0:10000 (all interfaces). |
| `enableDinit` | `bool` | `true`: dinit init system (boot.d enablement, `/var/log/dinit/` logging, dinit-shutdown poweroff/reboot wiring). `false`: OpenRC. |
| `enableEdgeRepos` | `bool` | Enables the `@edge` tagged repositories (main/community/testing) for `extraPackagesEdge` installs. Optional to set explicitly: a non-empty `extraPackagesEdge` **auto-enables it** (see `extraPackagesEdge`). Repos are NOT left enabled: they are added, used, and commented out per-install (see `extraPackagesEdge`). |

### Extra Packages

| Key | Type | Description |
|-----|------|-------------|
| `extraPackages` | `[]string` | Additional APK packages. Triggers `internal_rules.yaml` → `logic.ApplyRules`: injects `groups`, `services` (runlevels), `modules` (`/etc/modules`), `kernelCmdline`, `mkinitfs` features. Examples: `incus-feature`, `qemu-system-x86_64`. |
| `extraPackagesEdge` | `[]string` | Packages installed from the `@edge` tagged repos, installed LAST (after all other package installs + final upgrade) so the base install completes from the stable repos first. A non-empty list **auto-enables `enableEdgeRepos`** (`applyImpliedFlags` in `src/utils/config/config.go`) — no separate flag needed. `@tag` suffix allowed per package. First use: the mesa stack from edge (26.2.4 restores the virgl gallium driver dropped in 3.24 26.1.6) for GL-on-virtio in QEMU guests. Owner: `src/chrootPhase/features/edgerepos.go` (`SetupEdgeRepos`, called from the tail of `SetupPackages`). 3-step sequence: append tagged `@edge` repos → install the packages → comment the repos back out (apk v3 masks tagged-repo packages from dependency resolution — edge repos must not exist during the main stable batch install). |

## Compilation

1. **Generator** (`src/utils/createConfig/create_values.go`) starts from `src/variables/variables.yaml` template + `internal_rules.yaml` (enum options). Wizard asks **every** option — general, layout, disks, user, passwords, identity, repo, network/ssh, desktop, all `Enable*` toggles, `swapSize`, `extraPackages` — validates **each answer immediately** (invalid input re-asks the same field with the error), cross-checks disk count vs layout, then prints a summary: `yes` proceeds, `no` restarts the whole loop with the previous answers as defaults. Passwords: min 6 chars, masked input (plain read when stdin is not a tty), **asked twice with confirmation** (mismatch = both re-asked); LUKS passphrase empty = reuse user password (empty skips confirmation). Writes `variables.yaml` (`0600`). The INSTALLER runs the same wizard (`createConfig.PromptForConfig`): a config file with **invalid values does not abort** — the wizard starts pre-filled from the file so values can be corrected, then the installer re-validates and aborts only if still invalid (unattended runs skip the wizard, so they abort on invalid input).
   - Per-field validators live in `src/utils/config/validators.go` — single source shared with `LoadConfig` validation (`validateConfig`). Enum options come from `internal_rules.yaml` keys (`diskLayouts`, `desktopEnvironments`, `videoDrivers` + `none`).
2. **Installer** (`src/main.go`) loads `variables.yaml` + `internal_rules.yaml`, `logic.ApplyRules(conf)` injects groups/services/modules per package, validates `len(disks)`.
3. **Harness fixtures** (`tests/fixtures/variables/*.yaml`) are curated 19 templates covering minimal/noswap/explicit/full/isolate per layout + 3 new per-layout full_stack (raid0, server, unencrypted) and `btrfs_standard_niri_full_test.yaml`.

See `docs/architecture/feature_confinement.md` for where each `Enable*` is owned.
