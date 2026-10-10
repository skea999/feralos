# FirstBoot Architecture

Source: `AGENTS.md#FirstBoot Architecture` + `docs/FIRST_BOOT.md` (double docs — AGENTS.md is source of truth).

## Standalone Binaries

Activatable features use autonomous Go binaries in `src/firstBoot/`:

- `src/firstBoot/secureboot/main.go` — installs packages, generates EFI keys, enrolls `sbctl`, builds+signs UKI, writes hooks
- `src/firstBoot/tpm/main.go` — checks TPM `/dev/tpmrm0`, installs `clevis@edge` + `tpm2-tools`, binds each LUKS device to TPM2 (empty PCR `{}`), removes keyfile, rebuilds initramfs+UKI
- `src/firstBoot/webmin/main.go` — verifies `podman --version`, creates `/opt/webmin`, generates `docker-compose.yml` (image `demosthenesk/webmin:noble`, `read_only: true`, `tmpfs: /tmp /run /var/log /var/webmin`), seeds `/etc/webmin` via `webmin-seed` container, starts via `podman compose up -d`, creates `webmin-ensure` oneshot (dual-init: PID1 via `cat /proc/1/comm` → dinit `/etc/dinit.d/webmin-ensure` + `/usr/libexec/dinit-webmin-ensure` + boot.d link, OpenRC `/etc/init.d/webmin-ensure`), sets root password via `changepass.pl` + `chpasswd`

Binaries are autonomous — installer does NOT run them.

## Installer Flow

Compiled and copied by `main` after `chrootPhase.Run()` completes:

1. `firstboot.WriteInstallConfig(conf)` reads `/etc/kernel/cmdline` → writes `/opt/feralos/config.yaml` — disk layout, LUKS UUIDs, user info
2. `firstboot.SetupSecureboot(conf)` → compiles `feralos_secureboot`, copies to `/opt/feralos_secureboot`, chmod 755 (guarded `EnableSecureboot`)
3. `firstboot.SetupTpm(conf)` → compiles `feralos_tpm`, copies to `/opt/feralos_tpm`, chmod 755 (guarded `EnableTPM`)
4. `firstboot.SetupWebmin(conf)` → compiles `feralos_webmin`, copies to `/opt/feralos_webmin`, chmod 755 (guarded `EnableWebmin`)

Output: `/opt/feralos_secureboot`, `/opt/feralos_tpm`, `/opt/feralos_webmin`, `/opt/feralos/config.yaml` inside target.

FirstBoot binaries read `/opt/feralos/config.yaml` for LUKS UUIDs (fallback: kernel cmdline).

## 3-Boot Harness Flow

`tests/harness` — swtpm always passed to runner + target VMs even if `EnableTPM=false`.

1. **Runner (Debian)**: installer runs, compiles binaries, writes config. swtpm present.
2. **Target (Alpine) — first boot**: harness SSH runs `feralos_secureboot` + `feralos_tpm` + `feralos_webmin` (if enabled). Binaries install packages, configure features, bind LUKS to TPM2, start Webmin container. swtpm present.
3. **Target (Alpine) — verification**: harness checks `sbctl status`, `clevis luks list`, LUKS auto-unlock, `podman ps` + `wget https://localhost:10000/`. swtpm present.

`BootTimeout` 10min for 3-boot with swtpm.

## Real Hardware

User runs manually after first boot:

```bash
/opt/feralos_secureboot   # before enabling Secure Boot in BIOS
# reboot → BIOS: enable Secure Boot, Custom mode, save
/opt/feralos_tpm          # binds LUKS; SKEA_LUKS_PASSWORD env avoids prompt
/opt/feralos_webmin       # starts Webmin container (podman must be installed)
/opt/feralos_webmin        # idempotent — seeds config only once, restarts container
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
- `src/utils/firstboot/secureboot.go`, `src/utils/firstboot/tpm.go`, `src/utils/firstboot/webmin.go`
