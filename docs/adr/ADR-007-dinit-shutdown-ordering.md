# ADR-007: dinit Shutdown Ordering — Session Trackers Before Bus/elogind

- **Status:** Accepted
- **Date:** 2026-09-30
- **Deciders:** FeralOS team

## Context

dinit stops a dependency as soon as its hard dependents have *released* it — the release happens when a dependent *begins* stopping (`src/service.cc` → `release_dependencies()` on `prop_release`), not after its process has exited. At system shutdown that lets `dbus-daemon`/`elogind` die while `turnstiled` (session tracker) is still closing PAM sessions; `pam_elogind` then logs `pam_elogind(turnstiled:session): Failed to connect to system bus: Connection refused` (harness run `2026-09-29T22-35-57Z`, `log_errors` FAIL). `depends-on` cannot express "do not stop until the dependent has fully exited" in dinit 0.21 — verified against dinit 0.21 source and reproduced with a local dinit build (dependency TERM'd 3.7 s before the dependent finished tearing down).

## Decision

Enforce shutdown ordering with dinit's `stop-command` mechanism (feralos-aports):

- `dbus-dinit` r6 / `elogind-dinit` r5 service files add `stop-command = /usr/libexec/<pkg>-stop` and `stop-timeout = 30`.
- `stop-command` runs **instead of** dinit signalling the process: the script waits (bounded, ~15 s) for `turnstiled` to exit — matched via `pidof` (Alpine busybox `pgrep` has no `-x`) — then stops the daemon itself (TERM, then KILL escalation). `stop-timeout = 30` is the failsafe if the script ever wedges.
- When turnstile is not installed the wait is a no-op; behaviour unchanged (kde/cosmic/OpenRC installs).

## Consequences

- The bus and elogind outlive PAM session closing at shutdown — zero "Failed to connect to system bus" occurrences; the harness `log_errors` allowlist entry for this line stays removed (no masking) and regressions fail the run.
- Worst-case shutdown delay if `turnstiled` hangs: ~15 s, then daemons stop anyway (script KILL escalation + dinit `stop-timeout`).
- Manual `dinitctl stop dbus` with an active session waits for `turnstiled` first (rare; stopping the bus already tears down all bus clients).
- Pattern to reuse: shutdown ordering that dinit dependencies cannot express → `stop-command` guard on the dependency (extend to other session managers if needed).

## References

- feralos-aports commit `75c566a` — `fix(dbus-dinit,elogind-dinit): stop-command keeps bus/elogind alive until turnstiled exits` (CI build+deploy+client-test green)
- dinit 0.21 source: `src/service.cc` (`release()`, `release_dependencies()`, `stop_check_dependents()`), `src/proc-service.cc` (`bring_down()` — stop-command replaces the signal)
- Harness runs: `2026-09-29T22-35-57Z` (62/63, bug visible) → `2026-09-30T00-08-04Z` (63/63 PASS)
- `AGENTS.md#FirstBoot Architecture`, `docs/guides/niri.md`, `docs/architecture/overview.md`
