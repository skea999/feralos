# Dinit Troubleshooting (FeralOS + dinit-chimera)

> All debug knobs below are upstream kernel cmdline parameters — no rebuilds needed.

## First-Response: Boot Debug Cmdline

Append to kernel cmdline (edit via UKI `/etc/kernel/cmdline` + regenerate, or
bootloader edit):

```
# slow-motion early boot, each early service echoes before acting
dinit_early_debug=1 dinit_early_debug_slow=3

# reduce noise, keep warnings
dinit_quiet=1 dinit_console_level=warn

# full verbosity + file log (file must be writable — use /run or tmpfs)
dinit_log_level=debug dinit_log_file=/run/dinit.log

# early-only log to file
dinit_early_debug_log=/run/dinit-early.log
```

Recovery shell if boot fails: with `dinit_auto_recovery=1` dinit offers recovery
options; without it, boot `single` via dinit's single-user service (or temporarily
revert `init=` to drop back into OpenRC — coexistence is the safety net).

## Symptom → Cause Table

| Symptom | Likely cause | Fix |
|---|---|---|
| `dinit: boot: could not find service description.` right after booster switch | services in `/usr/lib/dinit.d` — stock Alpine dinit does NOT scan it | dinit-chimera APKBUILD `prepare()` must override `srvdir` to `/lib/dinit.d`; all `*-dinit` pkgs install to `/lib/dinit.d` (pkgrel ≥ dinit-chimera 0.99.24-r2) |
| `early-tmpfiles-dev command failed with exit code 139` → whole boot cascade FAILED | `sd-tmpfiles --create` segfaults — GCC use-after-scope in `CONF_PATHS_STRV` ([upstream #5](https://github.com/chimera-linux/sd-tools/issues/5)); Alpine's r3 is GCC-built and broken | use our `sd-tools-0.99.990-r993` (static-storage patch, inflated pkgver always beats Alpine 0.99.x); never let Alpine's r3 win: `apk policy sd-tools` must show 0.99.990-r993 |
| Boots into OpenRC despite `init=` | cmdline lost (UKI not regenerated) | regenerate UKI after `/etc/kernel/cmdline` change |
| `init=` ignored in initramfs stage | expected — booster is init-agnostic, only post-switch_root | n/a |
| Hangs at `early-devd` | udevd path wrong in our hook | check `command -v udevd` / `/usr/lib/udev/udevd` in chroot; fix `dinit-devd` |
| Hangs before `early-devd` | pseudo-fs/modules — rare (booster already did it); check `dinit_early_debug` echo of last service | read last echoed service name |
| `early-tmpfiles` fails | `sd-tmpfiles` missing | `apk add sd-tools` (exit 65/73 are tolerated upstream; 127 = binary missing) |
| `early-cryptdisks` fails | bad `/etc/crypttab` or our hook arg handling | test hook manually: `dinit-cryptdisks early start` |
| No control socket / `dinitctl` can't connect | root never went rw → `early-root-rw.target` failed | check fsck output; `dinit_early_root_remount=` debug |
| Service flaps (restart loop) | daemon exits: config/permission/fs not ready | run binary by hand as its user; add missing `depends-on: local.target` |
| Two console services block each other | both `runs-on-console` | demote one to `shares-console` |
| Network up but DNS dead | chronyd/netorder fine, resolv.conf empty | installer writes resolv.conf — check chroot step, not dinit |
| Display manager never starts | seat provider missing (elogind vs seatd) or DM `depends-on` wrong | `dinitctl status elogind seatd <dm>` |

## Live Inspection (running system)

```sh
dinitctl status                  # all services + targets
dinitctl status boot system      # entry points
dinitctl status login.target network.target local.target
dinitctl start|stop <svc>        # runtime control
dinitctl enable|disable <svc>    # boot.d symlink add/remove

ls /lib/dinit.d/boot.d/          # enabled-by-package
ls /etc/dinit.d/boot.d/          # enabled-by-admin
```

Logging: dinit logs to console (+ `dinit_log_file=` if set). System logs go through
**vector** (our stack) — no journalctl on FeralOS; check `/var/log/` vector sinks.

## Package / Repo Issues

| Symptom | Fix |
|---|---|
| `UNTRUSTED signature` | `/etc/apk/keys/feralos.rsa.pub` missing or CN mismatch |
| `dinit-chimera: not found` | repo URL wrong branch dir (`v3.24` vs arch) |
| helpers crash (`kmod` symbol) | libkmod ABI drift → rebuild against Alpine's `kmod-dev` (bump `pkgrel`) |
| service file shows literal `@SCRIPT_PATH@` | meson substitution didn't run — package build bug, check `meson install` |

## Reset Path (bricked boot)

Coexistence guarantee: remove `init=` from cmdline → boots OpenRC exactly as before
(same root, same fstab, LUKS via booster unchanged). Fix dinit state from there:

```sh
apk add dinit-chimera            # re-install/repair
dinitcheck /lib/dinit.d/<svc>
vi /etc/kernel/cmdline           # re-add init= when fixed, regenerate UKI
```

Nuclear: `apk del dinit-chimera` — OpenRC remains the installed default init.
