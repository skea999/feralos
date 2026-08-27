# First Boot — Secure Boot & TPM

> **User guide** — part of `docs/guides/` (user installer guides). For development flow see `docs/guides/development.md`.

Standalone Go programs for post-installation setup on real hardware. Compiled in `utils/firstboot/` (`src/utils/firstboot/secureboot.go` + `tpm.go`) → `/opt/feralos_*`, autonomous on first boot.

## Programs

| Program | Source | Purpose |
|---------|--------|---------|
| `feralos_secureboot` | `src/firstBoot/secureboot/main.go` | Enroll sbctl keys, build+sign UKI, write hooks |
| `feralos_tpm` | `src/firstBoot/tpm/main.go` | Bind LUKS to TPM2 via clevis, rebuild initramfs+UKI |

On **QEMU harness** both are run automatically via SSH on first target boot (swtpm always present). On **real hardware** run manually after first boot.

## Building (from `src/`)

```bash
go build -o ../feralos_secureboot ./firstBoot/secureboot/
go build -o ../feralos_tpm ./firstBoot/tpm/
# or via main: compiled automatically during install to /opt/feralos_*
```

## Real Hardware — Prerequisites

- FeralOS installed, boot to installed system (first boot)
- Root access, network for `apk add` (if needed), TPM/Secure Boot support if enabled

## Step 1: Secure Boot Enrollment (first boot, before BIOS enable)

```bash
# binaries already at /opt/feralos_secureboot after install
sudo /opt/feralos_secureboot
# or if copied:
# scp feralos_secureboot root@<target>:/root/ && ssh root@<target> /root/feralos_secureboot
```

What it does:
1. Installs `sbctl` (temp)
2. Mounts `efivarfs` if needed
3. Imports EFI certificates from `/etc/uefi-keys/`
4. Enrolls keys via `sbctl enroll-keys -m`
5. Removes `sbctl`

After enrollment:
1. Reboot → BIOS → Security → Secure Boot → Enable ON, Mode Custom → Save & Exit → Reboot

## Step 2: TPM Auto-Unlock (second boot, after Secure Boot enabled)

```bash
sudo /opt/feralos_tpm
# env var to avoid prompt:
# SKEA_LUKS_PASSWORD="..." sudo /opt/feralos_tpm
```

What it does:
1. Checks `/dev/tpmrm0`
2. Reads LUKS UUIDs from `/opt/feralos/config.yaml` (fallback `/etc/kernel/cmdline`)
3. Adds `@edge` repo temp, installs `clevis@edge` + `tpm2-tools`
4. Binds each LUKS device to TPM2 (empty PCR `{}`)
5. Removes temp keyfile, rebuilds booster initramfs + signed UKI

After binding: reboot → auto-unlock without passphrase (fallback to prompt if TPM fails).

## Harness Flow (QEMU, automated)

1. **Runner (Debian)** — installer compiles binaries → `/opt/feralos_*`, writes `/opt/feralos/config.yaml`
2. **Target first boot** — harness runs both binaries via SSH (swtpm always)
3. **Target verification** — harness checks `sbctl status`, `clevis luks list`, LUKS auto-unlock

swtpm always passed to runner + target (even if `EnableTPM=false`). QEMU cannot confirm Secure Boot enrollment (no real efivarfs), TPM auto-unlock CAN be confirmed.

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SKEA_LUKS_PASSWORD` | LUKS passphrase for non-interactive TPM bind |

## Troubleshooting

**Secure Boot — `sbctl enroll-keys` fails:** BIOS → Custom Mode → Delete All Keys → Save & Exit → Reboot → rerun.

**No boot after Secure Boot enable:** BIOS → Disable Secure Boot → boot → rerun → re-enable.

**TPM — `TPM not found at /dev/tpmrm0`:** BIOS → TPM Configuration → Enable TPM Device.

**LUKS auto-unlock fails:** Enter passphrase, `clevis luks list -d /dev/nvme0n1p3` — no `tpm2` → rerun `feralos_tpm`.

## Notes

- PCR policy empty `{}` — TPM identity only, reliable across reboots (PCR7 mismatch avoided)
- Kernel: `vmlinuz-lts` + `booster-lts.img` assumed
- EFI keys must exist at `/etc/uefi-keys/` (PK, KEK, db)

---
*See `docs/architecture/firstboot.md` for hostPhase/firstBoot design and `docs/guides/getting_started.md` for real hardware install.*
