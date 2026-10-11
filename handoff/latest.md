# Handoff

**Active state**: `state/session_state.json`
**Milestone log**: `CHANGELOG.md`

## Project Vision

**FeralOS** is a new Linux distribution based on Alpine Linux, focused on **memory safety** and **security**. It replaces traditional C components with modern alternatives (uutils, micro, vector). For software not in Alpine repos or requiring glibc, **Nix** is used. **Dinit** replaces OpenRC (complete, 47/47 verify). A **custom Alpine-style repository** for FeralOS packages is live on GitHub Pages.

## Current Focus

**pkexec vs login-shell fixed (2026-10-11)**: pkexec validates the session user's shell vs /etc/shells → FeralOS login-shell path unlisted = RC=127 on every privileged DMS op (Create User + all polkit). Diagnosed via debug_run (repro: pkexec RC=127, root useradd RC=0). Fix: installer lists login-shell in /etc/shells (safe — our niri-session has no re-exec trap, grep-verified); harness login_shell_trick now asserts it.

**DMS green theme seeded by installer (2026-10-11)**: niri.go writes `~/.config/DankMaterialShell/settings.json` (`currentThemeCategory: generic`, `currentThemeName: green`) after the `dms setup` fragments + per-file chown. Generic themes = static Material palettes (not matugen); user switching to Auto/wallpaper later overrides normally. Keys verified from DMS source (SettingsSpec.js / ThemeColorsTab.qml). Dark mode too: session.json (XDG state dir, ~/.local/state/DankMaterialShell/) seeded {"isLightMode": false} — default is already dark per DMS source, seed = portal-independent guarantee. Bar at bottom too: seed carries configVersion:1 + legacy dankBarAtBottom:true — DMS's own v1→v2 migration builds the full default barConfigs with position=1 (raw assignment means a partial seed would break the bar; migration = no default duplication). Bar left widgets pinned to launcher/workspaces/running-apps (focusedWindow swapped out) via the same legacy migration keys. Default apps seeded: xdg-terminals.list (Alacritty) + mimeapps.list (firefox/eog/cosmic-player) — verified desktop IDs from Alpine contents. Maps/calendar defaults: geo=org.gnome.Maps.desktop, calendar/webcal=org.gnome.Calendar.desktop (+ packages in niri list). Icon themes: adwaita+hicolor = dms-shell deps (aports 14f124b); GTK settings.ini seeded (Adwaita + prefer-dark). Harness `dms_icons` check added (packages + Adwaita on disk + GTK settings assertion).

**DMS on greeter fixed (2026-10-11)**: aports `dms-shell` `0004567` — dms-session greeter guard (username greeter / XDG_SESSION_CLASS) + dms.user restart=true removed (clean exit stops, smooth-recovery keeps crash recovery). Installer boot.d links are system-wide so greeter's user dinit started the full stack; gate opened because greeter's niri creates the socket. CI green; manual boot = final proof.

**Branch merge chain (2026-10-10)**: developDinit → develop fast-forward; develop → main merge commit `8568df0` (9 conflicts, develop side won — cancels the 2 old dinit reverts on main + net-zero resolv pair; keeps `btrfs_standard_dinit.yaml`). main = develop content now: bootstrap fix, feralos renames, full dinit integration. Pending cherry-pick item RESOLVED by the merge. All branches pushed (SHA match). **Work continues on `develop`** — `developDinit` == develop (same SHA), main = 1 merge commit ahead of develop.

**Repo hosting: Gitea → GitHub (2026-10-10)**: repository migrated to `github.com/skea999/feralos` (GitHub importer). Refs parity verified BEFORE switching: all branches identical SHAs except `developDinit` = local ahead 2 → fast-forward push; zero tags on both sides. `origin` = github, old remote kept as `gitea` temporarily (remove + archive gitea repo after confidence period); `gh auth setup-git` handles push auth. Hardcoded URLs fixed: `setup.sh` repoUrl + README wget — github raw format `raw.githubusercontent.com/<owner>/<repo>/main/` (NOT gitea's `raw/branch/main/`). No CI configs existed (`.github/` empty of tracked content — nothing to port). FOLLOW-UP same day: in-repo identifiers renamed `skeaAlpineInstaller` → `feralos` (Go modules `feralos/tests/harness` + `feralos/tests/debug_run`, imports, runner-VM guest path `/home/<user>/feralos`, docs; `src` module stays `alpineInstaller`; historical gitea-name records kept); getting_started.md curl/clone URLs fixed (wrong owner `skem` + nonexistent `latest-stable` path). Host folder also renamed → `~/Progetti/feralos` (git fully path-independent). FOLLOW-UP 2: src Go module `alpineInstaller` → `feralos/src` (module + ~50 files' imports + docs; branch name `alpineInstallerGolang` kept — it's a git branch; 5 binaries + vet/gofmt/lint/test green, `5d3a7a8`). FOLLOW-UP 3 (post-rename validation): tests green, harness dinit/niri run `2026-10-10T22-11-11Z` **94/94** (infra: runner qcow2 rebase to new path, prepare-runner -force 8G, cloud-image-utils reinstalled); debug_run live boot 8/8; debug_run stale `venus/blob/hostmem` QEMU flags fixed `dccb79a` (apt QEMU 8.2.2 rejects venus — validated native boot).

**DMS wifi/BT harness checks — 2 real bugs found+fixed, 94/94 (2026-10-10)**: new niri checks `bluetooth_service` (started marker + bluetoothd + org.bluez on the system bus — the DMS BT widget endpoint; init-conditional) and `nm_wifi_support` (networkmanager-wifi + wireless-regdb + wpa_supplicant + regulatory.db + `nmcli radio wifi`; init-agnostic). First run caught TWO real bugs: aports bluez-dinit ran bluetoothd from the Debian path (`/usr/libexec/`) — Alpine = `/usr/lib/bluetooth/` (fixed `fb64d77` r3, Pages live); harness tested `/usr/sbin/wpa_supplicant` — Alpine = `/sbin/` (check fixed). Re-run `2026-10-10T12-12-01Z`: **94/94 PASS** — `bluetooth (pid: 1562)` + org.bluez on bus, `radio: enabled`, zero failures, all sentinels green. QEMU has no BT/wifi radios — scan/connect/adapter = real-hw validation.

**Bluetooth unified + niri-only (2026-10-10)**: user review corrected the previous wiring — aports `e9ac00c` renames the feralos `bluez-dinit` service file to `/lib/dinit.d/bluetooth` (same name as the OpenRC initd — we own the package, r2 on Pages, tar-verified); `defaultRunlevelsDinit.boot` and `dinitSkipServices` back to original. Enablement = single `"bluetooth": "default"` in the NIRI desktop services map only (dinit boot.d link + OpenRC rc-update; server/minimal/kde/cosmic never enable it). DMS BT widget (org.bluez dbus) + NM wifi completed earlier same day: `networkmanager-wifi` plugin subpackage (Alpine split — without it NM had NO wifi support) + `wireless-regdb` (kernel regulatory db for real radios). Packages stay niri-scoped.

**DMS bluetooth + NM wifi completeness (2026-10-10, data-only)**: both diagnosed via `dms doctor` on the live target — **bluetooth**: bluez was in no package list (feralos `bluez-dinit` with dbus-wait-for existed since ADR-009 but never wired); niri packages += `bluez`/`bluez-openrc`/`bluez-dinit`; **name split**: OpenRC = `bluetooth`, bluez-dinit = `bluez` → services map enables `bluetooth` (OpenRC), `defaultRunlevelsDinit.boot += bluez`, `dinitSkipServices += bluetooth`. **wifi**: `networkmanager-wifi` plugin subpackage (Alpine split) was missing → NM had NO wifi support; niri packages += `networkmanager-wifi` + `wireless-regdb`; wpa_supplicant already present (NM supervises). DMS doctor inventory: dgop NOT a gap (embedded in dms binary), matugen/cava = user-handled, kimageformats/khal/adw-gtk3 user added. Zero Go changes. (Name split SUPERSEDED by the unified rename above.)

**niri/DMS dinit runtime depth — 5 checks added/hardened, 91/91 (2026-10-09)**: deeper "does it actually DISPLAY and WORK" verification for the dinit niri stack (fixture `btrfs_standard_niri_full_test.yaml`, `enableDinit: true`). NEW `niri_layers` (dinit): `niri msg layers` must list ≥1 `^Output` = the DMS bar RENDERS (layer-shell mapped; live shape `Output "Virtual-1": Background layer: Namespace: "quickshell"`) — deeper than process-alive/IPC. NEW `greetd_socket` (both inits): greetd IPC socket — GOTCHA: greetd socket is PID-suffixed `/run/greetd-<pid>.sock` (clients get path via GREETD_SOCKET env), NOT `/run/greetd.sock` → glob `/run/greetd-*.sock`. NEW `session_dbus_socket` + `session_pipewire_socket` (dinit): `/run/user/$uid/bus` + `pipewire-0` — "started" ≠ endpoints exist. `niri_screenshot` hardened: PNG magic bytes asserted (real 220KB capture validated). regreet runtime NOT checkable on verify boots (autologin starts the session directly). Validated: run `2026-10-09T22-30-58Z` 91/92 → 91/91 after greetd glob fix; windows shows DMS window `com.danklinux.dms` "Benvenuto". OpenRC niri fixture: `dms_ipc` + `niri_wayvnc_port` remain dinit-only FAILs by user decision.

**less shim — aports less-dummy package (2026-10-09)**: runtime mkpkg symlink `less→bat` CONFIRMED BROKEN on live target (debug_run): bat with TTY stdout execs its hardcoded pager `less -R -F -K --no-init` (ignores `$PAGER`, only `cat` short-circuits) → symlink made bat re-exec itself → clap `unexpected argument '-R'` — fish `history`, `git log`, `man` all broke; ash "worked" only because busybox ash has no pager-piping history builtin. Fix: aports **`less-dummy` 999.0.0-r2** (`f8dcee5`, CI green after ECR container-mirror fix, on Pages) — `/usr/bin/less` = POSIX-sh wrapper that drops less flags (file operands + post-`--` kept) and dumps via `bat --color=always --paging=never` (child never re-pages → terminates). `provides=less=999` + `replaces=less` (real C less blocked by file conflict). Installer: `less-dummy` in standardPackages, `dummyPackages: {}`. Harness: `less_dummy` falsifiable ×4 assertions (pkg + not-symlink + bat's EXACT flag set `-R -F -K --no-init` + stdin/file/`--` content passthrough) + NEW `less_fish_history` (no clap text in fish history output) — **both VALIDATED GREEN live** (run `2026-10-09T22-03-03Z`, fixture `btrfs_standard_niri`, 68/71; 3 unrelated FAILs = pre-existing dinit-vs-fixture gating gap on `console_keymap`/`dms_ipc`/`niri_wayvnc_port`, fixture has `enableDinit: false`). CI infra: `build.yml` container → `public.ecr.aws` mirror (Docker Hub rate limit). **Next**: `console_keymap` FIXED init-conditional (OpenRC rc-status marker, regex validated vs real output); `dms_ipc`/`niri_wayvnc_port` still dinit-only (user-dinit services don't exist under OpenRC — gate on cfg.EnableDinit or flip fixture); optional `BAT_PAGER=cat`; future moar/ov pager via same package.

**bug-3 DEAD — niri renders in QEMU + edge repos as confined feature (2026-10-09)**: host Nix QEMU 11.1.1 full build via dynamic `/usr/local/bin` wrappers (qemu-system-x86_64 + qemu-img exec the nix profile binaries — no dead copies, libvirt/virt-manager compatible; libvirt AppArmor local override for `/home/skem`) + `-device virtio-vga-gl,venus=on,blob=true,hostmem=1G` + `-display egl-headless,gl=on` + host kernel 7.0 HWE (venus-gpu) + guest mesa 26.2.4 `@edge` (virgl gallium driver restored upstream) ⇒ niri has a REAL output (`Virtual-1` 1280x800), wayvnc listens :5900, screenshot writes a real PNG. Run `2026-10-09T18-21-04Z`: **86/86 PASS, zero skips** (first fully-real niri run in QEMU). Installer: `enableEdgeRepos` + `extraPackagesEdge` confined in `chrootPhase/features/edgerepos.go` — 3-step at the tail of SetupPackages (append @edge repos → `apk add` → comment the repos out; apk v3 tagged-repo masking kills mid-install edge mixing — "masked in @edge" cascade verified). Console keymap: aports `busybox-loadkmap-dinit` r1 + `misc.go` KEYMAP= from user variables + internal_rules boot-list. setup.sh: config path = repo `src/variables/variables.yaml` + cdrom apt-source fix. Harness: QEMU DMI skip-gates REMOVED (all checks real on every host), `console_keymap` regex fixed for the `[{+}]` marker, screenshot check count-based fresh-PNG. Fixtures 28/28 = full 33-key template mirror. **Next**: user real-hw validation (keymap + real GPU), dms windows depth, booster initramfs module policy parked (host-specific default + user force_load).

**dinit services aligned to Artix/Chimera (2026-10-02, ADR-009)**: exhaustive audit (Artix gitea 7370 repos + 18768-pkg binary repo, Chimera cports + dinit-chimera) → 48 `-dinit` pkgs aligned (aports `b69a573`, CI green first try, 103 files +861/−163). `restart=true` removed from ALL daemons (smooth-recovery only; getty keeps `restart=true` + `termsignal=HUP` + config-driven baud/term via `/usr/lib/dinit/getty` + `/etc/dinit.d/config/getty.conf`, Artix agetty-default pattern, 115200 serial); scripted one-shots explicit `restart=false`; `before=login.target` ×33; network ordering (firewalld/nftables/iptables before `pre-network.target`, NM depends pre-network + before `network.target`, avahi/rsync depend networking, apparmor before `local.target`, fuse after `early-root-rw.target`); `dnsmasq` privilege drop `-u dnsmasq -g dnsmasq --enable-dbus`; `dbus` `smooth-recovery=no` (ADR-007 stop guard kept, clean shutdown verified); bluez/elogind bus readiness via NEW `dbus-wait-for-dinit` pkg (BSD-2-Clause, from dinit-rc, pipefd:4); `lxc` process+restart bug → scripted start/stop (restart-loop fixed); `sshd` `ssh-keygen` prereq. Audit facts: Artix dbus-activates polkit/udisks2/fwupd; irqbalance/rasdaemon/incus/nix/vector/klogd/cosmic-greeter not packaged there; ADR-008 wrapper+config retained as FeralOS extension. Verified: dinit niri **74/74** (run 2026-10-02T00-15-22Z — getty serial alive, dbus clean shutdown, elogind bus readiness, NM ordering), OpenRC minimal **49/49** (run 2026-10-02T00-27-53Z, +3 checks = harness growth). **Next**: earlyoom `--avoid` protection, real-hardware NVMe test, Pages race hardening (structural), xdg-portals as user services someday (Chimera model).

**conf.d-equivalent config files (2026-10-01, Artix pattern)**: 22 `-dinit` packages converted — service file → `/usr/lib/dinit/<svc>` wrapper → `[ -r ] && . /etc/dinit.d/config/<svc>.conf` → exec daemon `${VARS}`. OpenRC `/etc/conf.d` convention restored: user-editable, apk-protected (644, edits survive upgrades, new version = `.apk-new`), defaults match Alpine conf.d. `dnsmasq` `DNSMASQ_CFGFILE` defaults `/etc/dnsmasq.conf` (empty = no config); `cpufrequtils` gained a `stop-command` consuming `CPUFREQ_STOP_OPTS`. aports `cc107a1` pushed, CI green (client-test Pages propagation race = transient, re-run passed). Harness: permanent `dinit_config` check (`tests/harness/internal/service/service.go`, EnableDinit only: config dir + ≥10 `.conf` + wiring spot-checks). Verified: dinit niri full **74/74** (run 2026-10-01T22-19-23Z), OpenRC minimal **46/46** (run 2026-10-01T22-27-05Z, check correctly absent). ADR-008.

**klogd-openrc package (2026-10-01)**: l'OpenRC klogd ora arriva dal repo — **klogd-openrc 989.0.1.0** (feralos-aports `2d5e05f`, CI green; `replaces="busybox-openrc"` — senza, apk rifiuta l'overwrite del file, scoperto dal rehearsal). Installer: rimossa la scrittura runtime (`logging.go`), il path OpenRC installa il package (`packages.go`); abilitazione invariata (boot runlevel). Harness: `klogd_service` verifica anche il marker del package su OpenRC. Verificato: OpenRC minimal **48/48** (run 18-14-18Z, `Installing klogd-openrc (989.0.1.0-r0)`) + dinit niri **72/72** regression (18-17-10Z). **earlyoom**: confermato già configurato (default `-m 10 -s 10`, `-r 60`; `earlyoom-dinit` enabled/running).

**Disk maintenance + OpenRC klogd (2026-10-01)**: schedule disco via cron — **scrub settimanale** (Dom 03:30), **TRIM settimanale** (Dom 03:00, script con passthrough dm-crypt temporaneo via `cryptsetup refresh --allow-discards`, nessuna modifica a luksFormat/booster), **btrbk giornaliero** 02:00 (retention 7/4/3 già in btrbk.conf); path risolti all'install; log dedicati ruotati 7g. **OpenRC klogd risolto**: override `/etc/init.d/klogd` scritto dall'installer (il vendor richiede `logger` sostituito da vector) → kern.log su entrambe le init. Harness: check `disk_schedule` + validazioni klogd/coverage ora incondizionate. Verificato: dinit niri **72/72** + OpenRC minimal **48/48** (run 2026-10-01T17-44-16Z / 17-48-47Z).

**Logging Round 2 (2026-09-30)**: sink split (`sudo.log`, `firewalld.log`), vista JSON `/var/log/feralos.json.log`, `internal_logs` nel flusso centrale, API Vector locale **solo con `enableDebug`** (127.0.0.1:8686), sorgente incus **condizionale** (solo se pacchetto installato; webmin saltato — log in tmpfs), retention logrotate 30/14/7. 3 check nuovi (`json_log`, `vector_api`, `vector_validate`). Harness **70/70** (run 2026-09-30T23-09-27Z, json 523 righe). Config pre-validata in docker (2 fix VRL 0.55 trovati prima del run).

**Logging Round 1 (2026-09-30)**: kernel ring finalmente nei file — nuovo `klogd-dinit` (busybox klogd → /dev/log → vector `kern.log` + snapshot `/var/log/dmesg`), dinit-only (l'OpenRC klogd richiede `logger`, sostituito da vector). Vector: rimossi i duplicati da rotazione (`*.log.1` + copytruncate) e disk buffer sui 4 sink (min 256 MiB). Harness: 4 check nuovi — **67/67** (run 2026-09-30T22-52-14Z, kern.log 56 righe, 0 bus error). feralos-aports `f2842dc` (CI green).

**Artix-parity audit (2026-09-30, round 2)**: 49 servizi dinit confrontati con Artix — 18 pacchetti aggiornati in **un solo push** (feralos-aports `90197ed`, CI green): seatd readiness (`-g seat -n 3` + pipefd:3), nftables stop flush, iptables `-w 3`, sddm+elogind, cosmic-greeter+seatd, avahi/cupsd/dnsmasq+dbus, turnstiled log in `/var/log/dinit/`, `smooth-recovery` dove Artix lo ha; rimossi `pipewire-dinit`/`wireplumber-dinit` (mai installati). Harness niri **63/63** (run 2026-09-30T22-04-51Z).

**dinit shutdown ordering (2026-09-30)**: dependency stop race fixed at the source — dinit releases a dependency when dependents *begin* stopping, so at shutdown `dbus`/`elogind` could die while `turnstiled` was still closing PAM sessions ("Failed to connect to system bus"). feralos-aports `dbus-dinit` r6 + `elogind-dinit` r5 add `stop-command` guards (wait bounded for `turnstiled` via `pidof`, then stop the daemon; `stop-timeout=30` failsafe; no-op without turnstile) — commit `75c566a`, ADR-007. Harness `log_errors` allowlist removed (no masking); niri+dinit **63/63** (run 2026-09-30T00-08-04Z, 0 bus errors, `turnstiled` stops before `dbus`/`elogind` in both shutdowns).

**Dinit — COMPLETE** (47/47 verify, 2026-09-20). Modello Alpine/Artix: pacchetti `-dinit` ship-only, enablement via regole. Servizi: chronyd, sshd, incusd, smartd, nix-daemon, mdev, incus-agent + networking-dinit (ifup -a). Log: `log-type=file` → `/var/log/dinit/<nome>.log`, trail PID1 → `/run/dinit-boot.log` (wrapper legge da `/proc/cmdline`) → `/var/log/dinit/boot.log` (tail -F), vector ingests, logrotate. aports Step 7 COMPLETE. Harness: verify 47/47 su btrfs_standard_dinit.

**Desktops dinit**: KDE (`sddm-dinit`), Cosmic (`cosmic-greeter-dinit` + `seatd-dinit`), Niri (`greetd-dinit` + `elogind-dinit` + `polkit-elogind-dinit`) — tutti testati con harness.

**Dinit shutdown + harness phases (2026-09-25)**: installer ships `dinit-shutdown` + wires conventional commands — `/usr/bin/{poweroff,halt,shutdown}` symlinks + `/usr/bin/{reboot,soft-reboot}` wrappers (busybox `poweroff` sends SIGUSR2 — a no-op under dinit; `dinitctl shutdown` only HALTs). Harness: numbered run phases (`[harness] phase N/9: …` → stdout + `harness.log`) and init-aware `powerOffTarget()` (`enableDinit` → `/usr/sbin/dinit-poweroff`; OpenRC → `poweroff`) with 5-min fail-not-hang bound; new verify check `dinit_shutdown_commands`. Verified: dinit **48/48** (2m41s), OpenRC **39/39** (2m32s), live reboot/poweroff test. Milestone log moved from AGENTS.md bottom to `CHANGELOG.md` (`## Milestone Log`).

**Harness check truthfulness (2026-09-25)**: dinit verify checks made falsifiable — 8 of 9 `dinit_*` checks ended in `&& echo ok || echo fail` (the fallback `echo` always exits 0 → PASS forever), with 3 broken commands: busybox `ps` has no `-p` (→ `/proc/1/comm`), bare `dinitctl status` requires `<service-name>` (→ `dinitctl list`), `dinit_feralos_repo` grepped `/etc/apk/repositories` (keyring installs `/etc/apk/repositories.d/feralos.list`). All now `COND && echo '<ok>' || { echo '<fail>'; exit 1; }`; verified `btrfs_standard_dinit` **48/48** (run 2026-09-24T23-42-02Z).

**Installer**: go vet/lint/build clean. Branch `developDinit` published su gitea (275 commit). History scrubbed: 0 italiano, 0 `/home/skem` in entrambi i repo.

Phase 1: Go installer complete + fixtures library + AGENTS compress + githooks vet/lint. Highlights (latest runs **32/32**):

- **btrfsRaid0 layout** (`48ad7bc`): pure RAID0, EFI both disks, 2 LUKS swap p2 (`swapSize/2`) gated `enableSwap`, root p3 remainder, `swap_service` conditional (fixes latent bug: swap never activated on RAID0)
- **enableSwap + swapSize** (generic): `enableSwapfile`→`enableSwap`, `swapSize` `auto`=2×RAM via `utils/swap.ResolveSwapSizeMiB` or `8G`/`4096M`; `btrfsStandardServer` unchanged
- **Fixtures library** (`tests/fixtures/variables/` 14 templates): btrfsStandard/Server/Raid0 × swap on/off × swapSize auto/8G × secureboot/tpm/hardening isolate × niri × full-stack. README matrix + Tier1 CI (minimal+noswap). Old harness `variables-*.yaml` kept `DEPRECATED` → new fixtures. Verified: raid0 32/32, standard 32/32 (7.66GiB), noswap 32/32, server 32/32 — 128/128
- **Fixtures validation 2026-08-23**: `btrfsRaid0-noswap` 32/32 (header-only swap_status, RAID0 pure), `btrfsStandard`/`btrfsRaid0` `8G` explicit 32/32 (8192M file / 4177916 KiB per partition)
- **AGENTS compress**: 596→145 lines, structure moved to `docs/INSTALLER.md#project-structure` (pointer only), history truncated → `CHANGELOG.md`
- **Githooks vet/lint**: staged `*.go` → `go vet ./...` (src/harness) + `golangci-lint` (`$HOME/go/bin` fallback), fixed `grep --` leading dash bug, set `core.hooksPath=.githooks`
- **Swapfile/NoCoW fixes** — `btrfs filesystem mkswapfile` + `chattr +C` on @swap dir before mount, compression property removed from ALL NoCoW (NODATACOW implies no compression), `lsattr_nocow` harness check
- **Swap shutdown FIXED** — swap moved to boot runlevel + `need localmount` (was `before`), apk `.apk-new` survives upgrades
- **`log_errors` + `services_status`** — non-regression guards (whitelist smartd/fsck, parse supervise-daemon)
- **niri DE** — `nixPackages` in internal_rules, `chrootPhase/desktops/niri.go`, DMS via stable nixpkgs, **37/37 PASS**; tests/harness checks + virtio-gpu-pci; sudoers `includedir` enabled; nix.conf cleaned; `hwclock`? no
- **Dummy packages** — APK v3 `apk mkpkg` symlinks; BusyBox grep (not GNU/rg)

## What's Working

- Shell script installer (`internal/start.sh`) — full reference implementation
- Go installer: config loading, dynamic rules, host phase, chroot phase
- **btrfsStandardServer**: Dual-disk RAID1 layout with Btrfs subvolumes (21 subvolumes)
- **btrfsStandard**: Single-disk layout with swapfile (25 subvolumes)
- LUKS2 encryption on root and swap partitions
- **LUKS keyfile auto-unlock** — embedded in initramfs via `rd.luks.key=/keyfile:luks-<uuid>`
- booster initramfs (replaces mkinitfs)
- Signed UKI (Unified Kernel Image) for Secure Boot
- Serial console output (`console=ttyS0,115200`)
- efibootmgr guarded for BIOS runners
- APK hooks for kernel fallback and EFI sync
- Interactive wizard and standalone generator
- QEMU-based test harness with feature toggles
- Non-interactive mode for automated testing
- **Clevis TPM2 binding** — empty PCR policy (TPM identity only)
- **Vector logging** — replaces rsyslog (Rust, memory-safe); logrotate re-added for file rotation
  - Socket: `/run/vector/syslog.sock` (symlinked from `/dev/log`)
  - Sinks: `/var/log/messages`, `auth.log`, `daemon.log`, `kern.log`
  - Dinit services: `log-type=file` → `/var/log/dinit/<name>.log` + PID1 trail → `/var/log/dinit/boot.log`
  - Vector `dinit_services` file source ingests dinit logs
  - Logrotate: `/etc/logrotate.d/feralos` (copytruncate + delaycompress, daily, rotate 7)
  - Service: `vector-setup` (OpenRC) + `vector` (boot) + `dinit-boot-log` (dinit, mirrors PID1 trail)
- **btrbk** — btrfs backup/snapshot tool with subvolume `@btrbk`
- **turnstile** — session tracker for desktop environments
- **internal_rules.yaml cleaned** — only base OS in defaultRunlevels, services in packageLogic
- **APK v3 dummy packages** — `apk mkpkg` creates valid packages, installed before main `apk add`
- **providePackages** — empty dummies for dependency satisfaction (e.g., `util-linux-misc`)
- **uutils-coreutils-more** — provides `/usr/bin/more`, dummy `util-linux-misc` satisfies `incus-feature-vm`
- **Swapfile working** — `btrfs filesystem mkswapfile --size 8g` with `chattr +C` on @swap dir before mount
  - btrfsStandardServer: no swapfile (dedicated LUKS swap partitions)
- **NoCoW subvolumes clean** — no compression property anywhere (NODATACOW implies no compression)
  - `chattr +C` on dir is the only mechanism; all 7 NoCoW subvolumes verified by `lsattr_nocow` harness check
  - Compressed subvolumes verified to stay CoW (no C flag)

## What's Next

### Phase 1 (Installer)
1. earlyoom `--avoid` protection (exclude critical processes from earlyoom kills, esp. niri)
2. Test on real hardware with 2x NVMe
3. Pages race hardening (structural): harness installs feralos repo packages from CI artifact, not live Pages URL (client-test propagation race)
4. xdg-portals could become user services someday (Chimera model)
5. btrfsStandardUnencrypted layout (planned)
6. Test with partial features (Secure Boot only, TPM only)
7. Add more unit tests for verify checks

### Phase 2 (Distribution)
1. ~~Dinit init system (replace OpenRC)~~ ✅ DONE
2. Custom FeralOS repository — live on GitHub Pages
3. Full distribution release

## Architecture Principle

**Standard core, pluggable data.**

- Installer logic never changes
- Storage layouts are Go packages
- Essential packages hardcoded in Go
- Optional packages + rules in YAML
- Parser always same code, only data changes

## Key Files

| File | Purpose |
|------|---------|
| `src/main.go` | Installer entry point |
| `src/utils/config/config.go` | Config types + LoadConfig |
| `src/utils/shell/execute.go` | Unified ExecuteCmd (retry, logging) |
| `src/variables/variables.yaml` | Template with defaults |
| `src/variables/internal_rules.yaml` | Dynamic rules (packages, services) |
| `src/hostPhase/` | Host-side operations (orchestrator + internal/) |
| `src/hostPhase/disks/` | Storage layouts (Go packages) |
| `src/chrootPhase/` | Chroot operations (orchestrator + internal/ + features/) |
| `src/firstBoot/` | Standalone first-boot programs (secureboot, tpm) |
| `setup.sh` | Automated setup script |
| `tests/harness/` | QEMU-based test harness |
| `tests/debug_run/` | Debug tool for existing runs |
| `docs/INSTALLER.md` | Installer architecture documentation |
| `docs/USAGE.md` | Usage guide and configuration |
| `CONTEXT.md` | Domain vocabulary |

## Decisions Made

- **Vector socket**: `/run/vector/syslog.sock` (not `/dev/log` — permissions)
- **vector-setup**: boot stage (not sysinit — /var/log mounted by localmount)
- **Vector config**: Written AFTER package install (apk overwrites with demo_logs)
- **Vector check**: `apk info | grep -q '^vector$'` (init-system agnostic)
- **defaultRunlevels**: Only base OS services, rest in packageLogic
- **udev services**: In packageLogic under eudev (Alpine uses mdev by default)
- **modules service**: Removed (booster loads /etc/modules automatically)
- **btrbk snapshot_dir**: `/btrbk/snapshots` (not `/mnt/btrbk/snapshots`)
- **less package**: Solved via dummy package symlink (bat replaces less)
- **booster** instead of mkinitfs for initramfs
- **Signed UKI** instead of GRUB for boot
- **LUKS2** encryption on RAID and Swap partitions
- **Btrfs RAID1** on LUKS (not mdadm)
- **Phase separation**: hostPhase (writes files) vs chrootPhase (runs commands)
- **Ripgrep NOT a drop-in for grep**: rg doesn't support POSIX BRE (`\+`), breaks `efi-mkuki`
- **GNU grep removed**: BusyBox grep provides POSIX BRE, zero reverse dependencies
- **Dummy packages**: Symlink approach (APK content, not wrapper scripts)
- **Swapfile**: `btrfs filesystem mkswapfile` + `chattr +C` on dir before mount (Debian guide)
- **NoCoW**: no compression property — `chattr +C` on dir only (NODATACOW implies no compression, `m` attribute conflicts with C)
- **Swap service**: boot runlevel + `need localmount` (sed replaces `before localmount` — start order only, not stop order). apk preserves modified /etc files as `.apk-new` — the sed survives upgrades
- **Log errors check**: harness `log_errors` scans target-serial.log + installer.log for error/failed markers, fails run on any match (non-regression guard, whitelist for known innocuous lines incl. QEMU-only smartd/fsck)
- **Services active check**: harness `services_status` verifies configured services are actually started via `rc-status --all`; excludes QEMU-only services (smartd, fsck, udev-settle); parses supervise-daemon format
- **niri DE**: data in internal_rules (`nixPackages` field, auto-enables Nix) + config logic in `chrootPhase/desktops/niri.go` (kde/gnome stay data-only)
- **Nix stable**: registry pinned to nixos-26.05 (default maps nixpkgs→unstable); sandbox disabled only for chroot install; TMPDIR on nix build subvolume; NIXPKGS_ALLOW_* env vars (not nix.conf — unknown settings)
- **Seat provider**: elogind (not seatd); no render group (Alpine uses video); no turnstile (edge only)
- **sudoers**: `@includedir /etc/sudoers.d` enabled (Alpine default comments it out — wheel sudo was denied)
- **fstab swap**: `/dev/mapper/swap-*` (not `LABEL=swap-*` — busybox `swapon -a` can't resolve LABEL, literal path only)
- **dmcrypt keyfile**: written directly into chroot after subvol mounts (no /tmp + copy, mappers still open, `apk` preserves as `.apk-new`)
- **enableSwap + swapSize**: generic toggle (`auto`=2×RAM via `utils/swap.ResolveSwapSizeMiB`), `swapSize` total split across partitions, `btrfsRaid0` swap on p2 root on p3 remainder, `btrfsStandardServer` unchanged (always swap)
- **AGENTS compress**: pointer-only structure + history → `CHANGELOG.md` + `git log` + `.agent-memory` (<250 lines)
- **githooks**: staged Go → `go vet` + `golangci-lint` (`$HOME/go/bin` fallback), `grep --` fix for `-----BEGIN`

## Memory-Safe Alternatives

| Traditional (C) | FeralOS Alternative | Language | Drop-in? |
|-----------------|------------------|----------|----------|
| coreutils | uutils-coreutils | Rust | No |
| vim/nano | micro | Go | No |
| sudo | sudo-rs | Rust | No |
| rsyslog/logrotate | vector | Rust | No |
| bash/zsh | fish | Rust | No |
| lxd | incus | Go | No |
| openrc | dinit | C++/Alpine | ✅ DONE |
| grep (GNU) | BusyBox grep | POSIX BRE | Yes (built-in) |
| find (GNU) | fd | Rust | No (different syntax) |
| less | bat (via `less-dummy` wrapper pkg) | Rust + sh | Flags eaten, dumps (no interactive paging) |

---

*See `state/session_state.json` for current goals, decisions, risks, and next actions.*
*See `AGENTS.md` for project context and milestone history.*
*See `docs/PROJECT.md` for full distribution vision.*
