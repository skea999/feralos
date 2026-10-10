# Harness — Workflow (3-Phase Flow)

> Detail for the canonical harness. Shim: `tests/harness/README.md`. Project rules: `AGENTS.md`.

## Diagram

```
prepare-runner                    run                                          verify
─────────────                    ───                                          ──────
cloud.debian.org ─▶ base qcow2 ─▶ boot runner (Debian) ─▶ copy repo tar.gz ─▶ boot target (Alpine, OVMF+swtpm)
cache/base-images   overlay+8G    hostfwd 2222, cloud-init    tar -xzf,        disks qcow2 16G default
                    growpart/     tar/curl/parted,            setup.sh --test  3 boots: install→firstBoot→verify
                    resizefs      boot-finished, poweroff      QMP/poweroff    SSH checks → verify.md
                                 runner-qemu/serial.log       installer.log
                                                              swtpm always (tpm.sock.ctrl)
```

## Phase 1 — `prepare-runner`

`service.PrepareRunner` (`internal/service/service.go:PrepareRunner`), CLI `cmd/skea_harness/main.go:parsePrepareRunnerArgs`.

1. Download Debian cloud image — `https://cloud.debian.org/images/cloud/trixie/latest/debian-13-genericcloud-amd64.qcow2` (`defaultRunnerImageURL`) into `cache/base-images/` via `cache.EnsureDownloaded`; reused on next runs.
2. `qemu.CreateOverlay` from base → `cache/runner-images/skea-runner.qcow2`; then `qemu.ResizeDisk` to `RunnerImageSize` (default `8G`) — base is only 327 MB; without resize cloud-init `growpart` has no space.
3. Write cloud-init `user-data`/`meta-data` (`writeCloudInitFiles`): user `skea`, SSH key from `cache/keys/id_ed25519` (`guest.EnsureKeyPair`), packages `tar,curl,parted`, `growpart` + `resizefs` directives, `runcmd` that logs `boot-finished` under `/var/lib/cloud/instance/`.
4. `cloud-localds` builds `seed.iso`; QEMU boots with `-cdrom seed.iso`, `-enable-kvm`, `-netdev user,hostfwd=tcp::2222-:22`.
5. `guest.WaitForSSH` → poll `test -f /var/lib/cloud/instance/boot-finished` → `sudo poweroff` (fallback `qemu.SystemPowerdown` via QMP).
6. Keeps last 1 `prepare/<timestamp>/` dir (`cleanOldRuns`); writes `serial.log`, `qemu.log`, `seed.iso`, `user-data`.

## Phase 2 — `run`

`service.RunInstall` (`internal/service/service.go:RunInstall`), CLI `parseRunArgs` requires `-f <variables.yaml>`.

Phase announcements (`[harness] phase N/9: …`) print to stdout and are written to `harness.log`: 1 boot runner, 2 run installer, 3 reboot (runner), 4 boot target, 5 run sbctl, 6 run tpm, 7 reboot (target), 8 boot target (verification), 9 verify target. Target poweroff is init-aware: OpenRC → `poweroff`, dinit → `dinit-poweroff` (`dinit-shutdown` package).

1. `LoadVariablesFile` parses `variables.yaml` into `RunConfig` (diskLayout, enableTPM/secureboot/hardening/zswap/nix, enableSwap/swapSize, desktopEnvironment, mirror). All fixtures set `enableDebug: true` — **required**: `enableDebug` appends `console=ttyS0,115200` to the kernel cmdline, which the harness uses to watch boot output and answer the LUKS passphrase prompt over serial (`waitForTargetSSH`/`sendBootPassphrase`). Real-hardware installs keep `enableDebug: false` (hardened cmdline, no serial console).
2. `cache.StartHTTPMirrorServer` serves `cache/web/alpine` as cache-first Alpine mirror; `RunConfig.AlpineMirror` set to guest URL (harness rewrites `variables.yaml` Alpine mirror to this).
3. Create target disks: `qemu.CreateDisk` 16G default (`--disk-size`), 1 disk for `btrfsStandard`, 2 for `btrfsRaid0`/`btrfsStandardServer` under `runs/<timestamp>/`.
4. Start swtpm **always** (`qemu.StartSoftwareTPM` with `tpm.sock`, `tpm.sock.ctrl`, `tpm-state/`, `tpm.pid`, `tpm.log`) — even if `enableTPM=false` simplifies flow; QEMU 10 uses `--ctrl type=unixio` not `--server` (avoids `CMD_SET_DATAFD` guard).
5. Boot runner: virtio drives for runner image + install disks, `tpmQEMUArgs(tpm.sock.ctrl)` always appended, `EnableKVM` if available.
6. `guest.WaitForSSH` (runner 2222) → `createRepositoryArchive` tars `RepoRoot` excluding `tests/artifacts/`, `.git/`, `*.qcow2` → `guest.CopyToGuest` to `~/feralos.tar.gz` → `tar -xzf` → write `variables.yaml` + shims (`git`, `bash`, `fdisk` in `.skea_harness-bin` on `PATH`) → run:
   ```sh
   sudo env PATH=.skea_harness-bin:... SKEA_HARNESS_LOCAL_REPO=. SKEA_NONINTERACTIVE=1 bash ./setup.sh --test . -config ./variables.yaml
   ```
   Logs to `installer.log`. Shim notes: `git` shim and `SKEA_HARNESS_LOCAL_REPO` avoid git clone; `fdisk`/`bash` shims normalize runner env.
7. Runner `sudo poweroff` (or QMP `SystemPowerdown`); swtpm is **always** restarted after runner shutdown (QEMU `CMD_SHUTDOWN` kills it) before target boot — unconditional, not gated on `EnableTPM` (target always has TPM chardev).
8. Build target QEMU args (`buildTargetArgs`): `q35,smm=on`, OVMF `CODE`+`VARS` (`qemu.FindOVMF` → `CopyFile` to `OVMF_VARS.fd`), `virtio-blk` disks, `virtio-gpu-pci,virgl=on` if `desktopEnvironment==niri` (3D accel for niri — niri/wlroots GL needs it; without it only the pixman fallback works; **host needs GL/EGL** — if the host lacks it, QEMU fails at startup, fail loud intended), `-chardev socket,path=tser.sock,logfile=target-serial.log` for serial, QMP `target-qmp.sock`, debugcon `target-firmware.log`, always `tpmQEMUArgs`. Retry boot up to 3× on btrfs RAID race.
9. **3-boot for TPM/secureboot** (`bootAndVerifyTarget`):
   - Boot 1 — SSH as `root` via 2223, run `feralos_secureboot` (sbctl enroll — non-fatal in QEMU, no efivarfs) → log `sbctl.log`, run `feralos_tpm` (`tpm-post-install.log`), `runPostInstallDiag` → `post-install.log`.
   - TEST-ONLY niri autologin — `enableNiriTestAutologin` appends a greetd `[initial_session]` (`command = "niri-session"`, `user = <InstalledUser>`) to `/etc/greetd/config.toml` (idempotent) so the verification boot starts the real user session. The autologin is a SIMULATION of the real greetd/regreet login: the command stays `niri-session` — the exact same `/usr/bin/niri-session` script a real login runs (the fixed one shipped by `niri-user-dinit`, `replaces="niri"`). Plain `[initial_session] command = "niri-session"` ONLY — no wrapper script, no dinitctl polling. VM disk discarded after run; the installer NEVER writes `[initial_session]` — real installs log in manually through regreet → `niri.desktop` (`Exec=niri-session`).
     **POLICY — "The installer is the product; the harness simulates real life."** The installer must produce a system that works on real hardware with zero test-only crutches; the harness verifies that behavior by simulating real usage in a VM. Harness scope = tests + autologin simulation ONLY. If the niri session does not start/hold under the stock niri-session (async user-dinit spawn after PAM open; niri-session's dinitctl start returns early → greetd tears the session down), the FIX belongs in the installer or feralos-aports — never in the harness (this exact fix shipped in `niri-user-dinit`). Discovered empirically in run 2026-10-03T00-39-01Z (74/80: all 6 runtime checks failed, session never came up).
   - Poweroff, restart swtpm, boot 2 (verification boot) — `allowPassphraseFallback` only if TPM binding failed; otherwise expects auto-unlock via Clevis token.
   - `Verify` runs; if `KeepTargetRunning` persist via `Persistent: true` and keep swtpm alive, else `poweroff`/`SystemPowerdown` and `runstate.Clear`.

## Phase 3 — `verify`

`service.Verify` (`internal/service/service.go:Verify`) — also callable standalone via `verify` command.

- SSH as `root` (or `InstalledUser`) via `guest.Options`; context timeout `VerificationTimeout` (default 2 min; niri runs extend to 10 min — the 6 runtime checks poll up to 90s each).
- Checks map (~30 entries): `storage_layout` (`blkid`, `/dev/mapper/luks-*`, `btrfs filesystem show/usage`), `fstab`, `fstab_vs_mount`, `kernel_cmdline` (`root=UUID` or `/dev/mapper/luks-` + `rd.luks.name`), `booster_conf`, `secureboot_conf`, `uki_artifacts` (`linux-lts.efi` + `BOOTX64.EFI`), `subvolumes`, `installed_user`, `tpm_devices`/`clevis_*`/`tpm_auto_unlock` (when TPM expected), `less_dummy` (→ `bat`), `lsattr_nocow`, `services_status`, `zswap_status`/`swap_status`, `vector_*`, `niri_*`/`greetd_*`/`wayvnc_*`/`dms_*` when niri, etc.
- niri runtime checks (verification boot only, via the test-only greetd autologin): `niri_session_user` (`pgrep -u <user> -x niri`), `niri_wayland_socket` (`/run/user/$uid/wayland-1`), `niri_ipc` (`niri msg outputs` with `NIRI_SOCKET`/`XDG_RUNTIME_DIR`/`WAYLAND_DISPLAY`), `niri_user_dinit_runtime` (dinit only: all user boot.d services show the started marker in `dinitctl --user list`), `niri_wayvnc_port` (`netstat :5900`), `niri_screenshot` (`niri msg action screenshot-screen` → non-empty PNG). All poll ≤90s; all falsifiable (exit 1 on failure). Empirically verified on niri 25.11 (runs 2026-10-03T18-56-40Z + 2026-10-03T19-27-59Z + debug boots): `dinitctl list` has TWO started-marker formats — `[{+}     ] <name> (pid: N)` for DEPENDENCY-started services and `[[+]     ] <name> (pid: N)` for EXPLICITLY-started ones; niri is started via `dinitctl --user start niri` so it renders `[[+]] niri` (pattern `(\\{\\+|\\[\\+).*\\] <name>( |$)` matches both — the old `\\{\\+`-only pattern was a false negative that made 3 verify boots look like "race lost"). `niri msg action screenshot-screen` takes NO positional path arg (rejects: "unexpected argument"; use `--path <abs>`), defaults `--write-to-disk true`, saves per `screenshot-path` (default `~/Pictures/Screenshots`). bug-3 RESOLVED (2026-10-09): host QEMU ≥11.1 + `virtio-vga-gl` (venus+virgl) + `-display egl-headless,gl=on` + guest mesa from edge (26.2.4 — virgl gallium driver restored upstream) + host kernel ≥6.13 (venus-gpu) ⇒ niri gets a REAL output (`Virtual-1` 1280x800), wayvnc listens on :5900, screenshot writes a real PNG. The former QEMU DMI skip-gates on `niri_outputs_count`/`niri_wayvnc_port`/`niri_screenshot` (and the wayvnc.user required-list gate) are REMOVED — all checks real on every host (verified run 2026-10-09T18-21-04Z, 86/86, zero skips). Screenshot check mechanics: count-based fresh-PNG compare (before/after `ls *.png | wc -l` + 1s settle) — the newest-file compare loses the race (action returns rc 0 BEFORE the file lands). The autologin setup creates BOTH `~/Pictures` and `~/Pictures/Screenshots` with `install -d -o <user> -g <user> -m 2700` — a root `mkdir -p` leaves the parent root-owned (setgid home + root umask) and the user then cannot write screenshots at all.
- Real hardware: `tests/harness/niri_realhw_check.sh` (POSIX sh, busybox-clean) — same session checks + pre-session system checks (DRM card0/renderD128 + driver, greetd/elogind/turnstiled) + wayland logind session type. Run as root on the target after login: `sh niri_realhw_check.sh [username]`.
- Result: `VerifyResult{Success, Checks map[string]CheckResult{Pass, Detail}}` written to `verify.md` (markdown summary table) inside the run dir; failures flip `Success=false` with annotated `Detail`.
- Checks are layout-aware: `storage_layout`, `kernel_cmdline`, `secureboot_conf` gated on `EnableSecureboot`.
- `harness.log` writes a run summary at end of each run.

## References

- `tests/harness/internal/qemu/qemu.go` — `Start`, `Wait`, `Kill`, `SystemPowerdown`/`Quit`, `WriteSerial`, `QueryStatus`
- `tests/harness/internal/guest/ssh.go` — `WaitForSSH`, `CopyToGuest`, `Run`/`Output`
- `tests/harness/internal/runstate/state.go` — `ActiveRun` persisted to `state/active.json` (QMP paths, serial logs, phase)
