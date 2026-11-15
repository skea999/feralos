# ADR-012: Base Security Stack — auditd + AIDE + fail2ban + Tracee + lynis

- **Status**: Accepted (2026-11-15)
- **Context**: `docs/intent/goals.md` open TODO — "formal security-standards conformance and enterprise observability out of the box". FeralOS already ships hardening (AppArmor, kernel cmdline discipline, LUKS2, Secure Boot) and Vector logging (local), but no runtime detection, no integrity monitoring, no brute-force response, no conformance reporting.
- **Decision**: a six-tool, LOCAL-ONLY security stack shipped by the BASE installer (every target, no flag) + ClamAV gated behind `enableHardening`:

| Layer | Tool | Language | Notes |
|---|---|---|---|
| Log pipeline | Vector (existing) | Rust | local sinks; manual remote shipping via `/etc/vector/conf.d` drop-ins |
| Syscall audit | auditd (main) | C (kernel-native framework) | lean ruleset, `-e 2` immutable, auditd self-rotates `/var/log/audit/` |
| File integrity | AIDE 0.19.5 (ported from testing) | C | baseline DB generated in-chroot (step 18, AFTER the edge install); daily cron check |
| Brute-force IPS | fail2ban 1.1.1 (main) | Python | sshd jail, `nftables-multiport` banaction, log = vector's `/var/log/auth.log` |
| Runtime detection | Tracee 0.24.1 (ported — first musl packaging anywhere) | Go + eBPF | **shared** musl build (static cannot load Go-signature plugins — `builtin.so` is `--buildmode=plugin`); signatures compiled in the same musl/Go env |
| Conformance report | lynis 3.1.6 (ported from testing) | shell | report-only, daily cron, no daemon |

  ClamAV (community) under `enableHardening`: clamav-daemon + clamav-scanner + clamav-db (snapshot = offline-first) + freshclam (updates when a network is up; daemon `--daemon --foreground` under dinit).

- **Rationale highlights**:
  - "FIM in Rust" does not exist (top project 208★ hobby grade) — AIDE is the only credible FIM; C exception justified (batch, offline, small attack surface).
  - CrowdSec was researched and DEFERRED (port = engine + `cs-firewall-bouncer` + baked hub; fully offline feasible per research). fail2ban ships first; CrowdSec replaces it later (backlog TODO).
  - firewalld replacement: DEFERRED — approach to be defined by the project owner (backlog TODO).
  - Trivy / usbguard / rkhunter / chkrootkit: evaluated, rejected (DB-pull dependency / not selected / absent in Alpine + covered by Tracee+AIDE).
- **Consequences**:
  - **`lockdown=confidentiality` → `lockdown=integrity`** in the hardening cmdline: `LOCKDOWN_BPF_READ_KERNEL` is confidentiality-only but makes the BPF verifier reject every kernel-memory-read program — Tracee cannot load under confidentiality. integrity still blocks unsigned module loading, kexec and /dev/mem, and cannot be lowered at runtime.
  - The aports `-dinit` NAME CONTRACT holds: one daemon, one `-dinit` package named `<base>-dinit`, auto-installed by the conversion loop (`packages.go` scans installed `-openrc` subpackages). clamd/freshclam = `clamav-daemon-dinit` + `freshclam-dinit` (NOT one bundled package — Alpine ships clamav-daemon-openrc and freshclam-openrc separately). fail2ban-dinit is a BOTH-INIT override (`replaces=fail2ban-openrc` — the vendor initd `need logger` can never start under Vector). Tracee ships initd + dinit unit in the single main package (turnstile precedent, no conversion involvement).
  - Config ownership: `src/chrootPhase/internal/security_stack.go` (base stack configs: audit rules, fail2ban jail, aide config + cron + baseline), `features/hardening.go` (clamd.conf + ClamAV packages). Every tool logs to a `/var/log` file wired into Vector (`P1` gated sources); logrotate covers ONLY what packages don't self-manage (aide/lynis — auditd self-rotates, fail2ban/clamav ship their own logrotate).
  - Tracee idle footprint is third-party-measured ~500MB RAM (K8s context) — tuning via `/etc/dinit.d/config/tracee.conf` (`TRACEE_ARGS`: event scope + `--perf-buffer-size`); harness validates the real QEMU footprint.
- **Verification**: harness `security_stack` checks (auditd service + loaded immutable rules, fail2ban-server + sshd jail, tracee process + event log, aide baseline db + daily cron, lynis; clamd/freshclam/db under EnableHardening) + `services_status` golden lists updated (auditd/fail2ban/tracee expected on both inits).
- **References**: `docs/architecture/security_stack.md` (canonical detail), `docs/intent/goals.md`, `docs/adr/ADR-008-dinit-config-files.md`, `docs/adr/ADR-009-dinit-service-conventions.md`.
