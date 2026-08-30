# Guides

> `docs/guides/` holds **only user guides** — no architecture/ADR/intent. Two tracks:

## User Installer Guides (for real hardware)

- [`getting_started.md`](./getting_started.md) — **Real hardware quick start**: live USB (Alpine/Void), `setup.sh` → `feralos_installer` build, wizard (`variables.yaml`), install (<10 min), post-install checks. Entry point for end users.
- [`first_boot.md`](./first_boot.md) — **First boot post-install**: run standalone `feralos_secureboot` + `feralos_tpm` from `/opt/feralos_*`, enroll sbctl, bind LUKS to TPM2, rebuild UKI. QEMU harness runs these automatically; real hardware runs manually.
- [`variables.md`](./variables.md) — **Variables compilation**: full `variables.yaml` example + option tables (disks, layout, user, hostname, mirror, desktop, video, `Enable*`, `swapSize`, `extraPackages`), compilation via `logic.ApplyRules` + `internal_rules.yaml`, fixtures.

## Desktop Application Guides

- [`webmin.md`](./webmin.md) — **Webmin web administration**: install, configure modules (firewall, packages, btrfs, smart, cron, logs), custom commands for btrbk/nix, Firefox PWA setup.
- [`podman.md`](./podman.md) — **Podman rootless containers**: install, rootless config, Docker compatibility, volume management, Webmin integration.

## Development Guides (for contributors)

- [`development.md`](./development.md) — **How to code / how things are separated**: Go conventions (snake_case files, camelCase folders), module map (`main.go` → `hostPhase`/`chrootPhase`), isolation (Entry Point, internal rule, hostPhase order), feature confinement (single-owner), logging (`shell.ExecuteCmd`), commits, build/test/harness.

See also: `docs/architecture/` (deep dives), `docs/harness/` (QEMU harness full docs), `docs/INSTALLER.md` (full flow).
