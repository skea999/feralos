# Entry Point & Phase Isolation

## Rule

`main.go` is entry point ONLY. Collects/generates config, then delegates. Must NOT call cleanup, disk layouts, or chroot commands directly.

## Delegation

```go
// src/main.go — no host deps, only config + delegation
conf, _ := config.LoadConfig(*userConfigPath)
createConfig.PromptForConfig(conf)
logic.ApplyRules(conf)

// main only delegates — no direct disk/chroot work, no apt
cleanup.Cleanup(conf)                          // BEFORE hostPhase — clean state
if err := hostPhase.Run(conf); err != nil {
    log.Fatalf("[ERROR] Host phase failed: %v", err)
}
if err := chrootPhase.Run(conf); err != nil {
    log.Fatalf("[ERROR] Chroot phase failed: %v", err)
}
firstboot.WriteInstallConfig(conf)             // AFTER chroot — reads cmdline, writes config.yaml
firstboot.SetupSecureboot(conf)                // compile → copy → chmod 755
firstboot.SetupTpm(conf)                       // compile → copy → chmod 755
cleanup.Cleanup(conf)                          // final unmount before reboot
```

```
setup.sh (before build)
  ├─ apt update + apt install golang-go + btrfs/crypt/... (host deps)

main.go (installer entry)
  ├─ Load variables.yaml + internal_rules.yaml
  ├─ Run wizard (unattended-aware)
  ├─ Apply dynamic rules (logic.ApplyRules)
  └─ Delegate:
       cleanup.Cleanup(conf)         // pre-host cleanup
       hostPhase.Run(conf)           // host-side: disks→mount→bootstrap→chroot prep
       chrootPhase.Run(conf)         // chroot-side: packages, services, boot, UKI
       firstboot.WriteInstallConfig  // reads cmdline → /opt/feralos/config.yaml
       firstboot.SetupSecureboot     // compile + copy feralos_secureboot
       firstboot.SetupTpm            // compile + copy feralos_tpm
       cleanup.Cleanup(conf)         // final unmount before reboot
```

**Isolation contract:**

- `main` collects/generates config then delegates to five entry points: `cleanup.Cleanup`, `hostPhase.Run`, `chrootPhase.Run`, `firstboot.*`, and final `cleanup.Cleanup`.
- `hostPhase.Run()` owns: disk layout → mount → Alpine bootstrap → chroot prep (steps 1–4 only).
- `chrootPhase.Run()` owns everything executed inside the target chroot (packages, services, boot, UKI).
- `firstboot.*` owns: install config write, secureboot/tpm binary compilation and copy. Runs AFTER chroot (needs cmdline from chrootPhase).
- `cleanup.Cleanup()` runs twice: once before `hostPhase` (clean state) and once at the end (final unmount).
- `hostPhase/internal/*` holds host-side helpers (bind mounts, bootstrap, chroot env). Because it is an `internal` package, only `hostPhase` (and its subpackages) may import it — `main` reaches it exclusively through `hostPhase.Run()`. This is the structural guarantee of host/chroot isolation.

## Internal Package Rule

`hostPhase/internal/*` is importable ONLY by `hostPhase` (and its subpackages). `main` cannot import it.

Go `internal` visibility enforces this at compile time — any `import "alpineInstaller/hostPhase/internal"` outside `hostPhase` fails to compile.

## hostPhase.Run() Fixed Order

1. **Disk layout switch** (`hostPhase/disks/*`) by `conf.User.DiskLayout`:
   - `btrfsStandardServer` — 2 disks, RAID1, dual EFI, LUKS swap
   - `btrfsStandard` — 1 disk, LUKS root, swapfile, 24 subvolumes
   - `btrfsRaid0` — 2 disks, RAID0, dual EFI, 2 LUKS swap p2 gated `EnableSwap`
2. **Mount virtual FS** (`internal/bind_mounts.go`) — mount /dev, /proc, /sys to chroot, slave propagation.
3. **Alpine bootstrap** (`internal/bootstrap.go`) — download apk-tools-static, init base system, configure APK repos.
4. **Chroot prep** (`internal/chroot_env.go`) — networking, localization, limits, subuid/subgid, APK repos, aliases, sudo-rs, kernel modules, ZRAM/LXC, sysctl.

Steps 0 (cleanup), 5 (install config), 6 (firstBoot compile) are now outside `hostPhase.Run()`: cleanup before/after, firstboot after chroot.

See `src/hostPhase/host_phase.go` for implementation.

## References

- `AGENTS.md#Entry Point & Phase Isolation`
- `docs/INSTALLER.md#Entry Point & Phase Isolation` and `docs/INSTALLER.md#Phase 1: Host Operations`
