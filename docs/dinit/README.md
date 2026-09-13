# Dinit Integration — Installer Side

> **Status**: planning → execution (packaging in `feralos-aports` repo)
> **Branch**: `developDinit`
> **Updated**: 2026-09-10

## What lives here vs the aports repo

This directory owns the **system + installer** side of the Dinit integration.
Packaging (APKBUILDs, CI, publishing) lives in the separate repo
`references/feralos-aports` → https://github.com/skea999/feralos-aports

| Document | Repo | Competence |
|----------|------|------------|
| `BOOT.md` | **here** | how FeralOS boots with Dinit (architecture) |
| `TROUBLESHOOTING.md` | **here** | debugging the booted system |
| `PLAN.md` | **here** | installer-side phases (3-5) + boot test fixture |
| aports `PLAN.md` | [aports](https://github.com/skea999/feralos-aports) | packaging steps (Phase 1-2), checkboxes |
| aports `STATUS.md` | aports | current step / next action (updated every step) |
| aports `PACKAGES.md` | aports | APKBUILD drafts, distro hooks, deps |
| aports `SERVICES.md` | aports | upstream 52 services + service file templates |
| aports `REPOSITORY.md` | aports | GitHub Pages, CI, client setup |
| aports `TOOLCHAIN.md` | aports | GCC/aports decision record |
| aports `CONVERSION.md` | aports | OpenRC → Dinit service files (what packages ship) |

## Core decisions (summary — full context in the docs above and aports docs)

1. **dinit from Alpine community** (0.21.0, `--shutdown-prefix=dinit-`) — zero work
2. **Boot = Chimera**: `dinit-chimera` upstream suite (52 services + compiled helpers);
   we build it via APKBUILD from upstream tarball. Updates = bump `pkgver`
3. **Our glue is minimal**: 1 mandatory hook (`dinit-devd`, eudev) + 1 optional
   (`dinit-cryptdisks`) + `getty@` service (upstream ships none)
4. **Switch via kernel cmdline** `init=/usr/libexec/dinit/init` — OpenRC coexists;
   removing `init=` boots OpenRC again (no point of no return)
5. **GCC** toolchain (Alpine default) — decision record in aports `TOOLCHAIN.md`
6. **Phase 4**: turnstile (chimera project, ex `dinit-userservd`) → `dinit --user`,
   DMS becomes a real user service; requires Chimera's `turnstile.patch` on polkit
   (versions aligned: 127 = 127-r2)

## Installer touchpoints (why this repo cares)

- `internal_rules.yaml` `services:` map → `rc-update` today; with `enableDinit`
  the service applier emits `boot.d` symlinks instead (`logic.go` / services writer)
- `variables.yaml`: `enableDinit` toggle → repo key + packages + `/etc/kernel/cmdline`
- `desktops/niri.go` DMS OpenRC service → dinit user-service variant (turnstile)
- Harness: `enableDinit` fixture + boot-detection via serial getty (`getty@ttyS0`)
