# Dinit Boot Process (Chimera-identical)

> Verified against dinit-chimera master (v0.99.24) — README, `meson.build`, service files,
> early scripts. FeralOS differences are marked **[FeralOS]**.

## Boot Flow

```text
UEFI → UKI (booster + kernel + cmdline)
         │  cmdline: root=… rootfstype=btrfs … init=/usr/libexec/dinit/init [FeralOS path]
         ▼
   booster initramfs
         │  mounts /proc /sys /dev /run
         │  loads modules, unlocks LUKS (TPM/clevis or passphrase)
         │  mounts root ro, switch_root
         ▼
   init wrapper (dinit-chimera `init` script)
         │  sources /etc/dinit/init (overrides)
         │  ulimit -c, auto-recovery/quiet/log-level from kernel cmdline
         │  exec env -i PATH=… HOME=/root dinit
         ▼
    dinit (PID 1) reads /lib/dinit.d/boot   **[FeralOS]** /lib, NOT /usr/lib
         ▼
    boot ──depends-on──▶ system ──▶ login.target + network.target
      │                     │
      │                     └── waits-for.d: /lib/dinit.d/boot.d   (packages)
      └── waits-for.d: /etc/dinit.d/boot.d                              (admin overrides)
          ▼
    early chain (below) → local.target → network.target → login.target
          ▼
    getty / display manager
```

**[FeralOS] Service directory = `/lib/dinit.d` (Alpine standard).** The stock
Alpine `dinit` (0.21) scans only `/etc/dinit.d`, `/run/dinit.d`,
`/usr/local/lib/dinit.d`, `/lib/dinit.d` — it does **NOT** scan `/usr/lib/dinit.d`
(Chimera works because cports patches their dinit; Alpine's is unpatched).
Symptom if services land in `/usr/lib/dinit.d`: `dinit: boot: could not find
service description.` The dinit-chimera APKBUILD overrides meson `srvdir` to
`/lib/dinit.d` in `prepare()`; all FeralOS `*-dinit` packages install there.

Note the two `boot.d` directories — both are upstream conventions:
- `/lib/dinit.d/boot.d` — symlinks dropped by **packages** (our `*-dinit` pkgs)
- `/etc/dinit.d/boot.d` — symlinks created by **admin** (or `dinitctl enable`)

## Early Boot Chain (upstream services, in dependency order)

| Order | Service | What it does | Script/Helper |
|-------|---------|--------------|---------------|
| 1 | `early-env` | activation environment | `env.sh` |
| 2 | `early-pseudofs` | mount /proc /sys /dev /run (skippable: initramfs did it) | `pseudofs.sh` + `mnt` helper |
| 3 | `early-tmpfs` | /run sizing (`dinit.runsize=`) | `tmpfs.sh` |
| 4 | `early-modules-early` | initramfs-stage module load | `modules-early.sh` + `kmod` helper |
| 5 | `early-machine-id` | /etc/machine-id | `machine-id.sh` |
| 6 | `early-kernel-env` | kernel env params | `kernel-env.sh` |
| 7 | `early-tmpfiles-dev` | tmpfiles for /dev | `tmpfiles.sh` + `sd-tmpfiles` |
| 8 | `early-devd` | **distro hook** → start udevd | **[FeralOS]** our `/usr/libexec/dinit-devd` |
| 9 | `early-dev-trigger` | udevadm trigger | hook `trigger` arg |
| 10 | `early-dev-settle` | udevadm settle | hook `settle` arg |
| 11 | `early-hwclock` / `early-swclock` | RTC / swclock | `hwclock`/`swclock` compiled helpers |
| 12 | `early-hostname` | hostname | `hostname.sh` |
| 13 | `early-cryptdisks(-early)` | **distro hook** → crypttab | **[FeralOS]** our `/usr/libexec/dinit-cryptdisks` |
| 14 | `early-lvm` / `early-mdadm` / `early-dmraid` / `early-fs-btrfs` / `early-fs-zfs` | volume setup; **no-op exit 0 if tool missing** | respective scripts |
| 15 | `early-root-fsck` | fsck root (skippable/interruptible, no timeout) | `root-fsck.sh` + `fs-fsck.sh` |
| 16 | `early-root-rw.target` | remount root rw (`starts-rwfs` → dinit opens control socket) | `root-rw.sh` + `mnt` helper |
| 17 | `early-fs-fstab.target` | mount fstab non-network | `fs-fstab.sh` + `mnt` helper |
| 18 | `early-swap` | swapon | `swap.sh` + `swap` helper |
| 19 | `early-rng` | seedrng | `seedrng` helper |
| 20 | `early-sysctl` | sysctl.d | `sysctl.sh` + `sysctl` helper |
| 21 | `early-binfmt` | binfmt.d | `binfmt.sh` + `binfmt` helper |
| 22 | `early-cgroups` | cgroup controllers | `cgroups.sh` |
| 23 | `early-tmpfiles` | full tmpfiles run | `sd-tmpfiles` (exit 65/73 tolerated) |
| 24 | `early-console.target` | keymap/font — **distro hook** (optional) | our `dinit-console` (Step 2e: kbd loadkmap/setfont via `/etc/conf.d/keymaps`,`consolefont`) |
| 25 | `early-net-lo` | loopback up | `net-lo.sh` + `lo` helper |
| 26 | `local.target` / `pre-local.target` | oneshots done, rc.local | `local.sh` |

Everything above is **upstream code** — FeralOS only fills the 2 hook scripts (8, 13).

> **Keymap note**: `dinit-console` reads `/etc/conf.d/keymaps` (short name) and
> looks under `/usr/share/keymaps/` — paths that do not exist with
> `kbd-bkeymaps` (`/usr/share/bkeymaps/<code>/<code>.bmap.gz`). The REAL
> console keymap load is `busybox-loadkmap-dinit`'s `loadkmap` scripted
> service (boot.d-enabled, `depends-on = local.target`): it sources
> `/etc/conf.d/loadkmap` (`KEYMAP=` full path, written by the installer from
> the user `keymap` variable) and pipes the `.bmap.gz` through busybox
> loadkmap. `early-keyboard.target` stays a no-op on FeralOS; the font half
> of `dinit-console` (consolefont) still applies.

## Target Chain

```text
early-prepare.target      pseudo-fs mounted
early-modules.target      /etc/modules loaded
early-devices.target      /dev populated + settled
early-keyboard.target     keymap set (hook)
early-fs-pre.target       crypto/RAID/LVM/btrfs scan up
early-root-rw.target      root rw
early-fs-fstab.target     fstab non-network mounted
early-fs-local.target     local fs complete
pre-local.target          critical oneshots done
local.target              rc.local ran, tmpfiles done
pre-network.target        firewall up
network.target            network daemons may run
login.target              getty/DM may run
time-sync.target          clock set (chronyd waits here)
```

Third-party services (our Phase 3 `*-dinit` packages) depend on these targets —
typically `local.target` (daemons) or `network.target` (network daemons).

## Shutdown

Alpine's `dinit` is built with `--shutdown-prefix=dinit-` → PID 1 execs
`/usr/sbin/dinit-shutdown` at stop, which dispatches to `dinit-poweroff`,
`dinit-reboot`, `dinit-halt`, `dinit-soft-reboot`. dinit-chimera's `shutdown-hook`
unmounts/tears down late state. No FeralOS code needed.

## Kernel Command Line Parameters (upstream, zero code)

**Dinit core**: `dinit_auto_recovery=1`, `dinit_quiet=1`, `dinit_log_level=`,
`dinit_console_level=`, `dinit_log_file=`

**dinit-chimera boot debug**: `dinit_early_debug=1`, `dinit_early_debug_slow=N`,
`dinit_early_debug_log=FILE`, `dinit_early_root_remount=`

**Misc**: `dinit.runsize=` / `initramfs.runsize=` (default 10%),
`fsck.mode=skip|force`, `fsck.repair=yes|no`, `fastboot`, `forcefsck`, `fsckfix`,
`nokdump`, `dinit_skip_volumes`

## FeralOS Cmdline (UKI / /etc/kernel/cmdline)

```text
root=UUID=… rootfstype=btrfs rootflags=subvol=@ … init=/usr/libexec/dinit/init dinit_log_file=/run/dinit-boot.log
```

- `init=/usr/libexec/dinit/init` — switches from OpenRC to dinit
- `dinit_log_file=/run/dinit-boot.log` — full service status trail (read by wrapper via `/proc/cmdline` since booster does not forward unknown params to init's env; mirrored by `dinit-boot-log` to `/var/log/dinit/boot.log`)

Note: regenerate UKI after changing cmdline.
