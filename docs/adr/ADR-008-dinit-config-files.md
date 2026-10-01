# ADR-008: dinit Config Files — conf.d-equivalent via Artix Wrapper Pattern

- **Status:** Accepted
- **Date:** 2026-10-01
- **Deciders:** FeralOS team

## Context

OpenRC services ship a user-editable `/etc/conf.d/<svc>` file that the init script sources before starting the daemon — the standard Alpine customization point (52 conf.d files inventoried on an OpenRC target via the `confd_dump` harness check). The dinit conversion hardcoded the daemon flags directly in the service files, so there was no user customization point at all: changing e.g. `chronyd` options meant editing a package-owned service file that the next `apk upgrade` overwrites.

## Decision

Adopt the Artix pattern for **ALL** `-dinit` packages (22 converted: acpid chrony cronie cups earlyoom fwupd greetd irqbalance smartmontools rsync dnsmasq openssh klogd vector dbus incus incus-feature avahi lxc cpufrequtils iptables nftables):

- The service file's `command` becomes `/usr/lib/dinit/<svc>` — a small shell wrapper.
- The wrapper sources the config, then execs the daemon: `[ -r /etc/dinit.d/config/<svc>.conf ] && . /etc/dinit.d/config/<svc>.conf` → `exec <daemon> ${VARS}`.
- The config file installs 644 under `/etc/dinit.d/config/<svc>.conf` — apk-protected: user edits survive upgrades, a new upstream version lands as `.apk-new` (same semantics as `/etc/conf.d`).
- Defaults match the Alpine `conf.d` files; even an empty placeholder is shipped so the user has a customization point from day one.
- Scripted services (cpufrequtils, iptables, nftables, klogd) follow the same config-file pattern; cpufrequtils gained a `stop-command` that consumes `CPUFREQ_STOP_OPTS`.
- Exception: `dnsmasq` `DNSMASQ_CFGFILE` defaults to `/etc/dnsmasq.conf` — an empty value would mean *no config at all*, so the placeholder carries the meaningful default.

## Consequences

- +1 wrapper +1 config per service (package size, but trivial shell files).
- Users customize daemons by editing `/etc/dinit.d/config/<svc>.conf` without touching (apk-protected) service files — OpenRC `/etc/conf.d` convention restored.
- apk upgrade-safe by construction (644 config files are protected; modified files preserved as `.apk-new` on conflict).
- CI `client-test` has a GitHub Pages propagation race (repo published before Pages serves the new version) — transient; a re-run passes.
- Verified: dinit niri full **74/74** (run `2026-10-01T22-19-23Z`), OpenRC minimal **46/46** (run `2026-10-01T22-27-05Z`, check correctly absent). Harness gains a permanent `dinit_config` check (config dir + ≥10 `.conf` + wiring spot-checks).

## References

- feralos-aports commit `cc107a1` — 22 `-dinit` packages converted to wrapper+config (CI green after transient Pages-race re-run)
- Harness: `tests/harness/internal/service/service.go` — `dinit_config` check (EnableDinit only)
- Pattern source: Artix `packages/<name>-dinit` service + `/usr/lib/dinit/<svc>` wrapper layout
- `AGENTS.md#Activatable Feature Confinement`, `docs/adr/ADR-007-dinit-shutdown-ordering.md` (prior dinit aports convention round)
