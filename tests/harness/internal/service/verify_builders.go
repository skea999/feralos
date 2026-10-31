package service

// P2-4: Verify's check-building phase, split into domain builders. Each
// builder fills the checks map with the SAME payloads the pre-refactor
// inline code produced — byte-identity is proven by golden_test.go. Verify
// itself keeps orchestration + the evaluation chain.

import "fmt"

// buildStorageChecks — disk layout, fstab, subvolumes, swap/zswap,
// maintenance schedule and the apk inventory (storage domain).
func buildStorageChecks(checks map[string]string) {
	checks["storage_layout"] = "printf '===blkid===\n'; blkid; printf '\n===mapper===\n'; ls -1 /dev/mapper/luks-* 2>/dev/null; printf '\n===mounts===\n'; grep ' / ' /proc/mounts; printf '\n===btrfs_show===\n'; btrfs filesystem show /; printf '\n===btrfs_usage===\n'; btrfs filesystem usage -T /"
	checks["fstab"] = "cat /etc/fstab"
	checks["fstab_vs_mount"] = `echo "=== FSTAB vs MOUNT COMPARISON ===" && for mp in $(awk '/btrfs/ && !/^#/ {print $2}' /etc/fstab | sort -u); do fstab_opts=$(awk -v mp="$mp" '$2==mp && /btrfs/ && !/^#/ {print $4}' /etc/fstab | head -1); mount_opts=$(awk -v mp="$mp" '$2==mp {print $4}' /proc/mounts | head -1); if [ -n "$fstab_opts" ] && [ -n "$mount_opts" ]; then echo "=== $mp ==="; echo "  fstab:  $fstab_opts"; echo "  mount:  $mount_opts"; for opt in $(echo "$fstab_opts" | tr ',' '\n' | sort); do if ! echo "$mount_opts" | tr ',' '\n' | grep -q "^${opt}$"; then echo "  MISSING in mount: $opt"; fi; done; for opt in $(echo "$mount_opts" | tr ',' '\n' | sort); do if ! echo "$fstab_opts" | tr ',' '\n' | grep -q "^${opt}$"; then echo "  EXTRA in mount: $opt"; fi; done; fi; done; echo "=== DONE ==="`
	checks["disk_schedule"] = "test -x /usr/libexec/feralos-weekly-trim && echo 'trim script OK' || echo 'trim script MISSING'; for pat in 'feralos-weekly-trim' 'scrub start' 'btrbk run'; do n=$(grep -c \"$pat\" /etc/crontabs/root 2>/dev/null); echo \"cron [$pat]: ${n:-0}\"; done"
	checks["zswap_status"] = "cat /sys/kernel/debug/zswap/ 2>/dev/null || cat /sys/module/zswap/parameters/enabled 2>/dev/null || echo 'zswap not available'"
	checks["swap_status"] = "swapon -s 2>/dev/null || cat /proc/swaps 2>/dev/null || echo 'no swap active'"
	checks["apk_packages"] = "apk info | sort"
	checks["lsattr_nocow"] = "echo '=== NoCoW subvolume attributes ==='; for dir in /swap /srv /var/lib/ex /var/log /var/spool /var/tmp /nix/var/log; do echo \"=== $dir ===\"; lsattr -d \"$dir\" 2>&1; f=$(find \"$dir\" -type f 2>/dev/null | head -1); if [ -n \"$f\" ]; then lsattr \"$f\" 2>&1; else echo 'no files in dir'; fi; done; echo '=== Compressed subvolume attributes ==='; for dir in / /home /opt /root /tmp /var /var/lib/libvirt /var/lib/incus /var/lib/containerd /var/lib/docker /var/lib/containers /var/lib/vector /var/cache /var/opt /usr/local /boot /nix/store /nix/var/nix /nix/var/nix/build /btrbk; do echo \"=== $dir ===\"; lsattr -d \"$dir\" 2>&1; done"
}

// buildBootChecks — kernel cmdline, booster/UKI artifacts, installer
// config-file markers, identity, TPM/clevis surface, services_status feed,
// sudo stack, less shim and the conf.d dump (boot/system domain).
func buildBootChecks(cfg *VerifyConfig, checks map[string]string) {
	checks["kernel_cmdline"] = "cat /etc/kernel/cmdline"
	checks["booster_conf"] = "cat /etc/booster.yaml"
	checks["uki_artifacts"] = "ls -lah /boot/efi/Alpine /boot/efi/EFI/BOOT || true"
	checks["booster_image"] = "ls -lah /boot/booster-lts.img || true"
	checks["subvolumes"] = "btrfs subvolume list / || true"
	checks["incus_packages"] = "apk info | grep '^incus-feature' || true"
	checks["sshd_service"] = "rc-update show | grep sshd || true"
	checks["installed_user"] = "id " + shellQuote(cfg.InstalledUser)
	checks["hostname"] = "cat /etc/hostname"
	checks["network_interfaces"] = "ip addr || ifconfig || true"
	checks["tpm_devices"] = "ls -1 /dev/tpm* 2>/dev/null || true"
	checks["clevis_packages"] = "apk info | grep -E '^(bash|clevis|jose|luksmeta|tpm2-tools|tpm2-tss|tpm2-tss-tcti-device)$' || true"
	checks["clevis_bindings"] = "for dev in $(blkid -o device -t TYPE=crypto_LUKS 2>/dev/null); do printf '===%s===\\n' \"$dev\"; clevis luks list -d \"$dev\" 2>/dev/null || cryptsetup token list \"$dev\" || true; done"
	checks["confd_dump"] = "for f in /etc/conf.d/*; do echo \"===$f===\"; cat \"$f\" 2>/dev/null; done; echo '===END==='"
	checks["sudo_installed"] = "apk info | grep '^sudo-rs$' && echo 'sudo-rs installed' || echo 'sudo-rs not installed'"
	checks["sudo_wheel"] = "grep '^%wheel' /etc/sudoers.d/wheel 2>/dev/null || echo 'no wheel sudoers config'"
	checks["sudoers_includedir"] = "grep -q '^@includedir' /etc/sudoers && echo 'includedir enabled' || echo 'includedir disabled'"
	checks["sudo_works"] = "sudo -n id 2>&1 || echo 'sudo requires password (expected for non-root)'"
	// less shim (feralos-aports less-dummy): /usr/bin/less is a wrapper
	// script (NOT a symlink — moor rejects the hardcoded less flags tools
	// pass to their pager: `-R -F -K --no-init` → "flag provided but not
	// defined: -R", broke fish `history`). The wrapper drops every less
	// flag and execs moor, which pages interactively on a TTY and passes
	// content through on a pipe. Falsifiable, 5 assertions: package
	// installed + regular file + moor dependency present + less flag set
	// accepted (dropped by the wrapper) + content actually passes through
	// (stdin, file operand, `--` form). moor parses args BEFORE any TTY
	// check, so the flag test is a real discriminator even on the non-TTY
	// verify SSH.
	checks["less_dummy"] = `fail=0
apk info -e less-dummy || { echo 'FAIL: less-dummy not installed'; exit 1; }
test -f /usr/bin/less && ! test -L /usr/bin/less || { echo 'FAIL: /usr/bin/less is not a regular file'; exit 1; }
apk info -e moor || { echo 'FAIL: moor not installed (less-dummy dependency)'; exit 1; }
out=$(printf 'shim-marker-123\n' | /usr/bin/less -R -F -K --no-init 2>&1) || { echo "FAIL: less flag set rejected: $out"; exit 1; }
[ -n "$out" ] || { echo 'FAIL: wrapper output empty'; exit 1; }
echo "$out" | grep -q 'shim-marker-123' || { echo "FAIL: stdin content lost: $out"; exit 1; }
printf 'file-marker-456\n' > /tmp/.less-test
out=$(/usr/bin/less /tmp/.less-test 2>&1) || { echo "FAIL: file operand rejected: $out"; exit 1; }
echo "$out" | grep -q 'file-marker-456' || { echo "FAIL: file content lost: $out"; exit 1; }
printf 'dash-marker-789\n' > /tmp/.less-test-dash
out=$(/usr/bin/less -- /tmp/.less-test-dash 2>&1) || { echo "FAIL: -- form rejected: $out"; exit 1; }
echo "$out" | grep -q 'dash-marker-789' || { echo "FAIL: -- content lost: $out"; exit 1; }
rm -f /tmp/.less-test /tmp/.less-test-dash
echo 'less shim OK: pkg+moor installed, regular file, less flags dropped, moor passthrough stdin+file+--'`
	// The ORIGINAL bug: fish `history` piped to $PAGER on a TTY and got
	// an arg-parse error ("unexpected argument '-R'") instead of history.
	// Guard: run fish history and fail on ANY arg-parse error text in the
	// output (moor never produces this — the wrapper eats the flags).
	checks["less_fish_history"] = `fish -ic 'history' > /tmp/.fish-h.out 2>&1
if grep -q 'unexpected argument' /tmp/.fish-h.out; then echo 'FAIL: clap error in fish history output:'; cat /tmp/.fish-h.out; rm -f /tmp/.fish-h.out; exit 1; fi
rm -f /tmp/.fish-h.out
echo 'fish history clean (no clap error)'`
	// Bounded boot-wait before the snapshot: oneshots inside boot (e.g.
	// webmin-ensure pulling the Webmin image) can keep boot.target
	// "not-started" for minutes on a cold cache — poll up to 180s, then
	// snapshot as-is (the eval judges the final state).
	checks["services_status"] = "if [ -S /run/dinitctl ]; then i=0; while [ $i -lt 90 ]; do dinitctl list 2>/dev/null | grep -qE '^\\[\\[\\+\\] *boot' && break; sleep 2; i=$((i+1)); done; dinitctl list; else rc-status --all 2>/dev/null || rc-status 2>/dev/null; fi"
	// Security checks — read installer.log from HOST (not target via SSH)
	// Config file markers — verify installer wrote these files (not just Alpine defaults)
	checks["cmdline_marker"] = "grep -c 'Generated by FeralOS' /etc/kernel/cmdline 2>/dev/null || echo 0"
	checks["booster_marker"] = "grep -c 'Generated by FeralOS' /etc/booster.yaml 2>/dev/null || echo 0"
	checks["fstab_marker"] = "grep -c 'Generated by FeralOS' /etc/fstab 2>/dev/null || echo 0"
	checks["network_marker"] = "grep -c 'Generated by FeralOS' /etc/network/interfaces 2>/dev/null || echo 0"
}

// buildLogChecks — vector/klogd logging chain (logging domain).
func buildLogChecks(checks map[string]string) {
	checks["vector_service"] = "if [ -S /run/dinitctl ]; then dinitctl list 2>/dev/null | grep vector || test -e /etc/dinit.d/boot.d/vector && echo \"vector (boot.d)\" || test -e /lib/dinit.d/vector && echo \"vector (service file)\"; else rc-status boot 2>/dev/null | grep vector || rc-update show 2>/dev/null | grep vector || true; fi"
	checks["vector_logs"] = "ls -la /var/log/vector*.log 2>/dev/null || echo 'no vector log files'"
	checks["vector_running"] = "pgrep -f '/usr/bin/vector' 2>/dev/null && echo 'running' || echo 'vector not running'"
	checks["vector_diag"] = "rc-status boot 2>/dev/null | grep vector; pgrep -la vector 2>/dev/null; ls -la /var/log/vector* 2>/dev/null; echo '===vector.log==='; cat /var/log/vector.log 2>/dev/null | tail -30; echo '===end==='; vector --version 2>&1 || true"
	checks["klogd_service"] = "if [ -S /run/dinitctl ]; then dinitctl list 2>/dev/null | grep klogd || test -e /etc/dinit.d/boot.d/klogd && echo 'klogd (boot.d)'; else rc-status boot 2>/dev/null | grep klogd || rc-update show 2>/dev/null | grep klogd || echo 'klogd not wired'; grep -q 'klogd (vector)' /etc/init.d/klogd && echo 'klogd vector override file' || echo 'klogd override file MISSING'; fi"
	checks["klogd_running"] = "pidof klogd >/dev/null 2>&1 && echo 'klogd running' || echo 'klogd not running'"
	checks["dmesg_saved"] = "test -s /var/log/dmesg && grep -q 'Linux version' /var/log/dmesg && echo 'dmesg saved' || echo 'dmesg dump missing'"
	checks["kern_log"] = "test -s /var/log/kern.log && echo \"kern.log $(wc -l < /var/log/kern.log) lines\" || echo 'kern.log empty'"
	checks["json_log"] = "grep -q '\"timestamp\"' /var/log/feralos.json.log 2>/dev/null && echo \"json log OK ($(wc -l < /var/log/feralos.json.log) lines)\" || echo 'json log missing'"
	checks["vector_api"] = "wget -qO- --timeout=3 http://127.0.0.1:8686/health 2>/dev/null || echo 'api unreachable'"
	checks["vector_validate"] = "vector validate --skip-healthchecks /etc/vector/vector.yaml >/dev/null 2>&1 && echo 'vector config valid' || echo 'vector config INVALID'"
	checks["log_coverage"] = "for f in messages auth.log daemon.log kern.log sudo.log firewalld.log feralos.json.log vector.log dmesg; do if [ -s /var/log/$f ]; then echo \"$f: $(wc -l < /var/log/$f) lines\"; else echo \"$f: MISSING\"; fi; done"
}

// buildFeatureChecks — nix/podman/webmin/secureboot/flatpak feature
// checks plus the always-registered nix config markers (feature domain).
func buildFeatureChecks(cfg *VerifyConfig, checks map[string]string) {
	checks["trusted_users_clean"] = fmt.Sprintf("! grep -q %q /etc/nix/nix.conf", cfg.InstalledUser)
	checks["nix_packages_missing"] = fmt.Sprintf("su - %s -c 'nix profile list 2>/dev/null' | grep -oE \"quickshell|dms-shell|rclone-ui|rclone|mission-center|cups-browsed\" | sort -u | wc -l", cfg.InstalledUser) // Flatpak checks — generic, only when a fixture explicitly requests
	// flatpakPackages (none ship by default; niri uses wayvnc, not RustDesk).
	if len(cfg.FlatpakPackages) > 0 {
		checks["flatpak_installed"] = "flatpak list 2>/dev/null | wc -l"
	}

	// Podman checks — container runtime
	if cfg.EnablePodman {
		checks["podman_installed"] = "podman --version"
		checks["podman_running"] = "podman ps"
		checks["docker_symlink"] = "docker ps"
	}

	// Nix tmp exec — nix builds use the upstream default build dir (/tmp),
	// so /tmp must be mounted WITHOUT noexec (old /nix/var/nix/build/tmp
	// workaround is gone).
	if cfg.EnableNix {
		checks["nix_tmp_exec"] = "mount | grep ' /tmp ' | grep -q noexec && echo 'noexec!' || echo 'exec ok'"
		// trusted-users = root @wheel (NixOS default): wheel is trusted,
		// the session user is NEVER individually injected into nix.conf.
		checks["nix_trusted_wheel"] = "grep -q 'trusted-users = root @wheel' /etc/nix/nix.conf"
		// nix_registry_stable: flake registry pinned to the stable branch
		// (26.05) — the builtin default maps nixpkgs → nixpkgs-unstable and
		// would override the stable channel for `nixpkgs#...` references.
		// System registry file (covers root) + user registry both pinned.
		checks["nix_registry_stable"] = fmt.Sprintf("grep -q 'nixos-26.05' /etc/nix/registry.json && su - %s -c 'nix registry list 2>/dev/null' | grep -q 'nixpkgs/nixos-26.05'", cfg.InstalledUser)
	}

	// Webmin checks — functional assertions: container running, port mapped,
	// HTTP answering. Binary runs on first boot (runWebminOnTarget);
	// webmin-ensure oneshot restarts the container on this verification boot.
	// No "|| echo" fallbacks — these must FAIL when webmin doesn't work.
	if cfg.EnableWebmin {
		checks["webmin_binary"] = "test -x /opt/feralos_webmin"
		checks["webmin_container"] = "podman ps --format '{{.Names}}' | grep -q '^webmin$'"
		checks["webmin_port"] = "podman port webmin 10000 | grep -q 10000"
		checks["webmin_http"] = "for i in 1 2 3 4 5 6 7 8; do wget -q -T 5 -O /dev/null --no-check-certificate https://localhost:10000/ && exit 0; sleep 5; done; exit 1"
	}

	if cfg.EnableSecureboot {
		checks["secureboot_conf"] = "cat /etc/kernel-hooks.d/secureboot.conf"
	}
}

// buildDinitChecks — dinit PID1/cmdline/control socket, repo+key,
// shutdown command wiring and the conf.d-equivalent config files
// (dinit domain; enablement conditional on EnableDinit).
func buildDinitChecks(cfg *VerifyConfig, checks map[string]string) {
	isNiriFamily := isNiriFamilyDE(cfg.DesktopEnvironment)
	// Dinit checks — verify dinit is PID 1, base services are running,
	// cmdline contains init=, and dinit-chimera suite is installed.
	// These MUST be falsifiable: each command exits non-zero when the
	// condition is false. A bare "&& echo ok || echo fail" fallback always
	// exits 0 and would make the check unconditionally green.
	if cfg.EnableDinit {
		checks["dinit_cmdline"] = "grep -q 'init=/usr/libexec/dinit/init' /proc/cmdline && echo 'init= OK' || { echo 'init= MISSING'; exit 1; }"
		// Busybox ps has no -p flag — read /proc/1/comm directly.
		checks["dinit_pid1"] = "[ \"$(cat /proc/1/comm)\" = dinit ] && echo 'dinit PID1' || { echo 'NOT dinit'; exit 1; }"
		// dinitctl status needs <service-name>; bare `status` is a usage
		// error. `dinitctl list` proves the control socket is functional.
		checks["dinitctl_status"] = "dinitctl list || { echo 'dinitctl failed'; exit 1; }"
		checks["dinit_services"] = "dinitctl list | grep -E 'boot|getty' || { echo 'no dinit services'; exit 1; }"
		checks["dinit_chimera_installed"] = "apk info -e dinit-chimera && echo 'installed' || { echo 'NOT installed'; exit 1; }"
		checks["dinit_getty_installed"] = "apk info -e getty-dinit && echo 'installed' || { echo 'NOT installed'; exit 1; }"
		// feralos-keyring installs the repo definition into
		// /etc/apk/repositories.d/feralos.list (not /etc/apk/repositories).
		checks["dinit_feralos_repo"] = "grep -q 'feralos-aports' /etc/apk/repositories.d/feralos.list && echo 'repo OK' || { echo 'repo MISSING'; exit 1; }"
		checks["dinit_key"] = "test -f /etc/apk/keys/feralos.rsa.pub && echo 'key OK' || { echo 'key MISSING'; exit 1; }"
		// Conventional shutdown commands wired to dinit helpers by the
		// installer (busybox poweroff/reboot do not work under dinit).
		// + elogind shutdown hook: loginctl poweroff/reboot (DMS power menu)
		// reaches dinit via /etc/elogind/system-shutdown — elogind's own
		// exec target is the busybox /sbin/poweroff (SIGUSR2, ignored).
		// (The polkit power rule is asserted separately by power_polkit_rule
		// — written for both inits.)
		// + Custom reboot seed per DE: niri via DMS customPowerActionReboot
		// in settings.json; niriWayle via the seeded wayle config.toml
		// dropdown-reboot-command — both `sudo -n /usr/bin/reboot` via
		// /etc/sudoers.d/zz-power (must sort AFTER /etc/sudoers.d/wheel
		// (`%wheel ALL=(ALL) ALL`; sudo-rs last-match-wins would override
		// the NOPASSWD tag otherwise — live-caught 2026-10-11). The zz-power
		// rule is written by the niri/niriWayle owners only (DMS/Wayle session
		// commands); KDE/cosmic use their own session tools + the elogind hook,
		// so the rule must NOT be demanded there.
		shutdownCmd := "test -x /usr/bin/poweroff -a -x /usr/bin/reboot -a -x /usr/bin/halt -a -x /usr/bin/shutdown -a -x /usr/bin/soft-reboot && test -x /etc/elogind/system-shutdown/50-feralos-dinit.sh && grep -q dinit-poweroff /etc/elogind/system-shutdown/50-feralos-dinit.sh"
		if isNiriFamily {
			shutdownCmd += " && test -f /etc/sudoers.d/zz-power && grep -q '/usr/bin/reboot' /etc/sudoers.d/zz-power"
		}
		if cfg.DesktopEnvironment == desktopNiri {
			shutdownCmd += fmt.Sprintf(" && grep -q customPowerActionReboot /home/%s/.config/DankMaterialShell/settings.json", cfg.InstalledUser)
		}
		if cfg.DesktopEnvironment == desktopNiriWayle {
			shutdownCmd += fmt.Sprintf(" && grep -q 'dropdown-reboot-command = \"sudo -n /usr/bin/reboot\"' /home/%s/.config/wayle/config.toml", cfg.InstalledUser)
		}
		checks["dinit_shutdown_commands"] = shutdownCmd + " && echo 'shutdown commands OK' || { echo 'shutdown commands MISSING'; exit 1; }"
		// conf.d-equivalent config files (Artix pattern): every -dinit
		// package ships /etc/dinit.d/config/<svc>.conf sourced by its
		// wrapper. Core set on any dinstall: dbus chronyd cronie acpid sshd
		// vector klogd earlyoom + feature-gated (nftables, iptables, cups,
		// greetd, ...) — 10 is the conservative floor. Spot-check that a
		// service file actually points at its wrapper (the wiring this
		// round exists to establish).
		checks["dinit_config"] = "test -d /etc/dinit.d/config && c=$(ls /etc/dinit.d/config/*.conf 2>/dev/null | wc -l) && [ \"$c\" -ge 10 ] && grep -q 'usr/lib/dinit/dbus' /lib/dinit.d/dbus && grep -q 'KLOGD_OPTS' /etc/dinit.d/config/klogd.conf || { echo \"config files: $c (need >=10) or wiring broken\"; exit 1; }"
	}
}

// buildDesktopChecks — niri-family statics (config validation, greeter,
// power polkit/PPD), DMS-only checks, bluetooth/wifi/keymap, session
// wiring, niri runtime/IPC/rendering checks and the niriWayle-only Wayle/
// Veila/Raffi/oo7 checks (desktop domain).
func buildDesktopChecks(cfg *VerifyConfig, checks map[string]string) {
	isNiriFamily := isNiriFamilyDE(cfg.DesktopEnvironment)
	if isNiriFamily {
		checks["niri_config_valid"] = fmt.Sprintf("niri validate --config /var/lib/greetd/.config/niri/config.kdl && niri validate --config /home/%s/.config/niri/config.kdl && grep -q '^prefer-no-csd' /home/%s/.config/niri/config.kdl && grep -q 'QT_WAYLAND_DISABLE_WINDOWDECORATION \"1\"' /home/%s/.config/niri/config.kdl && echo 'niri config valid (prefer-no-csd + Qt no-decoration; hotkey overlay intentionally NOT skipped)'", cfg.InstalledUser, cfg.InstalledUser, cfg.InstalledUser)
		checks["greetd_config"] = "ls /etc/greetd/config.toml /etc/greetd/greeter.sh /etc/greetd/regreet.toml /var/lib/greetd/.config/niri/config.kdl && test -x /etc/greetd/greeter.sh && [ \"$(id -u greeter)\" -lt 1000 ] && echo \"greeter is a system user (hidden from ReGreet's UID-filtered list)\""
		// greetd_socket: greetd daemon alive (IPC socket). Runtime complement
		// to the static greetd_config check. Works on BOTH init systems —
		// regreet itself cannot be runtime-checked on verify boots (the
		// harness test-only autologin starts the user session directly, so
		// the greeter UI never runs there), but greetd always supervises.
		// NOTE: greetd's default socket path is PID-suffixed
		// (/run/greetd-<pid>.sock, verified on target: /run/greetd-1581.sock)
		// — clients get the real path via the GREETD_SOCKET env, so glob it.
		checks["greetd_socket"] = "i=0; while [ $i -lt 30 ]; do for s in /run/greetd-*.sock; do test -S \"$s\" && { echo \"greetd IPC socket OK ($s)\"; exit 0; }; done; sleep 1; i=$((i+1)); done; echo 'no greetd socket'; exit 1"
		// greeter compositor: greetd/niri start late on some QEMU boots
		// (turnstile/user-dinit + apparmor storm delay greetd's dinit start;
		// observed PID ~1570 vs ~1340 on cold boots) — poll up to 60s, else
		// the check races boot. Whole Verify budget is 2min; other checks
		// take ~30s, so 60s worst case still fits.
		checks["niri_greeter"] = "i=0; while [ $i -lt 60 ]; do pgrep -x niri >/dev/null && { echo 'niri running'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'niri not running'; exit 1"
		// power_polkit_rule: the wheel login1 rule (written by the installer
		// for BOTH inits — harmless under OpenRC, required under dinit
		// because elogind cannot attribute PIDs to sessions there
		// (NoSessionForPID): every loginctl power action would otherwise
		// become an auth challenge; wheel = admin already, standard desktop
		// posture). Covers the DMS/Wayle power buttons and stasis' automatic
		// `loginctl suspend` idle step.
		checks["power_polkit_rule"] = "test -f /etc/polkit-1/rules.d/49-feralos-power.rules && grep -q login1.power-off /etc/polkit-1/rules.d/49-feralos-power.rules && grep -q login1.suspend /etc/polkit-1/rules.d/49-feralos-power.rules && grep -q org.freedesktop.UPower.PowerProfiles.switch-profile /etc/polkit-1/rules.d/49-feralos-power.rules && grep -q isInGroup /etc/polkit-1/rules.d/49-feralos-power.rules && echo 'power polkit rule OK (wheel: power-off/reboot/halt/suspend + power profiles)'"
		// power_profiles: power-profiles-daemon reachable AND switchable
		// from the session (Wayle battery dropdown; DMS power UI shares it).
		// Alpine ships PPD D-Bus ACTIVATED (no init script by design — the
		// 0001-dbus-no-systemd patch sets Exec=/usr/libexec/... on the
		// shared service template, so BOTH bus names are activatable:
		// net.hadess.PowerProfiles alias + org.freedesktop.UPower
		// .PowerProfiles). The static half asserts the activation wiring +
		// the polkit policy (power-profiles-daemon.policy; action ids
		// renamed in 0.30). The runtime half performs a REAL switch:
		// `set power-saver` walks ...PowerProfiles.switch-profile
		// (allow_active=yes upstream — under dinit only the wheel polkit
		// rule authorizes it), asserts it took effect, then restores the
		// previous profile. The placeholder driver (QEMU: no
		// platform-profile hardware) still offers power-saver + balanced.
		// power_profiles: power-profiles-daemon reachable AND switchable
		// (Wayle battery dropdown / DMS power UI). ADR-010: both activation
		// files route through the dinit shim to the
		// org.freedesktop.UPower.PowerProfiles unit. The unit is ACTIVATION-
		// ONLY (ADR-010 R1 amendment 2026-10-25: dbus-by-design daemon — dbus
		// decides when it runs; never boot-enabled). Override sources:
		// /usr/share/feralos/dbus-system-services (materialized at boot into
		// /usr/local/share — see dbus_activation_uniform). Runtime: a REAL
		// switch as the session user (exercises the wheel polkit rule AND the
		// on-demand activation), then the dinit started-marker proves the
		// unit came up through dbus activation → shim → dinit.
		powerProfilesCmd := "test -f /usr/share/feralos/dbus-system-services/org.freedesktop.UPower.PowerProfiles.service && grep -q 'Exec=/usr/libexec/feralos/dbus-activate org.freedesktop.UPower.PowerProfiles' /usr/share/feralos/dbus-system-services/org.freedesktop.UPower.PowerProfiles.service && test -f /usr/share/feralos/dbus-system-services/net.hadess.PowerProfiles.service && grep -q 'Exec=/usr/libexec/feralos/dbus-activate org.freedesktop.UPower.PowerProfiles' /usr/share/feralos/dbus-system-services/net.hadess.PowerProfiles.service && test -f /lib/dinit.d/org.freedesktop.UPower.PowerProfiles || { echo 'PPD dinit activation wiring missing'; exit 1; }; test -f /usr/share/polkit-1/actions/power-profiles-daemon.policy && grep -q 'org.freedesktop.UPower.PowerProfiles.switch-profile' /usr/share/polkit-1/actions/power-profiles-daemon.policy || { echo 'PPD polkit policy missing'; exit 1; };"
		powerProfilesCmd += fmt.Sprintf(" su - %s -s /bin/sh -c 'p=$(powerprofilesctl get) || exit 1; powerprofilesctl set power-saver || exit 1; n=$(powerprofilesctl get); powerprofilesctl set \"$p\" || exit 1; if [ \"$n\" = power-saver ]; then echo \"power profiles OK (real switch: $p -> power-saver -> $p)\"; else echo \"power profile switch did not take effect ($n)\"; exit 1; fi'", cfg.InstalledUser)
		if cfg.EnableDinit {
			powerProfilesCmd += "; dinitctl list 2>/dev/null | grep -Eq '(\\{\\+|\\[\\+).*\\] org\\.freedesktop\\.UPower\\.PowerProfiles( |$)' && echo 'PPD started on demand (dbus activation -> shim -> dinit)' || { echo 'PPD unit not started after the real switch (activation routing broken)'; exit 1; };"
		}
		checks["power_profiles"] = powerProfilesCmd
	}
	if isNiriFamily {
		// dbus_activation_units_live: real activation proof for the third-party
		// targets covered by the new units (ADR-010): one system daemon
		// (org.freedesktop.UDisks2 → udisksd) and one session daemon
		// (org.gnome.OnlineAccounts → goa-daemon). Asserts the dinit unit
		// started via the shim AND the daemon environ has no DBUS_STARTER_*
		// (a raw helper spawn leaves them; the shim path excludes them).
		// The system half needs a system dinit (dinit installs only); the
		// session half runs on both inits via the turnstile user dinit.
		checks["dbus_activation_units_live"] = fmt.Sprintf(`
if [ "$(cat /proc/1/comm)" = "dinit" ]; then
    pkill -f "^/usr/libexec/udisks2/udisksd$" 2>/dev/null || true
    dinitctl --system stop org.freedesktop.UDisks2 >/dev/null 2>&1 || true
    sleep 1
    dbus-send --system --print-reply --dest=org.freedesktop.UDisks2 /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping >/dev/null 2>&1 || true
    i=0; ok=0
    while [ $i -lt 30 ]; do
        if dinitctl --system list 2>/dev/null | grep -Eq "(\{\+|\[\+).*\] org\.freedesktop\.UDisks2( |$)" && pgrep -f "^/usr/libexec/udisks2/udisksd$" >/dev/null 2>&1; then ok=1; break; fi
        sleep 1; i=$((i+1))
    done
    if [ "$ok" != 1 ]; then dinitctl --system status org.freedesktop.UDisks2 2>&1 || true; dinitctl --system list 2>/dev/null | grep -i udisks || true; echo "system: UDisks2 unit did not start via dbus activation"; exit 1; fi
    pid=$(pgrep -f "^/usr/libexec/udisks2/udisksd$" | head -1)
    if tr "\0" "\n" < /proc/$pid/environ | grep -q "^DBUS_STARTER_"; then echo "system: udisksd spawned raw (DBUS_STARTER_* present)"; exit 1; fi
    dinitctl --system stop org.freedesktop.UDisks2 >/dev/null 2>&1 || true
    echo "system unit activation OK (UDisks2 via shim + dinit)"
else
    echo "system unit activation SKIPPED (no system dinit)"
fi
su - %s -s /bin/sh -c '
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
dinitctl --user stop org.gnome.OnlineAccounts.user >/dev/null 2>&1 || true
pkill -f "^/usr/libexec/goa-daemon$" 2>/dev/null || true
sleep 1
dbus-send --session --print-reply --dest=org.gnome.OnlineAccounts /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping >/dev/null 2>&1 || true
i=0; ok=0
while [ $i -lt 20 ]; do
    if dinitctl --user list 2>/dev/null | grep -Eq "(\{\+|\[\+).*\] org\.gnome\.OnlineAccounts\.user( |$)" && pgrep -f "^/usr/libexec/goa-daemon$" >/dev/null 2>&1; then ok=1; break; fi
    sleep 1; i=$((i+1))
done
if [ "$ok" != 1 ]; then echo "session: goa unit did not start via dbus activation"; dinitctl --user list 2>/dev/null | grep -i onlineaccounts || true; exit 1; fi
pid=$(pgrep -f "^/usr/libexec/goa-daemon$" | head -1)
if tr "\0" "\n" < /proc/$pid/environ | grep -q "^DBUS_STARTER_"; then echo "session: goa spawned raw (DBUS_STARTER_* present)"; exit 1; fi
dinitctl --user stop org.gnome.OnlineAccounts.user >/dev/null 2>&1 || true
echo "session unit activation OK (goa via shim + dinit)"
' || exit 1
echo 'dbus activation units live OK'
`, cfg.InstalledUser)
		// DMS-only (niri DE): niriWayle has NO DMS — Wayle is the shell.
		if cfg.DesktopEnvironment == desktopNiri {
			checks["dms_installed"] = "apk info -e dms-shell quickshell && test -x /usr/bin/dms"
			// dms_window_buttons: the window-controls plugin chain — package
			// installed (system dir /etc/xdg/quickshell/dms-plugins), plugin
			// enabled in plugin_settings.json, widget wired in the bar's right
			// section (settings.json seed).
			checks["dms_window_buttons"] = fmt.Sprintf("apk info -e dms-window-buttons && test -f /etc/xdg/quickshell/dms-plugins/dmsWindowButtons/plugin.json && test -f /etc/xdg/quickshell/dms-plugins/dmsWindowButtons/WindowButtons.qml && grep -q dmsWindowButtons /home/%s/.config/DankMaterialShell/plugin_settings.json && grep -q dmsWindowButtons /home/%s/.config/DankMaterialShell/settings.json && echo 'window buttons plugin OK'", cfg.InstalledUser, cfg.InstalledUser)
		}
		// matugen: DMS dynamic-theming backend (feralos-aports 4.2.0, absent
		// in Alpine). dms-shell runs it for wallpaper palettes; the generated
		// DankMatugen KColorScheme export is what the theme-sync plugin
		// propagates to GTK/Qt/KDE. Package present + binary runs (musl).
		checks["matugen_installed"] = `apk info -e matugen >/dev/null && v=$(matugen --version 2>&1) && echo "matugen OK ($v)" || { echo "matugen BROKEN"; exit 1; }`
		// dms_icons: the DMS icon chain end-to-end. DMS resolves icons via
		// QT_QPA_PLATFORMTHEME=gtk3 → ~/.config/gtk-3.0/settings.ini →
		// gtk-icon-theme-name — unset it falls back to hicolor generics
		// (the missing-icons effect). Two assertions: icon theme PACKAGES
		// present (dms-shell deps: adwaita + hicolor) and the Adwaita theme
		// actually ON DISK (index.theme). The settings.ini is NOT seeded
		// anymore — dms-theme-sync (plugin) writes it at runtime.
		checks["dms_icons"] = "apk info -e adwaita-icon-theme hicolor-icon-theme && test -f /usr/share/icons/Adwaita/index.theme && echo 'icon themes OK (Adwaita present; settings.ini owned by dms-theme-sync)'"
		// bluetooth_service: DMS BT widget speaks org.bluez dbus — the stack
		// must be alive. Name UNIFIED at "bluetooth" on both inits (feralos
		// bluez-dinit r2 ships /lib/dinit.d/bluetooth). Three assertions:
		// started marker (init-specific) + bluetoothd process + org.bluez
		// actually REGISTERED on the system bus (what the DMS widget talks
		// to). No BT adapter exists in QEMU — org.bluez registers anyway,
		// adapter presence is real-hardware territory.
		if cfg.EnableDinit {
			checks["bluetooth_service"] = "dinitctl list | grep -E '\\[[^]]*\\] *bluetooth( |$)' && pgrep bluetoothd >/dev/null && dbus-send --system --print-reply --dest=org.freedesktop.DBus / org.freedesktop.DBus.ListNames 2>/dev/null | grep -q org.bluez && echo 'bluetooth stack OK (started, bluetoothd, org.bluez on bus)' || { echo 'bluetooth stack BROKEN'; exit 1; }"
		} else {
			checks["bluetooth_service"] = "rc-status --all 2>/dev/null | grep -E 'bluetooth[^[]*\\[ *started' && pgrep bluetoothd >/dev/null && dbus-send --system --print-reply --dest=org.freedesktop.DBus / org.freedesktop.DBus.ListNames 2>/dev/null | grep -q org.bluez && echo 'bluetooth stack OK (started, bluetoothd, org.bluez on bus)' || { echo 'bluetooth stack BROKEN'; exit 1; }"
		}
		// nm_wifi_support: DMS network widget gets wifi from NetworkManager —
		// Alpine splits the wifi PLUGIN subpackage (networkmanager-wifi), so
		// its presence is the actual discriminator (NM without it has NO
		// wifi support at all). Init-agnostic (NM is a daemon either way):
		// plugin + regdb + wpa_supplicant + regulatory.db + `nmcli radio
		// wifi` answers (proves the NM daemon + radio subsystem respond).
		// Real wifi runtime (scan/connect) needs real hardware.
		checks["nm_wifi_support"] = "apk info -e networkmanager-wifi wireless-regdb wpa_supplicant && test -s /lib/firmware/regulatory.db && test -x /sbin/wpa_supplicant && r=$(nmcli radio wifi 2>&1) && echo \"nm wifi OK (radio: $r)\" || { echo \"nm wifi BROKEN (radio: ${r:-n/a})\"; exit 1; }"
		// console_keymap: init-conditional — the layout is loaded at boot in
		// BOTH init systems, but the started-marker source differs.
		// dinit (busybox-loadkmap-dinit): conf must point at a kbd-bkeymaps
		// path AND the service must hold a started marker in dinitctl list
		// (boot-time scripted service, no polling needed). dinitctl list
		// marker: explicit start = [[+]], dependency start = [{+}] (3 chars)
		// — [^]]* matches both bracket styles.
		// OpenRC (busybox-initscripts loadkmap): same conf check + started
		// marker in rc-status --all (format: `loadkmap [ started ]`) — the
		// dinitctl call would fail with "No such file or directory" on the
		// /run/dinitctl socket (btrfs_standard_niri fixture runs OpenRC).
		if cfg.EnableDinit {
			checks["console_keymap"] = "grep -q 'bkeymaps' /etc/conf.d/loadkmap && dinitctl list | grep -E '\\[[^]]*\\] *loadkmap' && echo 'console keymap loaded'"
		} else {
			checks["console_keymap"] = "grep -q 'bkeymaps' /etc/conf.d/loadkmap && rc-status --all 2>/dev/null | grep -E 'loadkmap[^[]*\\[ *started' && echo 'console keymap loaded (openrc)'"
		}
		// niri session wiring: FeralOS niri-session (niri-user-dinit, no
		// shells/re-exec traps), login-shell trick (passwd shell listed in
		// /etc/shells — pkexec requires it), turnstile PAM → user dinit,
		// boot.d enables.
		checks["niri_user_service"] = "test -f /etc/dinit.d/user/niri && grep -q 'niri --session' /etc/dinit.d/user/niri"
		checks["login_shell_trick"] = fmt.Sprintf("getent passwd %s | cut -d: -f7 | grep -q feralos/login-shell && test -L /usr/libexec/feralos/login-shell && grep -qxF /usr/libexec/feralos/login-shell /etc/shells", cfg.InstalledUser)
		// turnstile_pkg: dual-init session launcher — files on BOTH inits;
		// the service must actually RUN (dinit: boot.d link + started
		// marker; OpenRC: init.d file + rc-status started + process).
		// GOTCHA (live-caught): busybox `pgrep -x turnstiled` matches the
		// WHOLE argv[0] — the daemon runs as /usr/bin/turnstiled — so -x can
		// never match; use -f against the binary path.
		if cfg.EnableDinit {
			checks["turnstile_pkg"] = "test -f /etc/dinit.d/turnstiled && test -e /etc/dinit.d/boot.d/turnstiled && test -f /usr/lib/security/pam_turnstile.so && dinitctl list | grep -E '\\[[^]]*\\] *turnstiled' && echo 'turnstile OK (dinit)'"
		} else {
			checks["turnstile_pkg"] = "test -f /etc/dinit.d/turnstiled && test -f /etc/init.d/turnstiled && test -f /usr/lib/security/pam_turnstile.so && rc-status --all 2>/dev/null | grep -E 'turnstiled[^[]*\\[ *started' && pgrep -f '/usr/bin/turnstiled' >/dev/null && echo 'turnstile OK (openrc)'"
		}
		// openrc_fallback_ready (dinit installs): OpenRC enablement is never
		// conditional — a dinit install must stay bootable under OpenRC. The
		// rc-update links + installer-written OpenRC services must exist
		// even while dinit is PID 1 (inert until the fallback boot).
		if cfg.EnableDinit {
			checks["openrc_fallback_ready"] = "test -L /etc/runlevels/default/turnstiled && test -L /etc/runlevels/default/greetd && test -L /etc/runlevels/boot/vector-setup && test -L /etc/runlevels/boot/klogd && test -x /etc/init.d/vector-setup && test -x /etc/init.d/turnstiled && apk info -e klogd-openrc >/dev/null && echo 'OpenRC fallback ready (runlevels + services)'"
		}
		// user_session_env: the Chimera turnstile env chain, proven
		// END-TO-END. /etc/environment -> pam_env (session optional
		// pam_env.so in /etc/pam.d/turnstiled, appended by the aports
		// turnstile package) -> turnstiled pam_getenvlist -> user dinit
		// spawn env. pipewire runs ONLY as a user-dinit service, so its
		// /proc environ is direct proof the seed reaches user services.
		// Asserts BOTH seed tails: flatpak exports (original) and
		// .nix-profile/share (nix XDG gap fix). Init-agnostic: turnstile
		// spawns the same user dinit under OpenRC too.
		checks["user_session_env"] = fmt.Sprintf("pid=$(pgrep -u %s -x pipewire | head -n1); [ -n \"$pid\" ] || { echo 'no pipewire process (user dinit down?)'; exit 1; }; tr '\\0' '\\n' < /proc/$pid/environ | grep -q '^XDG_DATA_DIRS=.*flatpak/exports' || { echo 'XDG_DATA_DIRS missing flatpak exports in user service env'; exit 1; }; tr '\\0' '\\n' < /proc/$pid/environ | grep -q '^XDG_DATA_DIRS=.*\\.nix-profile/share' || { echo 'XDG_DATA_DIRS missing nix profile in user service env'; exit 1; }; echo 'session env OK (pam_env -> user dinit chain delivers XDG_DATA_DIRS)'", cfg.InstalledUser)
		// gnome-keyring: package + D-Bus secrets service + PAM unlock lines
		// in greetd. NIRI-ONLY: niriWayle has NO gnome-keyring (oo7 handles
		// secrets there).
		if cfg.DesktopEnvironment == desktopNiri {
			checks["gnome_keyring"] = "apk info -e gnome-keyring && test -f /usr/share/dbus-1/services/org.freedesktop.secrets.service && grep -q pam_gnome_keyring /etc/pam.d/greetd"
		}
		// Runtime checks — the verification boot runs the REAL user session:
		// the harness injected a test-only greetd [initial_session]
		// (enableNiriTestAutologin, VM discarded after run) because nobody
		// logs in manually. The session comes up late (greetd + turnstile +
		// user dinit), so every check polls like niri_greeter (90s).
		checks["niri_session_user"] = fmt.Sprintf("i=0; while [ $i -lt 90 ]; do pgrep -u %s -x niri >/dev/null && { echo 'niri user session running'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'no niri session for user'; exit 1", cfg.InstalledUser)
		checks["niri_wayland_socket"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 90 ]; do ls /run/user/$uid/wayland-1 >/dev/null 2>&1 && { echo 'wayland-1 socket OK'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'no wayland socket'; exit 1", cfg.InstalledUser)
		// IPC proves the compositor actually responds — the socket must
		// exist AND `niri msg outputs` must answer. NIRI_SOCKET alone may
		// not be enough: XDG_RUNTIME_DIR + WAYLAND_DISPLAY passed too.
		checks["niri_ipc"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 90 ]; do s=$(ls /run/user/$uid/niri*.sock 2>/dev/null | head -1); if [ -n \"$s\" ]; then if XDG_RUNTIME_DIR=/run/user/$uid WAYLAND_DISPLAY=wayland-1 NIRI_SOCKET=\"$s\" niri msg outputs >/dev/null 2>&1; then echo 'niri IPC OK'; exit 0; fi; fi; sleep 1; i=$((i+1)); done; echo 'niri IPC failed'; exit 1", cfg.InstalledUser)
		// niri_msg_version: sharper IPC+binary proof — `niri msg version`
		// answers only if the IPC socket is live AND the binary speaks the
		// protocol; first 2 lines of output become the check detail.
		checks["niri_msg_version"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 90 ]; do s=$(ls /run/user/$uid/niri*.sock 2>/dev/null | head -1); if [ -n \"$s\" ]; then out=$(NIRI_SOCKET=\"$s\" XDG_RUNTIME_DIR=/run/user/$uid niri msg version 2>&1); if [ $? -eq 0 ]; then echo \"$out\" | head -2; exit 0; fi; fi; sleep 1; i=$((i+1)); done; echo 'niri msg version failed'; exit 1", cfg.InstalledUser)
		// niri_windows: compositor state query — rc 0 proves the IPC layer
		// answers a state query; the window list may legitimately be tiny,
		// so pass on rc 0 and show the first 5 lines only.
		checks["niri_windows"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 90 ]; do s=$(ls /run/user/$uid/niri*.sock 2>/dev/null | head -1); if [ -n \"$s\" ]; then out=$(NIRI_SOCKET=\"$s\" XDG_RUNTIME_DIR=/run/user/$uid niri msg windows 2>&1); if [ $? -eq 0 ]; then echo \"$out\" | head -5; exit 0; fi; fi; sleep 1; i=$((i+1)); done; echo 'niri windows query failed'; exit 1", cfg.InstalledUser)
		// niri_outputs_count — real check, >=1 output expected on EVERY host.
		// The old QEMU skip is obsolete: the GPU chain renders (virtio-vga-gl
		// + egl-headless,gl=on + guest mesa edge 26.2.4 with virgl restored +
		// host virglrenderer 1.3.0 + kernel >=6.13) — niri holds Virtual-1
		// 1280x800 in QEMU too.
		checks["niri_outputs_count"] = fmt.Sprintf("uid=$(id -u %s); s=$(ls /run/user/$uid/niri*.sock 2>/dev/null | head -1); out=$(NIRI_SOCKET=\"$s\" XDG_RUNTIME_DIR=/run/user/$uid niri msg outputs 2>&1); if [ $? -ne 0 ]; then echo 'niri msg outputs failed'; exit 1; fi; n=$(echo \"$out\" | grep -c '^Output'); if [ \"$n\" -ge 1 ]; then echo \"outputs: $n\"; exit 0; fi; echo 'no outputs'; exit 1", cfg.InstalledUser)
		// dms_ipc (NIRI-ONLY: niriWayle has no DMS): DMS IPC liveness
		// through the real CLI. Syntax VERIFIED
		// empirically on the LIVE target of run 2026-10-05T00-42-34Z:
		// `dms ipc call outputs status` answers `auto: off / active: none /
		// outputs: []` rc 0 (the empty list is the QEMU zero-output
		// reality — the call still proves IPC responsive). GOTCHA 1: the
		// dms binary REFUSES to run as root (`FATAL go: This program should
		// not be run as root. Exiting.`), so the probe su's to the
		// installed user (-s /bin/sh: user login shell is fish). GOTCHA 2:
		// `dms ipc call widget list` ALSO returned rc 0 on the same box
		// despite `widget` not existing in the target list — DMS answers
		// unknown targets rc 0, so only a REAL read-only target/function
		// pair is valid proof; `outputs status` is one (help-verified
		// target list: outputs → current/listProfiles/refresh/.../status).
		// The client locates the running shell's socket via
		// XDG_RUNTIME_DIR (DMS_SOCKET may override); NIRI_SOCKET exported
		// for symmetry with the niri checks.
		if cfg.DesktopEnvironment == desktopNiri {
			checks["dms_ipc"] = fmt.Sprintf("i=0; while [ $i -lt 60 ]; do su - %s -s /bin/sh -c 'export XDG_RUNTIME_DIR=/run/user/$(id -u); NIRI_SOCKET=$(ls /run/user/$(id -u)/niri*.sock 2>/dev/null | head -1); timeout 10 dms ipc call outputs status' >/dev/null 2>&1 && { echo 'dms IPC OK'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'dms IPC failed'; exit 1", cfg.InstalledUser)
		}
		// dms_runtime (NIRI-ONLY): process-level check (deeper than the
		// dms.user service-list entry above). dms-session wrapper execs
		// `dms run`; quickshell is the actual shell process — match both;
		// adjust the pattern after run 1 if the cmdline differs.
		if cfg.DesktopEnvironment == desktopNiri {
			checks["dms_runtime"] = fmt.Sprintf("i=0; while [ $i -lt 90 ]; do pgrep -u %s -f 'dms run|quickshell' >/dev/null && { echo 'dms shell running'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'dms not running'; exit 1", cfg.InstalledUser)
		}
		// niri_layers: layer-shell surfaces MAPPED to an output — the
		// "it actually displays" proof. DMS's bar is a layer-shell
		// client (quickshell); a session with DMS running always maps
		// >=1 layer surface, so `niri msg layers` must list an Output
		// heading. Deeper than dms_runtime (process alive) and
		// dms_ipc (IPC answers): proves the shell RENDERS.
		// VERIFY the exact `niri msg layers` output shape on run 1.
		checks["niri_layers"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 90 ]; do s=$(ls /run/user/$uid/niri*.sock 2>/dev/null | head -1); if [ -n \"$s\" ]; then out=$(NIRI_SOCKET=\"$s\" XDG_RUNTIME_DIR=/run/user/$uid niri msg layers 2>&1); if [ $? -eq 0 ]; then n=$(echo \"$out\" | grep -c '^Output'); if [ \"$n\" -ge 1 ]; then echo \"layer surfaces mapped on $n output(s)\"; echo \"$out\" | head -5; exit 0; fi; fi; fi; sleep 1; i=$((i+1)); done; echo 'no layer surfaces (DMS bar not rendered?)'; exit 1", cfg.InstalledUser)
		// Session runtime sockets — the services being "started" in
		// niri_user_dinit_runtime is necessary but not sufficient; the
		// consumer endpoints must EXIST:
		//   session_dbus_socket   — /run/user/$uid/bus (session bus)
		//   session_pipewire_socket — /run/user/$uid/pipewire-0 (audio)
		checks["session_dbus_socket"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 60 ]; do test -S /run/user/$uid/bus && { echo 'session dbus socket OK'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'no session dbus socket'; exit 1", cfg.InstalledUser)
		checks["session_pipewire_socket"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 60 ]; do test -S /run/user/$uid/pipewire-0 && { echo 'pipewire socket OK'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'no pipewire socket'; exit 1", cfg.InstalledUser)
		// GPU userspace drivers (videoDriver: virtio) — packages must be
		// installed AND the gallium megadriver must exist under /usr/lib/dri
		// (virtio_gpu_dri.so; verify the exact filename on run 1 and adjust
		// the ls grep if needed — apk info stays strict).
		if cfg.VideoDriver == "virtio" {
			checks["video_driver"] = "apk info -e mesa-dri-gallium mesa-vulkan-virtio && ls /usr/lib/dri/ | grep -q virtio && echo 'virtio GPU userspace OK' || { echo 'virtio GPU userspace missing'; exit 1; }"
		}
		// wayvnc listens on :5900 — real check on every host: the GPU chain
		// renders (virtio-vga-gl + egl-headless,gl=on + guest mesa edge
		// 26.2.4 virgl restored), wayvnc holds an output and keeps the
		// listener up (no more exit-1 smooth-recovery loop).
		checks["niri_wayvnc_port"] = "i=0; while [ $i -lt 90 ]; do netstat -tln 2>/dev/null | grep -q ':5900' && { echo 'wayvnc listening on 5900'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'wayvnc not listening'; exit 1"
		// Real-rendering proof: ask the compositor for a screenshot and
		// require a non-empty PNG. niri takes NO path argument on
		// screenshot-screen (rejects it: "unexpected argument") and writes
		// to its default ~/Pictures/Screenshots — the dir the autologin
		// setup below pre-creates with correct ownership (silent no-write
		// otherwise). Real check on every host: the GPU chain renders
		// (virtio-vga-gl + egl-headless,gl=on + guest mesa edge 26.2.4
		// virgl restored), so the capture writes a fresh non-empty PNG.
		// Compare PNG COUNTS before/after the action (not ls -t newest):
		// the action returns rc 0 BEFORE the file write lands, so the
		// immediate ls -t keeps seeing the previous shot (n == f race,
		// lost 90/90 on the live target 2026-10-09). 1s settle after the
		// action lets the write land.
		checks["niri_screenshot"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 60 ]; do s=$(ls /run/user/$uid/niri*.sock 2>/dev/null | head -1); if [ -n \"$s\" ]; then b=$(ls /home/%s/Pictures/Screenshots/*.png 2>/dev/null | wc -l); XDG_RUNTIME_DIR=/run/user/$uid WAYLAND_DISPLAY=wayland-1 NIRI_SOCKET=\"$s\" niri msg action screenshot-screen >/dev/null 2>&1; sleep 1; a=$(ls /home/%s/Pictures/Screenshots/*.png 2>/dev/null | wc -l); if [ \"$a\" -gt \"$b\" ]; then n=$(ls -t /home/%s/Pictures/Screenshots/*.png 2>/dev/null | head -1); if ! head -c 8 \"$n\" | od -An -tx1 | grep -q '89 50 4e 47 0d 0a 1a 0a'; then echo \"screenshot NOT a valid PNG: $n\"; exit 1; fi; echo \"screenshot OK (valid PNG): $n ($(wc -c < \"$n\") bytes)\"; exit 0; fi; fi; sleep 1; i=$((i+1)); 		done; echo 'screenshot failed'; exit 1", cfg.InstalledUser, cfg.InstalledUser, cfg.InstalledUser, cfg.InstalledUser)
	}
	// niriWayle-only checks — Wayle shell, Veila lock daemon, Raffi
	// launcher, oo7 secrets. niri has no counterpart here (and vice versa:
	// the DMS checks above are niri-only).
	if cfg.DesktopEnvironment == desktopNiriWayle {
		// wayle_running: the Wayle shell process (bar/notifications/OSD/
		// wallpaper) alive for the session user. Poll 90s like the other
		// session checks — the shell starts after greetd + turnstile +
		// user dinit + niri.
		checks["wayle_running"] = fmt.Sprintf("i=0; while [ $i -lt 90 ]; do pgrep -u %s -f '/usr/bin/wayle' >/dev/null && { echo 'wayle shell running'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'wayle not running'; exit 1", cfg.InstalledUser)
		// veila_daemon: the Veila lock-screen daemon behind `veila lock`
		// (stasis idle plan + Super+Alt+L keybind + dashboard dropdown).
		// Runs through the veilad-session env wrapper (WAYLAND_DISPLAY/
		// NIRI_SOCKET resolution — the user-dinit env carries neither;
		// upstream's veilad.service PassEnvironment's the same variables).
		checks["veila_daemon"] = fmt.Sprintf("test -f /etc/dinit.d/user/scripts/veilad-session && grep -q 'command = /etc/dinit.d/user/scripts/veilad-session' /etc/dinit.d/user/veila.user && grep -q 'exec veilad' /etc/dinit.d/user/scripts/veilad-session && i=0; while [ $i -lt 90 ]; do pgrep -u %s -f 'veilad' >/dev/null && { echo 'veila daemon running (veilad-session wrapper)'; exit 0; }; sleep 1; i=$((i+1)); done; echo 'veilad not running'; exit 1", cfg.InstalledUser)
		// raffi_installed: launcher package + binary on PATH + the toggle
		// script (feralos-aports raffi-toggle; raffi r1 depends on it —
		// Mod+D and the Wayle bar button both toggle, they never spawn
		// raffi directly).
		checks["raffi_installed"] = "apk info -e raffi raffi-toggle && test -x /usr/bin/raffi && test -x /usr/bin/raffi-toggle && echo 'raffi + raffi-toggle installed'"
		// oo7_secrets: gnome-keyring replacement — suite core + PAM module
		// packages present (the unlock wiring has its own runtime check
		// below).
		checks["oo7_secrets"] = "apk info -e oo7 oo7-pam && echo 'oo7 secrets installed'"
		// keyrack_flatpak: Key Rack (app.drey.KeyRack) is the niriWayle
		// secrets GUI — Flathub flatpak, NOT an apk (seahorse conflicts
		// with oo7-server on org.freedesktop.secrets.service; nixpkgs too
		// old at 0.4.0). Asserts the system flatpak install AND the
		// Secret portal pin (oo7 README config: niriWayle has no
		// gnome-keyring, so Secret must resolve to oo7-portal explicitly).
		// No DMI guard: flatpak + the portals conf are real on every host.
		checks["keyrack_flatpak"] = fmt.Sprintf("flatpak info app.drey.KeyRack >/dev/null 2>&1 && grep -q 'org.freedesktop.impl.portal.Secret=oo7-portal' /home/%s/.config/xdg-desktop-portal/niri-portals.conf && echo 'Key Rack flatpak installed + Secret=oo7-portal pinned'", cfg.InstalledUser)
		// oo7_pam_unlock: full login-unlock parity proof for the oo7 stack.
		// STATIC half registered here; the RUNTIME half (authenticated PAM
		// login on service "greetd" + secrets.log proof) is driven by
		// verifyOO7PamUnlock (oo7_pam_unlock.go) — greetd 0.10.3 never
		// authenticates the autologin initial_session, so a plain
		// autologin boot can NEVER produce an unlock request (run
		// 2026-10-17T17-29-10Z + live debug proved it). The login runs
		// TARGET-side (python3+ctypes libpam over plain exec SSH): host
		// ssh -L is denied by Alpine's stock `AllowTcpForwarding no` and a
		// greetd-IPC second start_session kills the one-session greetd
		// autologin (turns the whole user session down — runs
		// 2026-10-18T00-52-02Z / 01-49-53Z). Direct PAM chain leaves the
		// live session + daemon untouched: no race, no collateral fails.
		// Statics:
		//   - pam_oo7.so on disk, the greetd auth+session lines the
		//     installer appends, the /etc/pam.d/passwd override (password
		//     re-sync stack)
		//   - /usr/bin/oo7-daemon recovery wrapper delegating to
		//     dinitctl --user (pam_oo7 auto_start fallback; the daemon may
		//     only ever start as a supervised user-dinit service)
		//   - /etc/dinit.d/config/secrets.conf (+ RUST_LOG=info when
		//     enableDebug — the INFO line the runtime proof greps)
		//   - the daemon's live PAM listener socket
		//   - python3 present (remote PAM login client)
		checks["oo7_pam_unlock"] = fmt.Sprintf("uid=$(id -u %s); i=0; while [ $i -lt 30 ]; do test -S /run/user/$uid/oo7-pam.sock && break; sleep 1; i=$((i+1)); done; test -f /usr/lib/security/pam_oo7.so || { echo 'pam_oo7.so missing'; exit 1; }; grep -q 'auth.*pam_oo7.so' /etc/pam.d/greetd || { echo 'greetd auth line missing'; exit 1; }; grep -q 'session.*pam_oo7.so' /etc/pam.d/greetd || { echo 'greetd session line missing'; exit 1; }; grep -q 'password.*pam_oo7.so' /etc/pam.d/passwd || { echo 'passwd override missing'; exit 1; }; test -f /usr/bin/oo7-daemon && test -x /usr/bin/oo7-daemon || { echo 'oo7-daemon recovery wrapper missing/not executable'; exit 1; }; grep -q 'dinitctl --user start' /usr/bin/oo7-daemon || { echo 'recovery wrapper does not delegate to dinitctl --user start (unsupervised daemon risk)'; exit 1; }; test -f /etc/dinit.d/config/secrets.conf || { echo 'secrets.conf missing'; exit 1; }; %s test -S /run/user/$uid/oo7-pam.sock || { echo 'oo7 PAM listener socket missing'; exit 1; }; command -v python3 >/dev/null || { echo 'python3 missing (remote PAM login client needs it)'; exit 1; }; echo 'oo7 PAM static wiring OK'", cfg.InstalledUser, debugRUSTLogAssert(cfg))
		// wayle_session_actions: the Wayle session-action seed (installer
		// step 7b). v0.7.0 source-verified: the dashboard dropdown's
		// user-row buttons run `sh -c <dashboard.dropdown_*_command>` —
		// upstream defaults are systemd-only and dead on FeralOS
		// (`systemctl reboot|poweroff`; `loginctl lock-session` a no-op on
		// niri), and the default bar layout has no session entry point.
		// Static assert of the seed: 4 dropdown commands + the Windows-
		// style bottom bar (launcher+window-title left, dashboard far
		// right, icon-only status, everforest palette). The reboot
		// command runs the dinit wrapper via zz-power (asserted by
		// dinit_shutdown_commands — same rule/rationale as niri/DMS).
		// Init-aware: OpenRC fixtures expect `loginctl reboot`.
		wayleRebootCmd := "loginctl reboot"
		if cfg.EnableDinit {
			wayleRebootCmd = "sudo -n /usr/bin/reboot"
		}
		checks["wayle_session_actions"] = fmt.Sprintf("f=/home/%s/.config/wayle/config.toml; test -f \"$f\" || { echo 'wayle config.toml missing'; exit 1; }; grep -q 'dropdown-lock-command = \"veila lock\"' \"$f\" || { echo 'lock command missing'; exit 1; }; grep -q 'dropdown-logout-command = \"niri msg action quit --skip-confirmation\"' \"$f\" || { echo 'logout command missing'; exit 1; }; grep -q 'dropdown-reboot-command = %q' \"$f\" || { echo 'reboot command missing'; exit 1; }; grep -q 'dropdown-poweroff-command = \"loginctl poweroff\"' \"$f\" || { echo 'poweroff command missing'; exit 1; }; grep -q 'location = \"bottom\"' \"$f\" || { echo 'bar not at bottom'; exit 1; }; grep -qF 'left = [\"custom-launcher\", \"window-title\"]' \"$f\" || { echo 'launcher+window-title layout missing'; exit 1; }; grep -qF 'right = [\"systray\", \"battery\", \"bluetooth\", \"network\", \"microphone\", \"volume\", \"clock\", \"dashboard\"]' \"$f\" || { echo 'right layout mismatch'; exit 1; }; grep -q 'label-show = false' \"$f\" || { echo 'icon-only status modules missing'; exit 1; }; grep -q 'primary = \"#7fbbb3\"' \"$f\" || { echo 'everforest palette missing'; exit 1; }; grep -q 'left-click = \"/usr/bin/raffi-toggle\"' \"$f\" || { echo 'launcher toggle not wired'; exit 1; }; echo 'wayle session actions OK (Windows-style bottom bar + dashboard dropdown + raffi-toggle launcher)'", cfg.InstalledUser, wayleRebootCmd)
		// stasis_idle: the stasis context-aware idle manager (feralos-aports
		// 1.6.3, dependency of wayle-session) — package + supervised
		// session service stasis.user (started by the wayle-session script
		// after niri, existence-filtered). STATIC: the FeralOS system
		// config /etc/stasis/stasis.rune (veila lock + loginctl suspend)
		// and the `-c /etc/stasis/stasis.rune` startup in the
		// stasis-session wrapper (without `-c` the daemon would bootstrap a
		// ~/.config template with swaylock/systemctl — absent on FeralOS —
		// and prefer it over the system file; the ABSENCE of that user file
		// proves `-c` was honored). RUNTIME: started marker in the user
		// dinit + `stasis info` as the session user (IPC round trip against
		// the live daemon — only possible with a loaded config).
		checks["stasis_idle"] = fmt.Sprintf("apk info -e stasis >/dev/null && test -f /etc/dinit.d/user/stasis.user || { echo 'stasis package or stasis.user service missing'; exit 1; }; test -f /etc/stasis/stasis.rune || { echo '/etc/stasis/stasis.rune missing'; exit 1; }; grep -q 'command \"veila lock\"' /etc/stasis/stasis.rune || { echo 'veila lock missing from stasis config'; exit 1; }; grep -q 'loginctl suspend' /etc/stasis/stasis.rune || { echo 'loginctl suspend missing from stasis config'; exit 1; }; grep -q -- '-c /etc/stasis/stasis.rune' /etc/dinit.d/user/scripts/stasis-session || { echo 'stasis-session does not use -c /etc/stasis/stasis.rune'; exit 1; }; test ! -f /home/%s/.config/stasis/stasis.rune || { echo 'user stasis config exists (bootstrap ran?)'; exit 1; }; uid=$(id -u %s); export XDG_RUNTIME_DIR=/run/user/$uid; i=0; ok=0; while [ $i -lt 90 ]; do out=$(dinitctl --user list 2>/dev/null) || { sleep 1; i=$((i+1)); continue; }; echo \"$out\" | grep -Eq '(\\{\\+|\\[\\+).*\\] stasis\\.user( |$)' && { ok=1; break; }; sleep 1; i=$((i+1)); done; [ \"$ok\" = 1 ] || { echo 'stasis.user not running'; exit 1; }; su - %s -s /bin/sh -c 'export XDG_RUNTIME_DIR=/run/user/$(id -u); for s in $XDG_RUNTIME_DIR/wayland-*; do [ -S \"$s\" ] && export WAYLAND_DISPLAY=\"${s##*/}\"; done; stasis info >/dev/null 2>&1' && echo 'stasis idle manager OK (system config + stasis.user started + info IPC)' || { echo 'stasis info failed'; exit 1; }", cfg.InstalledUser, cfg.InstalledUser, cfg.InstalledUser)
		// niriwayle_session_unique: ONE greeter session entry (real-hw
		// finding — regreet listed BOTH the upstream bare `niri` session
		// from the niri package and the FeralOS niriWayle one from
		// wayle-session). ReGreet 0.2.0 respects the Desktop Entry
		// Hidden/NoDisplay fields (src/sysutil.rs); the installer appends
		// NoDisplay=true to niri.desktop on niriWayle installs. Static
		// asserts: the niriwayle session file exists, the bare niri session
		// is NoDisplay-hidden, the niriwayle Name= line is intact. (No
		// inverse assert needed on the niri DE — there the bare niri session
		// IS the session.) No DMI guard: the desktop files are static
		// on-disk state on every host.
		checks["niriwayle_session_unique"] = "test -f /usr/share/wayland-sessions/niriwayle.desktop || { echo 'niriwayle.desktop missing'; exit 1; }; grep -q '^NoDisplay=true' /usr/share/wayland-sessions/niri.desktop || { echo 'bare niri session not hidden (NoDisplay=true missing from niri.desktop)'; exit 1; }; grep -q '^Name=niriWayle' /usr/share/wayland-sessions/niriwayle.desktop || { echo 'niriwayle.desktop Name= line missing'; exit 1; }; echo 'niriWayle session unique OK (niriwayle.desktop + bare niri NoDisplay-hidden)'"
	}
}

// buildSessionChecks — the per-DE user-session service list checks
// (data from deSessionStacks, de_data.go) and the kde/cosmic session
// runtime payloads (one parameterized builder).
func buildSessionChecks(cfg *VerifyConfig, checks map[string]string) {
	if isNiriFamilyDE(cfg.DesktopEnvironment) {
		stack := sessionStackForDE(cfg.DesktopEnvironment)
		// user dinit services: supervised session stack (dbus/audio/wayland
		// gate/polkit-agent + dms OR wayle/veila) shipped by *-user-dinit
		// packages (per-DE list, de_data.go)
		checks["dinit_user_services"] = fmt.Sprintf("for s in %s; do test -f /etc/dinit.d/user/$s || { echo \"missing $s\"; exit 1; }; done; echo 'user services OK'", stack.dinitUserSvcFiles)
		// user_services_enabled: infrastructure services only in the
		// system-wide boot.d. Session apps (per-DE absent list: niri →
		// polkit-agent/dms/wayvnc; niriWayle → wayle/veila/polkit-agent)
		// are bound to the session (started by the session script, upstream
		// "add-wants niri.service" model) — they must NOT be in boot.d,
		// or every login (greetd greeter included) would start them.
		checks["user_services_enabled"] = fmt.Sprintf("for s in %s; do test -e /usr/lib/dinit.d/user/boot.d/$s || { echo \"missing $s\"; exit 1; }; done; for s in %s; do test -e /usr/lib/dinit.d/user/boot.d/$s && { echo \"session app $s must not be in boot.d\"; exit 1; }; done; echo ok", stack.userSvcBootLinks, stack.sessionAppsAbsent)
		// User dinit session stack — BOTH inits: turnstile spawns the
		// per-user dinit instance under OpenRC too (turnstiled runs as an
		// OpenRC system service from the turnstile-openrc subpackage; the
		// user backend is the same dinit). Every boot.d user service must
		// show the started marker. dinitctl list renders started services
		// as `[{+}] name (pid: ...)` — '{+' appears only when running. ERE
		// anchors the name so '] niri' cannot match '] niri-shutdown'.
		// dinitctl list marks DEPENDENCY-started services `[{+}]` and
		// EXPLICITLY-started ones `[[+]]`. Accept both bracket styles;
		// both mean running. Service list is PER-DE (niri: dms.user;
		// niriWayle: wayle.user + veila.user + stasis.user). wayvnc.user stays REQUIRED
		// on every host: the GPU chain renders (virtio-vga-gl +
		// egl-headless,gl=on + guest mesa edge 26.2.4 virgl restored),
		// wayvnc holds an output and stays up.
		checks["niri_user_dinit_runtime"] = fmt.Sprintf("uid=$(id -u %s); export XDG_RUNTIME_DIR=/run/user/$uid; i=0; while [ $i -lt 90 ]; do out=$(dinitctl --user list 2>/dev/null) || { sleep 1; i=$((i+1)); continue; }; missing=''; svcs='%s'; for s in $svcs; do echo \"$out\" | grep -Eq '(\\{\\+|\\[\\+).*\\] '\"$s\"'( |$)' || missing=\"$missing $s\"; done; if [ -z \"$missing\" ]; then echo 'user dinit stack running'; exit 0; fi; sleep 1; i=$((i+1)); done; echo \"user services not running:$missing\"; exit 1", cfg.InstalledUser, stack.userDinitRuntimeSvcs)
	}
	// DE session runtime checks (test-only autologin): the session must
	// actually come up on the verification boot. Poll ≤90 s like the niri
	// runtime checks; falsifiable (exit 1 on failure).
	//   kde:    sddm autologin → plasma.desktop → startplasma-wayland
	//           (classic boot, no systemd) → kwin_wayland + plasmashell
	//   cosmic: greetd initial_session → start-cosmic → cosmic-session
	//           → cosmic-comp (+ panel/applets); XDG_RUNTIME_DIR via
	//           pam_rundir (base-session; cosmic uses seatd, not elogind)
	if probe, ok := deSessionProbes[cfg.DesktopEnvironment]; ok {
		checks[probe.checkName] = buildDESessionCheck(cfg, &probe)
	}
}

// buildDBusActivationChecks — ADR-010 D-Bus activation coverage:
// dbus_system_activation (every dinit desktop), the niri/kde/cosmic
// uniform+env+migration trio and the niri-family live-activation proof.
func buildDBusActivationChecks(cfg *VerifyConfig, checks map[string]string) {
	isNiriFamily := isNiriFamilyDE(cfg.DesktopEnvironment)
	// dbus_system_activation: system-bus ADR-010 coverage for EVERY desktop.
	// Every upstream system activation file must resolve to a FeralOS override
	// source (shim Exec), a package-shipped dinit unit and the boot-materialized
	// copy in /usr/local/share/dbus-1/system-services. Runs on dinit only —
	// system-bus activation is helper-driven and needs the system dinit (OpenRC
	// keeps the raw fallback). The niri-family `dbus_activation_uniform` check
	// still covers the session stack + the /etc-clean invariant.
	if cfg.EnableDinit {
		checks["dbus_system_activation"] = `
fail=0
for f in /usr/share/dbus-1/system-services/*.service; do
    [ -e "$f" ] || continue
    name=$(sed -n 's/^Name=//p' "$f" | head -1)
    [ -n "$name" ] || { echo "no Name= in $f"; fail=1; continue; }
    src=/usr/share/feralos/dbus-system-services/$name.service
    [ -f "$src" ] || { echo "missing system override source: $name"; fail=1; continue; }
    grep -q '^Exec=/usr/libexec/feralos/dbus-activate ' "$src" || { echo "system override not shim: $name"; fail=1; continue; }
    unit=$(sed -n 's/^Exec=.*dbus-activate \([^ ]*\) .*/\1/p' "$src" | head -1)
    [ -f "/lib/dinit.d/$unit" ] || { echo "system unit missing: $unit ($name)"; fail=1; continue; }
    eff=/usr/local/share/dbus-1/system-services/$name.service
    [ -f "$eff" ] || { echo "system override not materialized into /usr/local/share: $name"; fail=1; continue; }
    grep -q '^Exec=/usr/libexec/feralos/dbus-activate ' "$eff" || { echo "materialized system override not shim: $name"; fail=1; continue; }
done
for f in /etc/dbus-1/system-services/*.service; do
    [ -e "$f" ] || continue
    echo "override in helper-ineligible dir (enumerated but not activatable): $f"; fail=1
done
[ "$fail" = 0 ] && echo 'system-bus activation coverage OK (every activation file routes through the dinit shim)' || exit 1
`
	}
	// ADR-010 activation coverage (uniform + migration): enforced on EVERY
	// desktop — kde/cosmic sessions now materialize the session overrides at
	// login via dbus-session-materializer.user (plasma/cosmic dbus-run-session
	// read the same XDG_RUNTIME_DIR servicedir the wrapper writes to).
	if isNiriFamily || cfg.DesktopEnvironment == desktopKDE || cfg.DesktopEnvironment == desktopCosmic {
		// dbus_activation_uniform: ADR-010 enforcement — every INSTALLED
		// system-bus override must route through the uniform shim, its target
		// unit must exist, and it must be materialized into the launch
		// helper's first eligible dir; on dinit installs EVERY upstream
		// activation file must have an override (no exceptions — the -openrc
		// packages are converted to their -dinit equivalents there). OpenRC
		// installs use the plain packages (no -dinit units to route to), so
		// the coverage half applies to dinit installs only. Session
		// overrides ship in /usr/share/feralos/dbus-services (abuild forbids
		// /usr/local) and are materialized at login by the dbus-session
		// wrapper into $XDG_RUNTIME_DIR/dbus-1/services — the highest-
		// precedence servicedir (strict naming, read at bus start). System
		// overrides ship in /usr/share/feralos/dbus-system-services and are
		// materialized at boot (dbus-services-setup, ordered before dbus)
		// into /usr/local/share/dbus-1/system-services — the setuid launch
		// helper resolves activation names ONLY against the standard dirs
		// (its config parser is the trivial one; /usr/local/share is first
		// in precedence); /etc/dbus-1/system-services is daemon-enumerated
		// but NOT helper-activatable (live-verified on dbus 1.16.2). The
		// check also fails if any override is left in the helper-ineligible
		// /etc dir.
		uniformCoverage := ""
		if cfg.EnableDinit {
			uniformCoverage = `
for f in /usr/share/dbus-1/system-services/*.service; do
    [ -e "$f" ] || continue
    name=$(sed -n 's/^Name=//p' "$f" | head -1)
    [ -n "$name" ] || { echo "no Name= in $f"; fail=1; continue; }
    [ -f "/usr/share/feralos/dbus-system-services/$name.service" ] || { echo "missing system override source: $name"; fail=1; }
done
`
		}
		checks["dbus_activation_uniform"] = fmt.Sprintf(`
uid=$(id -u %s)
fail=0
for f in /usr/share/dbus-1/services/*.service; do
    [ -e "$f" ] || continue
    name=$(sed -n 's/^Name=//p' "$f" | head -1)
    [ -n "$name" ] || { echo "no Name= in $f"; fail=1; continue; }
    case "$name" in
        org.kde.kcookiejar5) echo "skip (broken upstream: kded5 binary absent on Alpine 3.24 — no unit possible)"; continue ;;
    esac
    src=/usr/share/feralos/dbus-services/$name.service
    [ -f "$src" ] || { echo "missing session override source: $name"; fail=1; continue; }
    grep -q '^Exec=/usr/libexec/feralos/dbus-activate ' "$src" || { echo "session override not shim: $name"; fail=1; continue; }
    unit=$(sed -n 's/^Exec=.*dbus-activate \([^ ]*\) .*/\1/p' "$src" | head -1)
    [ -f "/etc/dinit.d/user/$unit" ] || { echo "user unit missing: $unit ($name)"; fail=1; continue; }
    rt="/run/user/$uid/dbus-1/services/$name.service"
    [ -f "$rt" ] || { echo "session override not materialized into the runtime dir: $name"; fail=1; continue; }
done
for f in /usr/share/feralos/dbus-system-services/*.service; do
    [ -e "$f" ] || continue
    name=$(sed -n 's/^Name=//p' "$f" | head -1)
    [ -n "$name" ] || { echo "no Name= in $f"; fail=1; continue; }
    grep -q '^Exec=/usr/libexec/feralos/dbus-activate ' "$f" || { echo "system override not shim: $name"; fail=1; continue; }
    unit=$(sed -n 's/^Exec=.*dbus-activate \([^ ]*\) .*/\1/p' "$f" | head -1)
    [ -f "/lib/dinit.d/$unit" ] || { echo "system unit missing: $unit ($name)"; fail=1; continue; }
    eff=/usr/local/share/dbus-1/system-services/$name.service
    [ -f "$eff" ] || { echo "system override not materialized into /usr/local/share: $name"; fail=1; continue; }
    grep -q '^Exec=/usr/libexec/feralos/dbus-activate ' "$eff" || { echo "materialized system override not shim: $name"; fail=1; continue; }
done
%sfor f in /etc/dbus-1/system-services/*.service; do
    [ -e "$f" ] || continue
    echo "override in helper-ineligible dir (enumerated but not activatable): $f"; fail=1
done
[ "$fail" = 0 ] && echo 'dbus activation uniform OK (every activation file routes through the dinit shim)' || exit 1
`, cfg.InstalledUser, uniformCoverage)
		// dbus_activation_env: REAL activation-environment forwarding proof.
		// Session: stop the dconf unit, push a marker var through the D-Bus
		// activation environment, trigger activation with a Peer.Ping, then
		// assert (a) the dinit unit is started (activation went through
		// dinit, not a raw spawn) and (b) the daemon's environ carries the
		// marker (the shim forwarded the activation env via dinitctl
		// setenv). System (dinit only, when upower is installed): REAL
		// activation must start the dinit unit with NO DBUS_STARTER_* in the
		// daemon environ (a raw helper spawn would leave them); then a
		// direct shim run with a marker proves the setenv forwarding to the
		// system dinit (UpdateActivationEnvironment is denied on the system
		// bus).
		checks["dbus_activation_env"] = fmt.Sprintf(`
su - %s -s /bin/sh -c '
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
# This check verifies the NIRI session stack (dbus.user owns the session bus
# and the materialized overrides). On DEs that own their session bus (kde/
# cosmic: dbus-run-session, address NOT /run/user/<uid>/bus) the check is
# skipped honestly — their session targets are covered by the same
# uniform/migration checks; the dbus-session-materializer serves their bus.
[ -S "$XDG_RUNTIME_DIR/bus" ] || { echo "session env check SKIPPED (DE-owned session bus)"; exit 0; }
diag() {
    echo "--- diagnostics ---"
    echo "stop: $stop_out"
    echo "envpush: $env_out"
    echo "dbus-send: $send_out"
    dinitctl --user status ca.desrt.dconf.user 2>&1 || true
    dinitctl --user list 2>/dev/null | grep -i dconf || echo "no dconf unit in list"
    pgrep -f "^/usr/libexec/dconf-service$" || echo "no dconf-service process"
    ls -l "$XDG_RUNTIME_DIR/dbus-1/services/" 2>&1 | head -5
    echo "--- direct shim attempt (bypasses dbus) ---"
    FERALOS_DBUS_ACT_TEST=session DBUS_STARTER_BUS_TYPE=session /usr/libexec/feralos/dbus-activate ca.desrt.dconf.user -- /usr/libexec/dconf-service
    echo "shim rc=$?"
    sleep 2
    dinitctl --user status ca.desrt.dconf.user 2>&1 || true
    pgrep -f "^/usr/libexec/dconf-service$" || true
    tail -20 "$HOME/.local/share/dinit/ca.desrt.dconf.log" 2>&1 || true
}
stop_out=$(dinitctl --user stop ca.desrt.dconf.user 2>&1 || true)
env_out=$(dbus-update-activation-environment --all 2>&1 || true)
env_out="$env_out$(dbus-update-activation-environment FERALOS_DBUS_ACT_TEST=session 2>&1 || true)"
send_out=$(dbus-send --session --print-reply --dest=ca.desrt.dconf /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping 2>&1 || true)
i=0; ok=0
while [ $i -lt 15 ]; do
    if dinitctl --user list 2>/dev/null | grep -Eq "(\{\+|\[\+).*\] ca\.desrt\.dconf\.user( |$)" && pgrep -f "^/usr/libexec/dconf-service$" >/dev/null 2>&1; then ok=1; break; fi
    sleep 1; i=$((i+1))
done
if [ "$ok" != 1 ]; then diag; echo "session: dconf unit did not start via dbus activation"; exit 1; fi
pid=$(pgrep -f "^/usr/libexec/dconf-service$" | head -1)
if ! tr "\0" "\n" < /proc/$pid/environ | grep -q "^FERALOS_DBUS_ACT_TEST=session$"; then diag; echo "session: activation env not forwarded to the dinit service"; exit 1; fi
dinitctl --user stop ca.desrt.dconf.user >/dev/null 2>&1 || true
echo "session activation env OK (dconf started via dinit with forwarded env)"
' || exit 1
sysdiag() {
    echo "--- system diagnostics ---"
    dinitctl --system status org.freedesktop.UPower 2>&1 || true
    dinitctl --system list 2>/dev/null | grep -i upower || echo "no upower unit in list"
    up=$(pgrep -f "^/usr/libexec/upowerd$" | head -1)
    if [ -n "$up" ]; then
        echo "upowerd pid=$up"
        tr "\0" "\n" < /proc/$up/environ 2>/dev/null | grep -E "^(DBUS_STARTER_|FERALOS_DBUS_ACT_TEST)" || echo "no DBUS_STARTER_/feralos vars in upowerd env"
    else
        echo "no upowerd process"
    fi
    ls -l /usr/local/share/dbus-1/system-services/org.freedesktop.UPower.service 2>&1
    ls -l /usr/share/feralos/dbus-system-services/org.freedesktop.UPower.service 2>&1
    dinitctl --system catlog org.freedesktop.UPower 2>&1 | tail -10 || true
}
if [ "$(cat /proc/1/comm)" = "dinit" ] && apk info -e upower >/dev/null 2>&1; then
    dinitctl --system stop org.freedesktop.UPower >/dev/null 2>&1 || true
    sleep 1
    pkill -f "^/usr/libexec/upowerd$" 2>/dev/null || true
    sleep 1
    ssend_out=$(dbus-send --system --print-reply --dest=org.freedesktop.UPower /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping 2>&1 || true)
    i=0; ok=0
    while [ $i -lt 30 ]; do
        if dinitctl --system list 2>/dev/null | grep -Eq "(\{\+|\[\+).*\] org\.freedesktop\.UPower( |$)" && pgrep -f "^/usr/libexec/upowerd$" >/dev/null 2>&1; then ok=1; break; fi
        sleep 1; i=$((i+1))
    done
    if [ "$ok" != 1 ]; then sysdiag; echo "system: UPower unit did not start via dbus activation (send: $ssend_out)"; exit 1; fi
    pid=$(pgrep -f "^/usr/libexec/upowerd$" | head -1)
    if tr "\0" "\n" < /proc/$pid/environ | grep -q "^DBUS_STARTER_"; then
        sysdiag; echo "system: upowerd spawned raw (DBUS_STARTER_* present) - activation did not route through dinit"; exit 1
    fi
    echo "system: real activation OK (upowerd started via the shim + dinit, no DBUS_STARTER_*)"
    # Direct-shim env-forwarding proof. Deterministic against two races:
    #   - smooth-recovery: pkill only AFTER the graceful stop completed
    #     (a killed while-stopping process would be resurrected without the
    #     marker and the shim would short-circuit on is-started);
    #   - session re-activation: a desktop client (e.g. KDE powerdevil) can
    #     re-activate upowerd in the window before our shim runs — its
    #     is-started short-circuit would leave a marker-less process. Retry
    #     with a FRESH per-try marker; only OUR shim start carries it.
    try=1
    while [ $try -le 3 ]; do
        dinitctl --system stop org.freedesktop.UPower >/dev/null 2>&1 || true
        i=0
        while dinitctl --system --quiet is-started org.freedesktop.UPower >/dev/null 2>&1 && [ $i -lt 10 ]; do sleep 1; i=$((i+1)); done
        pkill -f "^/usr/libexec/upowerd$" 2>/dev/null || true
        sleep 1
        FERALOS_DBUS_ACT_TEST=system-$try DBUS_STARTER_BUS_TYPE=system DBUS_STARTER_ADDRESS=unix:path=/run/dbus/system_bus_socket /usr/libexec/feralos/dbus-activate org.freedesktop.UPower -- /usr/libexec/upowerd
        sleep 2
        pid=$(pgrep -f "^/usr/libexec/upowerd$" | head -1)
        if [ -n "$pid" ] && tr "\0" "\n" < /proc/$pid/environ 2>/dev/null | grep -q "^FERALOS_DBUS_ACT_TEST=system-$try$"; then
            break
        fi
        try=$((try+1))
    done
    if [ $try -gt 3 ]; then sysdiag; echo "system: activation env not forwarded to the system dinit service"; exit 1; fi
    echo "system activation env OK (upower started via dinit with forwarded env)"
else
    echo "system activation env SKIPPED (no system dinit or upower absent)"
fi
`, cfg.InstalledUser)
		// dbus_activation_migration: forward-looking migration gate (user
		// request 2026-10-19). The FeralOS mechanism (override files + shim +
		// hand-written units) can only be retired when BOTH hold (AND, not
		// OR):
		//   1) dbus-broker is installed (the launcher then activates units by
		//      name itself), AND
		//   2) EVERY D-Bus-activated service on the target is unit-covered —
		//      enumerated live from the standard servicedirs, not from a static
		//      list, so new apps count too. "Covered" = a package-shipped dinit
		//      unit with that exact name, OR a FeralOS override whose shim Exec
		//      maps the name onto an existing package-shipped unit (aliases
		//      like net.hadess.PowerProfiles → org.freedesktop.UPower
		//      .PowerProfiles).
		// Either condition alone is not enough: with dbus-daemon the overrides
		// are still the only route; with incomplete unit coverage the broker
		// would fall back to ephemeral/raw starts. While not both, the check
		// passes and says to keep the FeralOS mechanism. When both hold it
		// FAILS with a "retire" message so the cleanup cannot be forgotten.
		// Unit ownership via `apk info -W` (last field = pkg-version) across
		// /lib, /usr/lib and the admin /etc dinit dirs.
		checks["dbus_activation_migration"] = `
broker=0
apk info -e dbus-broker >/dev/null 2>&1 && broker=1
check_covered() {
    n=$1
    for p in "/lib/dinit.d/$n" "/usr/lib/dinit.d/$n" "/etc/dinit.d/$n" "/etc/dinit.d/user/$n.user" "/usr/lib/dinit.d/user/$n.user"; do
        [ -e "$p" ] || continue
        o=$(apk info -W "$p" 2>/dev/null | sed -n 's/.* is owned by \(.*\)$/\1/p')
        [ -n "$o" ] && return 0
    done
    for s in "/usr/share/feralos/dbus-system-services/$n.service" "/usr/share/feralos/dbus-services/$n.service"; do
        [ -f "$s" ] || continue
        u=$(sed -n 's/^Exec=.*dbus-activate \([^ ]*\) .*/\1/p' "$s" | head -1)
        [ -n "$u" ] || continue
        for p in "/lib/dinit.d/$u" "/usr/lib/dinit.d/$u" "/etc/dinit.d/$u" "/etc/dinit.d/user/$u" "/etc/dinit.d/user/$u.user" "/usr/lib/dinit.d/user/$u" "/usr/lib/dinit.d/user/$u.user"; do
            [ -e "$p" ] || continue
            o=$(apk info -W "$p" 2>/dev/null | sed -n 's/.* is owned by \(.*\)$/\1/p')
            [ -n "$o" ] && return 0
        done
    done
    return 1
}
total=0
missing=0
for f in /usr/share/dbus-1/system-services/*.service /usr/share/dbus-1/services/*.service; do
    [ -e "$f" ] || continue
    name=$(sed -n 's/^Name=//p' "$f" | head -1)
    [ -n "$name" ] || continue
    case "$name" in
        org.kde.kcookiejar5) echo "skip (broken upstream: kded5 binary absent on Alpine 3.24 — no unit possible)"; continue ;;
    esac
    total=$((total+1))
    check_covered "$name" || { missing=$((missing+1)); echo "no package-shipped dinit unit yet: $name"; }
done
if [ "$broker" = 1 ] && [ "$missing" = 0 ]; then
    echo "MIGRATION READY: dbus-broker installed AND all $total activation targets have package-shipped dinit units — retire the FeralOS overrides/shim (ADR-010)"; exit 1
fi
echo "keep the FeralOS activation mechanism (dbus-broker=$broker, $missing/$total activation targets lack package-shipped units)"
`
	}
	if isNiriFamily {
		// dbus_activation_units_live: real activation proof for the third-party
		// targets covered by the new units (ADR-010): one system daemon
		// (org.freedesktop.UDisks2 → udisksd) and one session daemon
		// (org.gnome.OnlineAccounts → goa-daemon). Asserts the dinit unit
		// started via the shim AND the daemon environ has no DBUS_STARTER_*
		// (a raw helper spawn leaves them; the shim path excludes them).
		// The system half needs a system dinit (dinit installs only); the
		// session half runs on both inits via the turnstile user dinit.
		checks["dbus_activation_units_live"] = fmt.Sprintf(`
if [ "$(cat /proc/1/comm)" = "dinit" ]; then
    pkill -f "^/usr/libexec/udisks2/udisksd$" 2>/dev/null || true
    dinitctl --system stop org.freedesktop.UDisks2 >/dev/null 2>&1 || true
    sleep 1
    dbus-send --system --print-reply --dest=org.freedesktop.UDisks2 /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping >/dev/null 2>&1 || true
    i=0; ok=0
    while [ $i -lt 30 ]; do
        if dinitctl --system list 2>/dev/null | grep -Eq "(\{\+|\[\+).*\] org\.freedesktop\.UDisks2( |$)" && pgrep -f "^/usr/libexec/udisks2/udisksd$" >/dev/null 2>&1; then ok=1; break; fi
        sleep 1; i=$((i+1))
    done
    if [ "$ok" != 1 ]; then dinitctl --system status org.freedesktop.UDisks2 2>&1 || true; dinitctl --system list 2>/dev/null | grep -i udisks || true; echo "system: UDisks2 unit did not start via dbus activation"; exit 1; fi
    pid=$(pgrep -f "^/usr/libexec/udisks2/udisksd$" | head -1)
    if tr "\0" "\n" < /proc/$pid/environ | grep -q "^DBUS_STARTER_"; then echo "system: udisksd spawned raw (DBUS_STARTER_* present)"; exit 1; fi
    dinitctl --system stop org.freedesktop.UDisks2 >/dev/null 2>&1 || true
    echo "system unit activation OK (UDisks2 via shim + dinit)"
else
    echo "system unit activation SKIPPED (no system dinit)"
fi
su - %s -s /bin/sh -c '
export XDG_RUNTIME_DIR=/run/user/$(id -u)
export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
dinitctl --user stop org.gnome.OnlineAccounts.user >/dev/null 2>&1 || true
pkill -f "^/usr/libexec/goa-daemon$" 2>/dev/null || true
sleep 1
dbus-send --session --print-reply --dest=org.gnome.OnlineAccounts /org/freedesktop/DBus org.freedesktop.DBus.Peer.Ping >/dev/null 2>&1 || true
i=0; ok=0
while [ $i -lt 20 ]; do
    if dinitctl --user list 2>/dev/null | grep -Eq "(\{\+|\[\+).*\] org\.gnome\.OnlineAccounts\.user( |$)" && pgrep -f "^/usr/libexec/goa-daemon$" >/dev/null 2>&1; then ok=1; break; fi
    sleep 1; i=$((i+1))
done
if [ "$ok" != 1 ]; then echo "session: goa unit did not start via dbus activation"; dinitctl --user list 2>/dev/null | grep -i onlineaccounts || true; exit 1; fi
pid=$(pgrep -f "^/usr/libexec/goa-daemon$" | head -1)
if tr "\0" "\n" < /proc/$pid/environ | grep -q "^DBUS_STARTER_"; then echo "session: goa spawned raw (DBUS_STARTER_* present)"; exit 1; fi
dinitctl --user stop org.gnome.OnlineAccounts.user >/dev/null 2>&1 || true
echo "session unit activation OK (goa via shim + dinit)"
' || exit 1
echo 'dbus activation units live OK'
`, cfg.InstalledUser)
	}
}
