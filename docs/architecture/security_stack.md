# FeralOS Security Stack

**Canonical detail source for the base security stack** (ADR-012). `AGENTS.md` links here; this file owns the per-tool inventory, file paths, and the deferred-work TODOs.

## Principle

Local-only desktop stack, shipped by the BASE installer (no flag), minimum tool count, every tool logs to `/var/log` (Vector collects; manual remote shipping later = one `conf.d` drop-in). Memory-safe preference — the two C exceptions (auditd userspace, AIDE) are documented in ADR-012 with rationale.

## Stack inventory

| Layer | Package(s) | Service units (both inits) | Config (installer-written) | Log path | Vector source |
|---|---|---|---|---|---|
| Syscall audit | `audit` + `audit-openrc` + `audit-dinit` (auto via conversion) | `auditd` | `/etc/audit/audit.rules` (`internal/security_stack.go`) | `/var/log/audit/audit.log` (self-rotated by auditd.conf) | `audit_logs` |
| File integrity | `aide` (feralos port of Alpine testing; no daemon) | — (daily cron `/etc/periodic/daily/aide-check`) | `/etc/aide.conf` + baseline db (`internal/security_stack.go`) | `/var/log/aide/aide.log` | `aide_logs` |
| Brute-force IPS | `fail2ban` + `fail2ban-dinit` (BOTH-INIT override, `replaces=fail2ban-openrc`) | `fail2ban` | `/etc/fail2ban/jail.d/50-feralos.conf` (`internal/security_stack.go`) | `/var/log/fail2ban.log` (own logrotate) | `fail2ban_logs` |
| Runtime detection | `tracee` (feralos port — first musl packaging; SHARED build) | `tracee` | `/etc/dinit.d/config/tracee.conf` (tuning; also `/etc/conf.d/tracee`) | `/var/log/dinit/tracee.log` (dinit) / `/var/log/tracee.log` (OpenRC) | rides `dinit_services` (dinit) |
| Conformance report | `lynis` (feralos port; no daemon) | — (daily cron from package) | none (package default.prf) | `/var/log/lynis.log` | `lynis_logs` |
| Malware (HARDENING-gated) | `clamav-daemon` + `clamav-scanner` + `clamav-db` + `clamav-daemon-dinit` + `freshclam-dinit` | `clamd` (depends-on freshclam), `freshclam` | `/etc/clamav/clamd.conf` (`features/hardening.go`) | `/var/log/clamav/clamd.log` + `freshclam.log` (own logrotate) | `clamav_logs` |

## File ownership (installer)

- `src/chrootPhase/internal/security_stack.go` — BASE stack configs (audit rules, fail2ban jail, aide config/cron), called step 2b; `BaselineAide` at step 18 (AFTER the edge install — the baseline must capture the final filesystem state).
- `src/chrootPhase/features/hardening.go` — ClamAV packages + `/etc/clamav/clamd.conf` (feature confinement: hardening owner file).
- `src/chrootPhase/internal/logging.go` — Vector gated sources + `conf.d` drop-in dir + README + install-time `vector validate --skip-healthchecks`.
- aports: `audit-dinit`, `fail2ban-dinit`, `tracee`, `aide`, `lynis`, `clamav-daemon-dinit`, `freshclam-dinit` (all tag-pinned).

## Key mechanics

- **audit.rules**: lean baseline, existence-filtered watches (sudoers/doas/ssh), `-e 2` immutable-until-reboot LAST; loaded by `auditctl -R` best-effort (dinit wrapper `|| true` — immutable reloads fail with EPERM by design and must never kill the daemon). auditd.conf (package default) self-rotates.
- **fail2ban**: `nftables-multiport` banaction (base has NO iptables; f2b table independent of firewalld's tables); sshd jail tails vector's `/var/log/auth.log` (polling backend); first-boot race (vector not yet written auth.log) self-heals via smooth-recovery.
- **Tracee**: shared musl build ON PURPOSE (static loads ZERO Go-signature plugins); `builtin.so` compiled in the same musl/Go env (abuild); binary at `/usr/libexec/tracee/tracee` (signatures MUST sit in `<dir-of-exe>/signatures/`); libbpf 1.5.1 vendored-submodule pinned as a second tag-archived source; eBPF object compiled at build time (clang19) and `go:embed`-ed. Tuning: `TRACEE_ARGS` (event scope, `--perf-buffer-size`) — default loads the full event catalog; third-party idle RAM ≈ 500MB.
- **lockdown=integrity** (hardening cmdline, ADR-012): `LOCKDOWN_BPF_READ_KERNEL` (confidentiality-only) rejects kernel-memory-read BPF programs — Tracee cannot load under confidentiality.
- **ClamAV**: `clamd --foreground` (CLI overrides config file — clamd.c argv scan); `freshclam --daemon --foreground --user clamav` (dinit) / package initd (OpenRC); DB snapshot ships in `clamav-db` → offline-first, freshclam updates on network; clamd depends-on freshclam (the Alpine `rc_need="freshclam"` semantic).
- **Vector conf.d**: `/etc/vector/conf.d/*.yaml` merged on top of the base config by the wrapper (shell-resolved glob — empty dir = no extra args); manual remote shipping = one admin file; README documents component-ID collisions.

## TODO (deferred by owner decision, 2026-11-15)

1. **CrowdSec offline port** (engine + `cs-firewall-bouncer` + baked hub; research done: engine ~160MB RSS, bouncer ~26MB, hub MIT, CAPI off config verified) → **replaces fail2ban** when landed.
2. **firewalld replacement** — approach to be defined by the project owner (consumer analysis not yet done: NetworkManager/plasma-firewall/libvirt/incus/podman).

## Verification

Harness: `auditd_service`, `auditd_rules`, `auditd_immutable`, `fail2ban_service`, `fail2ban_sshd`, `tracee_running`, `tracee_log`, `aide_baseline`, `aide_cron`, `lynis_present`, and under EnableHardening `clamd_service`, `freshclam_service`, `clamav_db_snapshot`; `services_status` golden lists carry `auditd`/`fail2ban`/`tracee` on both inits; `vector_conf_d` proves the drop-in mechanism.
