# ADR-009: dinit Service Conventions — Artix/Chimera Alignment

- **Status:** Accepted
- **Date:** 2026-10-02
- **Deciders:** FeralOS team

## Context

An exhaustive audit of our 48 `-dinit` packages against Artix (full gitea enumeration: 7370 repos, plus the 18768-package binary repo checked for `-dinit` subpackages) and Chimera (cports + dinit-chimera) found conventions drift accumulated across the conversion rounds:

- `restart = true` on daemons — Artix and Chimera **never** set `restart` on daemons; crash recovery is `smooth-recovery` only. Our restart directives could mask crashes and flap-loop on persistent failures.
- No `before = login.target` on user-facing services — Artix and Chimera put login.target on *every* service a user session may touch (33 of ours were missing it), so login could proceed before e.g. dbus-adjacent services were ready.
- Network ordering incomplete: firewalld/nftables/iptables had no `before = pre-network.target`, NetworkManager did not depend on `pre-network.target` nor order `before = network.target`, avahi/rsync did not depend on `networking` — a firewall race window existed during boot.
- `dnsmasq` ran as root — Artix drops privileges (`-u dnsmasq -g dnsmasq`) and enables the DBus interface.
- `getty` hardcoded (baud/term) — Artix uses an `agetty-default` wrapper + config (`/etc/dinit.d/config/getty.conf`), serial defaults 115200.
- `lxc` latent bug: `process` + `restart = true` on a one-shot (`lxc-autostart`) — a restart loop waiting to fire.
- Audit facts established: Artix dbus-activates polkit/udisks2/fwupd (no init service by design); irqbalance/rasdaemon/incus/nix/vector/klogd/cosmic-greeter are NOT packaged on Artix; `lxd-dinit` + `cpupower-dinit` are the existing references for lxc/cpufrequtils; turnstile-dinit on Artix is a split subpackage (we already match its 2 seds).

## Decision

Align all 48 `-dinit` packages to the Artix/Chimera service conventions (48 pkgs updated, aports commit `b69a573`, CI green):

- **No `restart` on daemons** — crash recovery is `smooth-recovery` only, matching Artix/Chimera universally. Sole exception: `getty` keeps `restart = true` (login must come back).
- **Scripted one-shots** declare `restart = false` explicitly.
- **`before = login.target`** added to the 33 services a user session may touch (Artix+Chimera universal convention).
- **Network ordering chain**: firewalld/nftables/iptables `before = pre-network.target`; NetworkManager `depends on pre-network.target` + `before = network.target`; avahi/rsync depend on `networking`; apparmor orders `before local.target` (after `early-fs-local.target` — profiles load BEFORE local services); fuse after `early-root-rw.target`.
- **dnsmasq privilege drop**: `-u dnsmasq -g dnsmasq --enable-dbus` (Artix exact; was running as root).
- **dbus `smooth-recovery = no`** — nobody restarts the bus; the ADR-007 stop guard stays (verified clean shutdown elogind → dbus → power down).
- **Bus-name readiness** via the NEW package `dbus-wait-for-dinit` (BSD-2-Clause helper from Artix dinit-rc, libdbus, installed at `/usr/lib/dinit/dbus-wait-for`, `pipefd:4`): services whose daemon registers on the system bus (bluez, elogind) use it as readiness condition.
- **One-shots scripted, not process**: `lxc` fixed — scripted start/stop + `stop-command` instead of `process` + `restart = true` (restart-loop bug eliminated).
- **sshd**: scripted `ssh-keygen` prereq service — keys baked at install, belt-and-braces (Artix/Chimera pattern).
- **ADR-008 config pattern retained as FeralOS extension** — the wrapper + `/etc/dinit.d/config/<svc>.conf` convention is NOT in Artix/Chimera (they hardcode flags); we keep it on top of the aligned conventions.

## Consequences

- Shutdown ordering deterministic; no firewall race window at boot (firewalls up before pre-network, NM up before network target).
- dnsmasq unprivileged; lxc no restart loop; no daemon flap-looping on persistent crash (restart removed).
- **New dependency**: `dbus-wait-for-dinit` for bluez/elogind bus-name readiness.
- Divergence from Artix by design remains in two places: ADR-008 wrapper+config files (customization point) and `restart = true` on getty only.
- Verified: dinit niri full **74/74** (run `2026-10-02T00-15-22Z` — getty serial alive, dbus clean shutdown, elogind bus readiness, NM ordering), OpenRC minimal **49/49** (run `2026-10-02T00-27-53Z`, +3 checks = harness growth not regression).

## References

- feralos-aports commit `b69a573` — 48 `-dinit` packages aligned (103 files, +861/−163, CI green first try)
- Audit method: Artix gitea repo enumeration (7370) + binary repo `-dinit` subpackage check (18768 pkgs) + Chimera cports + dinit-chimera
- New package: `dbus-wait-for-dinit` (BSD-2-Clause, from Artix `dinit-rc`, libdbus)
- `docs/adr/ADR-007-dinit-shutdown-ordering.md` (dbus/elogind stop guards, kept), `docs/adr/ADR-008-dinit-config-files.md` (wrapper+config pattern, retained)
- `AGENTS.md#Activatable Feature Confinement` (dinit line)
