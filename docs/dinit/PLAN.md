# Dinit Plan — Installer Side

> Packaging phases (1-2) live in `references/feralos-aports/PLAN.md` (aports repo).
> This file owns what touches the **installer codebase and the installed system**.
> Updated: 2026-10-18

## Phase 2 (shared) — Boot Test

Installer-side deliverable: fixture + harness green under Dinit.

1. Test fixture `tests/fixtures/variables/btrfs_standard_dinit.yaml`
   (`enableDinit: true` once the toggle exists; interim: manual chroot steps)
2. Harness detects boot via serial getty → requires aports `getty-dinit` with
   `ttyS0` instance (see aports PLAN step 4)
3. `log_errors` allowlist: add dinit noise if any appears
4. New harness check (optional): `login.target` reached, parsed from serial log
5. Debug knobs if boot fails (documented in `BOOT.md`, no rebuilds):
   `dinit_early_debug=1 dinit_early_debug_slow=3 dinit_quiet=1 dinit_console_level=warn`

Risk areas pre-identified: `udevd` path in eudev hook (aports-side, one line);
`getty@` missing (aports side); everything else is upstream.

## Phase 3 — System services wiring

Packages are built in aports (`sshd-dinit`, `dbus-dinit`, … — see aports PLAN
step 6). Installer-side impact is small:

- `internal_rules.yaml` keeps `services: {name: runlevel}` as the single source;
  the services applier learns the dinit mode (below)
- Services that ship **only** OpenRC scripts get `-openrc` subpackages dropped when
  `enableDinit` (e.g. `chrony-openrc` → `chrony`)

## Phase 4 — Desktop services wiring

- `elogind-dinit` / `seatd-dinit` (mutually exclusive, same policy as today)
- `greetd-dinit` / `cosmic-greeter-dinit` / `sddm-dinit` + `turnstile` + patched
  `polkit` (packages from aports)
- PAM setup (installer writes files in chroot):
  - turnstiled stack: `session optional pam_elogind.so`
  - display manager stack: `session optional pam_turnstile.so`
- `desktops/niri.go`: DMS dinit **user** service replaces the OpenRC async hack
  (only Go-code change of the whole integration)
- PAM files must keep exact names expected by upstream polkit patch (`turnstiled`)

## Phase 5 — `enableDinit` end-to-end

1. `variables.yaml`: `enableDinit` toggle (template default: `true`; OpenRC co-installed either way)
2. When true, chrootPhase:
   - copies `feralos.pub` → `/etc/apk/keys/feralos.rsa.pub`
   - adds `https://<pages-url>/v3.24/main` to `/etc/apk/repositories`
   - installs `dinit`, `dinit-chimera`, `sd-tools`, `getty-dinit` + service set
   - appends `init=/usr/libexec/dinit/init` to `/etc/kernel/cmdline` (UKI regen)
3. Services applier mode switch (the one code change in `logic.go`/services writer):
   - OpenRC mode (today): `rc-update add <svc> <runlevel>`
   - Dinit mode: symlink `/lib/dinit.d/boot.d/<svc>`;
     `boot`/`sysinit`-level entries are **dropped** (upstream early chain replaces)
4. Harness: fixture `btrfs_standard_dinit.yaml` + full variants; matrix after green
5. OpenRC stays installed (coexistence). Removing it: separate future decision.
   Default switch to Dinit: only after all 4 layouts × 3 DEs green.

## Definition of done (installer side)

- [x] `enableDinit: true` fixture installs and boots via Dinit in harness
- [x] All standard services verified running (`dinitctl status` / serial log)
- [x] Desktop matrix (niri/niriWayle/kde/cosmic) boots to graphical login under Dinit
- [x] Docs updated (this dir + AGENTS.md history + aports STATUS.md cross-ref)
