# Getting Started — Real Hardware

> Source: `AGENTS.md#Stack` is source of truth. Details: `docs/INSTALLER.md`, `docs/guides/variables.md` (full `variables.yaml` reference), `docs/architecture/storage.md` (layouts).

FeralOS installer — Go 1.22 + YAML, Alpine-based, LUKS2 + Btrfs + booster + UKI (no GRUB). Runs once, <10 min.

## Prerequisites

- **Disks**: `btrfsStandardServer` → 2× ≥8 GB disks (RAID1, dual EFI, LUKS swap); `btrfsStandard` / `btrfsRaid0` / `btrfsUnencrypted` → 1× or 2× disks respectively. All data on target disks erased.
- **USB stick**: ≥2 GB for live ISO.
- **Live env**: Alpine (recommended) or Void. Must boot UEFI, not Legacy BIOS. Network required (APK + apk-tools-static download).
- **CPU**: x86_64 (AMD/Intel). RAM ≥2 GB.

## Quick Start

**Option A — Automated (curl, recommended for hardware):**

```bash
sudo bash <(curl -fsSL https://raw.githubusercontent.com/skem/skeaAlpineInstaller/latest-stable/setup.sh)
# follows wizard, builds feralos_installer, runs install
```

**Option B — Manual (clone + build):**

1. **Boot live** — flash Alpine/Void ISO to USB, boot UEFI, connect network.

2. **Install deps**:

   ```bash
   # Alpine
   apk add git go

   # Void
   xbps-install -S git go
   ```

3. **Clone + build**:

   ```bash
   git clone https://github.com/skem/skeaAlpineInstaller.git
   cd skeaAlpineInstaller
   cd src && go build -o ../feralos_installer . && cd ..
   ```

4. **Run** (root required):

   ```bash
   # wizard (prompts for missing values)
   sudo ./feralos_installer

   # or with pre-filled config
   sudo ./feralos_installer -config variables.yaml
   # also: src/variables/variables.yaml, tests/fixtures/variables/*.yaml (14 templates)
   ```

   Wizard asks: target `disks` → `diskLayout` (`btrfsStandard`/`btrfsRaid0`/`btrfsStandardServer`/`btrfsUnencrypted`) → `hostname` → `userName`/`password` (also LUKS passphrase) → `desktopEnvironment` (`none`/`niri`/`kde`/`gnome`) → optional `Enable*` flags. Full list + `swapSize`/`extraPackages` → `docs/guides/variables.md`; rules → `internal_rules.yaml`.

5. **Wait** — partitioning → LUKS → Btrfs → bootstrap → chroot → UKI. ~5-10 min. Reboot, remove USB.

Non-interactive: `SKEA_NONINTERACTIVE=1 SKEA_NONINTERACTIVE_DECISION=ignore sudo ./feralos_installer -config variables.yaml` (see `docs/INSTALLER.md#non-interactive`)

## Post-Install — First Boot

System boots UKI directly (`BOOTX64.EFI`), prompts LUKS passphrase, starts OpenRC.

Verify:

```bash
lsblk -f                          # EFI + LUKS + btrfs
sudo btrfs filesystem show        # RAID1 (server) or single/RAID0
sudo cryptsetup status /dev/mapper/luks-*  # LUKS2 active
cat /etc/kernel/cmdline           # zswap/hardening args
sudo sbctl status                 # only if EnableSecureboot
```

If `EnableSecureboot` / `EnableTPM` was set, installer only compiled binaries — run manually on real hardware:

```bash
sudo /opt/feralos_secureboot   # installs sbctl, generates keys, enrolls, signs UKI
sudo /opt/feralos_tpm          # binds LUKS to TPM2 (clevis), rebuilds initramfs+UKI
sudo reboot
```

Config for both: `/opt/feralos/config.yaml` (disk layout, LUKS UUIDs). Harness runs these automatically on first target boot; hardware requires manual run. See `AGENTS.md#FirstBoot Architecture`, `docs/architecture/firstboot.md`.

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| **EFI not booting** | Check UEFI boot order, `efibootmgr -v`; verify `BOOTX64.EFI` on EFI partition (`mount /dev/sdX1 /mnt && ls /mnt/EFI/BOOT/`). Re-run `efibootmgr --create` or copy fallback. See `docs/INSTALLER.md#Boot Process`. |
| **LUKS passphrase rejected** | Same as user `password` in `variables.yaml`. Check keymap (`loadkeys`). Verify `cryptsetup luksDump /dev/sdX2`. |
| **Bootstrap/apk fails** | Check network, `mirror` in `variables.yaml` (`https://dl-cdn.alpinelinux.org/alpine`), DNS (`cat /etc/resolv.conf`). |
| **No logs** | Vector sinks: `/var/log/messages`, `auth.log`, `daemon.log`, `kern.log` (socket `/run/vector/syslog.sock` → `/dev/log`). Service: `rc-service vector status`. Config `/etc/vector/vector.yaml` written after apk (overwrites demo). |
| **QEMU-only noise** | `smartd`/`fsck` warnings whitelisted in harness; ignore on hardware unless persistent. |

Logs: installer stdout = `shell.ExecuteCmd` with CMD/result. Harness artifacts: `tests/artifacts/harness/runs/*/`.

## Uninstall

Boot live USB, repartition target disks (`parted`, `wipefs -a`, `blkdiscard`) or run installer with different layout. **Erases all data on selected disks.**

## References

- `AGENTS.md` — Stack, Build & Test, isolation, logging (source of truth)
- `docs/INSTALLER.md` — full flow, disk layouts, security, project structure
- `docs/guides/variables.md` — complete `variables.yaml` reference (compilation + options)
- `docs/guides/first_boot.md` — Secure Boot + TPM post-install
- `docs/harness/README.md` — QEMU harness (automated install + verify)
