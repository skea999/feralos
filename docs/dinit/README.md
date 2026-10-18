# Dinit Integration — Installer Side

> **Status**: shipped — merged into `develop` (2026-10-10); Dinit is selectable via
> `enableDinit` (configuration template default), OpenRC stays co-installed as fallback.
> Packaging lives in [feralos-aports](https://github.com/skea999/feralos-aports)
> (CI live: build → sign → publish → client-verify; **rule 0** active: use Alpine
> packages whenever they exist — sd-tools dropped, Alpine community provides it).
> **Updated**: 2026-10-18

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
6. **Phase 4 (wired)**: turnstile (chimera project, ex `dinit-userservd`) →
   per-user dinit at login; DMS/pipewire/dbus/etc. are real user services.
   Requires Chimera's `turnstile.patch` on polkit (versions aligned: 127 = 127-r2).
   `niri-session` runs FIXED (shipped by `niri-user-dinit`, `replaces="niri"` —
   upstream post-25.11 dinit mechanism: full login-env setenv + dinit-monitor
   wait): the login-shell trick (passwd shell =
   `/usr/libexec/feralos/login-shell`, not in `/etc/shells`; installer re-points
   the package symlink from `/bin/ash` to fish — busybox applet dispatch breaks
   the `/bin/ash` target when invoked as `login-shell`) skips its bash
   re-exec so its dinit branch starts the `niri` user service. Packages:
   `turnstile` + `niri-user-dinit` (feralos-aports; published)

## Installer touchpoints (why this repo cares)

- `internal_rules.yaml` `services:` map → `rc-update` today; with `enableDinit`
  the service applier emits `boot.d` symlinks instead (`logic.go` / services writer)
- `variables.yaml`: `enableDinit` toggle → repo key + packages + `/etc/kernel/cmdline`
- `desktops/niri.go` DMS OpenRC service → user dinit services; session startup =
  turnstile + fixed `niri-session` (`niri-user-dinit`, `replaces="niri"`;
  login-shell trick) + user `niri`
  service (`niri-user-dinit`); installer enables `/etc/dinit.d/boot.d/turnstiled`
  and the user `boot.d` stack under `/usr/lib/dinit.d/user/boot.d/`
- Harness: `enableDinit` fixture + boot-detection via serial getty (`getty@ttyS0`)
