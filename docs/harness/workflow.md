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

1. `LoadVariablesFile` parses `variables.yaml` into `RunConfig` (diskLayout, enableTPM/secureboot/hardening/zswap/nix, enableSwap/swapSize, desktopEnvironment, mirror).
2. `cache.StartHTTPMirrorServer` serves `cache/web/alpine` as cache-first Alpine mirror; `RunConfig.AlpineMirror` set to guest URL (harness rewrites `variables.yaml` Alpine mirror to this).
3. Create target disks: `qemu.CreateDisk` 16G default (`--disk-size`), 1 disk for `btrfsStandard`, 2 for `btrfsRaid0`/`btrfsStandardServer` under `runs/<timestamp>/`.
4. Start swtpm **always** (`qemu.StartSoftwareTPM` with `tpm.sock`, `tpm.sock.ctrl`, `tpm-state/`, `tpm.pid`, `tpm.log`) — even if `enableTPM=false` simplifies flow; QEMU 10 uses `--ctrl type=unixio` not `--server` (avoids `CMD_SET_DATAFD` guard).
5. Boot runner: virtio drives for runner image + install disks, `tpmQEMUArgs(tpm.sock.ctrl)` always appended, `EnableKVM` if available.
6. `guest.WaitForSSH` (runner 2222) → `createRepositoryArchive` tars `RepoRoot` excluding `tests/artifacts/`, `.git/`, `*.qcow2` → `guest.CopyToGuest` to `~/skeaAlpineInstaller.tar.gz` → `tar -xzf` → write `variables.yaml` + shims (`git`, `bash`, `fdisk` in `.skea_harness-bin` on `PATH`) → run:
   ```sh
   sudo env PATH=.skea_harness-bin:... SKEA_HARNESS_LOCAL_REPO=. SKEA_NONINTERACTIVE=1 bash ./setup.sh --test . -config ./variables.yaml
   ```
   Logs to `installer.log`. Shim notes: `git` shim and `SKEA_HARNESS_LOCAL_REPO` avoid git clone; `fdisk`/`bash` shims normalize runner env.
7. Runner `sudo poweroff` (or QMP `SystemPowerdown`); swtpm is **always** restarted after runner shutdown (QEMU `CMD_SHUTDOWN` kills it) before target boot — unconditional, not gated on `EnableTPM` (target always has TPM chardev).
8. Build target QEMU args (`buildTargetArgs`): `q35,smm=on`, OVMF `CODE`+`VARS` (`qemu.FindOVMF` → `CopyFile` to `OVMF_VARS.fd`), `virtio-blk` disks, `virtio-gpu-pci` if `desktopEnvironment==niri`, `-chardev socket,path=tser.sock,logfile=target-serial.log` for serial, QMP `target-qmp.sock`, debugcon `target-firmware.log`, always `tpmQEMUArgs`. Retry boot up to 3× on btrfs RAID race.
9. **3-boot for TPM/secureboot** (`bootAndVerifyTarget`):
   - Boot 1 — SSH as `root` via 2223, run `feralos_secureboot` (sbctl enroll — non-fatal in QEMU, no efivarfs) → log `sbctl.log`, run `feralos_tpm` (`tpm-post-install.log`), `runPostInstallDiag` → `post-install.log`.
   - Poweroff, restart swtpm, boot 2 (verification boot) — `allowPassphraseFallback` only if TPM binding failed; otherwise expects auto-unlock via Clevis token.
   - `Verify` runs; if `KeepTargetRunning` persist via `Persistent: true` and keep swtpm alive, else `poweroff`/`SystemPowerdown` and `runstate.Clear`.

## Phase 3 — `verify`

`service.Verify` (`internal/service/service.go:Verify`) — also callable standalone via `verify` command.

- SSH as `root` (or `InstalledUser`) via `guest.Options`; context timeout `VerificationTimeout` (default 2 min).
- Checks map (~30 entries): `storage_layout` (`blkid`, `/dev/mapper/luks-*`, `btrfs filesystem show/usage`), `fstab`, `fstab_vs_mount`, `kernel_cmdline` (`root=UUID` or `/dev/mapper/luks-` + `rd.luks.name`), `booster_conf`, `secureboot_conf`, `uki_artifacts` (`linux-lts.efi` + `BOOTX64.EFI`), `subvolumes`, `installed_user`, `tpm_devices`/`clevis_*`/`tpm_auto_unlock` (when TPM expected), `less_dummy` (→ `bat`), `lsattr_nocow`, `services_status`, `zswap_status`/`swap_status`, `vector_*`, `niri_*`/`greetd_*`/`wayvnc_*`/`dms_*` when niri, etc.
- Result: `VerifyResult{Success, Checks map[string]CheckResult{Pass, Detail}}` written to `verify.md` (markdown summary table) inside the run dir; failures flip `Success=false` with annotated `Detail`.
- Checks are layout-aware: `storage_layout`, `kernel_cmdline`, `secureboot_conf` gated on `EnableSecureboot`.
- `harness.log` writes a run summary at end of each run.

## References

- `tests/harness/internal/qemu/qemu.go` — `Start`, `Wait`, `Kill`, `SystemPowerdown`/`Quit`, `WriteSerial`, `QueryStatus`
- `tests/harness/internal/guest/ssh.go` — `WaitForSSH`, `CopyToGuest`, `Run`/`Output`
- `tests/harness/internal/runstate/state.go` — `ActiveRun` persisted to `state/active.json` (QMP paths, serial logs, phase)
