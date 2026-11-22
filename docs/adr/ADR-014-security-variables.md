# ADR-014: Security Variables — baseSecurity / completeSecurity / hardening

- **Status**: Accepted
- **Date**: 2026-11-22
- **References**: `docs/architecture/security_stack.md`, `docs/architecture/package-configs.md`, `docs/architecture/feature_confinement.md`, `docs/guides/variables.md`, ADR-012, ADR-013

## Context

ADR-012 (2026-11-15) shipped the "base security stack" — auditd, AIDE,
fail2ban, Tracee, lynis — as ALWAYS-ON (hardcoded in `standardPackages`):
every install carries the enterprise observability tools whether or not the
target wants them. Tracee alone idles around 500 MB RAM. Meanwhile the
kernel hardening (sysctls, module blacklist, cmdline fragment, umask) and
AppArmor rode one `enableHardening` toggle through the `hardening-config`
family package, mixing three different concerns in one switch:

1. Mandatory access control (AppArmor) — a posture, not a tool set.
2. Enterprise observability (auditd/AIDE/fail2ban/Tracee/lynis) — heavy,
   opt-in-able by nature.
3. Static kernel hardening (+ ClamAV, the only tool exception the owner
   wants) — cheap, static, no daemons.

The installer also still wrote the last two static security configs
(`/etc/audit/audit.rules`, `/etc/aide.conf` + daily cron) — the open rows of
the ADR-013 migration inventory.

## Decision

Split the security work into THREE INDEPENDENT variables (owner decisions
2026-11-22 — defaults D1: `enableBaseSecurity: true`,
`enableCompleteSecurity: true`, `enableHardening: false`; D2: set-permissions
stays in the hardening family; D3: `enable*` YAML prefix; D4: no layering —
the toggles do not imply each other):

| Variable | Go owner (`features/`) | Package (ADR-013 R2) | Content |
|---|---|---|---|
| `enableBaseSecurity` | `basesecurity.go` | `base-security-config` | AppArmor stack + `40-apparmor.conf` LSM cmdline fragment (trigger: reassemble + UKI rebuild) |
| `enableCompleteSecurity` | `completesecurity.go` | `complete-security-config` | audit, aide, fail2ban (+ fail2ban-config), tracee, lynis + `audit.rules` + `aide.conf` + daily cron |
| `enableHardening` | `hardening.go` (existing) | `hardening-config r5` | sysctls 93-99, module blacklist, `50-hardening.conf`, umask, ClamAV stack, `set-permissions` (boot) |

- AppArmor LEAVES `hardening-config` and `50-hardening.conf`: the LSM
  enablement (`apparmor=1 security=apparmor` fragment + stack deps + boot
  enablement) is a base-security concern.
- The enterprise stack LEAVES `standardPackages`: nothing security-related is
  always-on anymore; only infrastructure (klogd, vector, feralos-dbus-audit).
- `internal/security_stack.go` is DELETED: `writeAuditRules`/`writeAideConfig`
  move to the `complete-security-config` package (the vendor audit package
  ships no `audit.rules`; the existence-filtered sudo/doas/ssh watches
  materialize at POST-INSTALL — an install-time fact, idempotent, above the
  `-e 2` immutable marker). `BaselineAide` stays Go in
  `features/completesecurity.go` (step 18: the baseline must capture the
  FINAL filesystem state — not packageable).
- Service enablement for dep-installed packages rides the FAMILY keys in
  `internal_rules.yaml` (`base-security-config` → apparmor boot;
  `complete-security-config` → auditd/fail2ban/tracee default;
  `hardening-config` → clamd/freshclam/set-permissions) — the standalone
  `audit`/`fail2ban`/`tracee`/`clamav-daemon` keys remain for manual apk-add
  installs only.
- `BaselineAide` gates on BOTH `enableCompleteSecurity` and the `aide` package
  being present in the live `apk info` list.

## Consequences

- A minimal install can now opt out of ~0.5 GB of Tracee RAM and the whole
  enterprise stack with `enableCompleteSecurity: false`, while keeping the
  MAC baseline on (or off) — three orthogonal knobs instead of one.
- The ADR-013 static-config inventory is EMPTY for security files: the
  chroot-installation prerequisite (spec-distro §7 item 3 — no installer
  writing configs) advances.
- Harness checks are gated per variable: the auditd/AIDE/fail2ban/Tracee/
  lynis domain only registers under `enableCompleteSecurity`, the AppArmor
  checks under `enableBaseSecurity`, clamav under `enableHardening`; the
  `services_status` golden lists append auditd/fail2ban/tracee only under
  `enableCompleteSecurity`.
- The `apparmor=1` cmdline tokens now appear in `/etc/kernel/cmdline` only
  when `base-security-config` is installed (the harness `apparmor=1` grep
  moved from `config_packages_hardening` to `config_packages_base_security`).
- ADR-012's "always-on enterprise observability OOTB" goal is superseded:
  the stack ships OOTB via the DEFAULT (`true`), but a user variables.yaml
  without the new keys gets zero-value `false` (YAML semantics) — document
  the defaults in the template and `variables.md`.
