# Variables Reference

> **User guide** — compilation of `variables.yaml`. Template: `src/variables/variables.yaml`. Fixtures (14 templates): `tests/fixtures/variables/` (see `tests/fixtures/variables/README.md`). Processed by `utils/logic.ApplyRules` + `internal_rules.yaml`.

Full example (current template):

```yaml
variables:
  unattended: true
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
  arch: "x86_64"
  branch: "latest-stable"
  diskLayout: "btrfsStandard" # btrfsStandard | btrfsRaid0 | btrfsStandardServer | btrfsUnencrypted
  desktopEnvironment: "none"        # none | kde | niri | gnome
  videoDriver: "none"               # none | amdgpu | nvidia | intel
  sshAuthorizedKey: ""
  enableSecureboot: false
  enableTPM: false
  enableHardening: false
  enableZswap: false
  enableNix: false
  enableSwap: true
  swapSize: auto # auto = 2×RAM (via utils/swap), or explicit 8G / 4096M / 4096K
  enableDebug: false
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
| `locale` | `string` | `it`, `en`, `de`, etc. |
| `keymap` | `string` | Console keymap `it`, `us`, etc. |

### Repository and Architecture

| Key | Type | Description |
|-----|------|-------------|
| `mirror` | `string` | Alpine mirror `https://dl-cdn.alpinelinux.org/alpine`. |
| `arch` | `string` | `x86_64` (only tested). |
| `branch` | `string` | `latest-stable`, `v3.19`, `edge`, etc. |
| `unattended` | `bool` | `true` skips wizard, uses file as-is. `false` runs interactive `createConfig`. |

### Desktop and Video

| Key | Type | Description |
|-----|------|-------------|
| `desktopEnvironment` | `string` | `none` (server), `kde`, `niri`, `gnome`. `niri` pulls `greetd`+`elogind`. |
| `videoDriver` | `string` | `none`, `amdgpu`, `nvidia`, `intel` — adds `mesa`, `vulkan`, `xf86-*`. |

### Feature Toggles

| Key | Type | Description |
|-----|------|-------------|
| `enableSecureboot` | `bool` | Compile `feralos_secureboot` → `/opt/feralos_secureboot`, enroll sbctl + sign UKI. Run manually on hardware, harness auto. |
| `enableTPM` | `bool` | Compile `feralos_tpm` → `/opt/feralos_tpm`, bind LUKS → TPM2 via clevis. Always passes swtpm to QEMU. |
| `enableHardening` | `bool` | AppArmor, sysctl, permissions. Includes `enableDebug` false path. |
| `enableZswap` | `bool` | Zswap compressed swap, `vm.min_free_kbytes` 1% RAM. Server always, others toggle. |
| `enableNix` | `bool` | Nix package manager, `/nix` subvolumes, daemon. |
| `enableSwap` | `bool` | `btrfsStandard`/`btrfsUnencrypted`/`btrfsRaid0`: enable swap (file or LUKS partitions). `btrfsStandardServer`: always true (LUKS swap). `swapSize` only matters when true. |
| `swapSize` | `string` | `auto` (2×RAM) or explicit `8G`/`4096M`/`4096K`. Split `/2` per disk on `btrfsRaid0`. |
| `enableDebug` | `bool` | Debug logging via `HardeningPackages()`. |
| `enablePodman` | `bool` | Podman rootless containers, subuid/subgid, podman group. |
| `enableWebmin` | `bool` | Webmin web admin panel (requires podman). Modules: firewall, packages, btrfs, smart, proc, cron, logs, custom commands. |
| `webminListenAll` | `bool` | `false`: localhost:10000 only. `true`: 0.0.0.0:10000 (all interfaces). |

### Extra Packages

| Key | Type | Description |
|-----|------|-------------|
| `extraPackages` | `[]string` | Additional APK packages. Triggers `internal_rules.yaml` → `logic.ApplyRules`: injects `groups`, `services` (runlevels), `modules` (`/etc/modules`), `kernelCmdline`, `mkinitfs` features. Examples: `incus-feature`, `qemu-system-x86_64`. |

## Compilation

1. **Generator** (`src/utils/createConfig/create_values.go`) starts from `src/variables/variables.yaml` template, prompts for `disks` → `diskLayout` → `hostname` → `userName`/`password` → `desktopEnvironment` → flags, writes `variables.yaml`.
2. **Installer** (`src/main.go`) loads `variables.yaml` + `internal_rules.yaml`, `logic.ApplyRules(conf)` injects groups/services/modules per package, validates `len(disks)`.
3. **Harness fixtures** (`tests/fixtures/variables/*.yaml`) are curated 15 templates covering minimal/noswap/explicit/full/isolate per layout.

See `docs/architecture/feature_confinement.md` for where each `Enable*` is owned.
