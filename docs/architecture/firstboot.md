# FirstBoot Architecture

Source: `AGENTS.md#FirstBoot Architecture` + `docs/FIRST_BOOT.md` (double docs — AGENTS.md is source of truth).

## Standalone Binaries

Activatable features use autonomous Go binaries in `src/firstBoot/`:

- `src/firstBoot/secureboot/main.go` — installs packages, generates EFI keys, enrolls `sbctl`, builds+signs UKI, writes hooks
- `src/firstBoot/tpm/main.go` — checks TPM `/dev/tpmrm0`, installs `clevis@edge` + `tpm2-tools`, binds each LUKS device to TPM2 (empty PCR `{}`), removes keyfile, rebuilds initramfs+UKI

Binaries are autonomous — installer does NOT run them.

## Installer Flow

Compiled and copied by `main` after `chrootPhase.Run()` completes:

1. `firstboot.WriteInstallConfig(conf)` reads `/etc/kernel/cmdline` → writes `/opt/feralos/config.yaml` — disk layout, LUKS UUIDs, user info
2. `firstboot.SetupSecureboot(conf)` → compiles `feralos_secureboot`, copies to `/opt/feralos_secureboot`, chmod 755 (guarded `EnableSecureboot`)
3. `firstboot.SetupTpm(conf)` → compiles `feralos_tpm`, copies to `/opt/feralos_tpm`, chmod 755 (guarded `EnableTPM`)

Output: `/opt/feralos_secureboot`, `/opt/feralos_tpm`, `/opt/feralos/config.yaml` inside target.

FirstBoot binaries read `/opt/feralos/config.yaml` for LUKS UUIDs (fallback: kernel cmdline).

## 3-Boot Harness Flow

`tests/harness` — swtpm always passed to runner + target VMs even if `EnableTPM=false`.

1. **Runner (Debian)**: installer runs, compiles binaries, writes config. swtpm present.
2. **Target (Alpine) — first boot**: harness SSH runs `feralos_secureboot` + `feralos_tpm`. Binaries install packages, configure features, bind LUKS to TPM2. swtpm present.
3. **Target (Alpine) — verification**: harness checks `sbctl status`, `clevis luks list`, LUKS auto-unlock. swtpm present.

`BootTimeout` 10min for 3-boot with swtpm.

## Real Hardware

User runs manually after first boot:

```bash
/opt/feralos_secureboot   # before enabling Secure Boot in BIOS
# reboot → BIOS: enable Secure Boot, Custom mode, save
/opt/feralos_tpm          # binds LUKS; SKEA_LUKS_PASSWORD env avoids prompt
```

See `docs/FIRST_BOOT.md#Usage on Real Hardware` for BIOS steps, env `SKEA_LUKS_PASSWORD`, troubleshooting.

## swtpm Always Present

Simplifies harness — no conditional QEMU args. All runs include swtpm device.

## Known Limitation

- **Secure Boot enrollment cannot be confirmed in QEMU** — no real `efivarfs`. Harness skips sbctl verification in VM.
- **TPM auto-unlock CAN be confirmed** — `clevis luks list -d /dev/<disk>p3` shows `tpm2` binding, works reliably with swtpm.

## References

- `AGENTS.md#FirstBoot Architecture`
- `docs/FIRST_BOOT.md`
- `docs/INSTALLER.md#Phase 1.5: FirstBoot Setup`
- `src/firstBoot/secureboot/main.go:1`, `src/firstBoot/tpm/main.go:1`
- `src/utils/firstboot/secureboot.go`, `src/utils/firstboot/tpm.go`
