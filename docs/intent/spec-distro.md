# Spec — FeralOS Distribution

**The specification of the distribution.** Single source for what ships and why. Goals live in `docs/intent/goals.md`; this document specifies the system itself. (The former toolchain-round file `spec-2026-11-13-opencode-v2-toolchain.md` was removed: the spec is the distribution, not the editor it was written with.)

## 1. System definition

FeralOS = Alpine Linux base (musl + apk + audited/stripped packages) + memory-safe userland (Rust/Go replacements) + **dinit** init + the **FeralOS dbus routing** (ADR-010) + **booster/UKI/LUKS2/Btrfs** boot+storage stack + the **niriWayle** desktop composition + **flatpak** and **nix** for full software coverage + **ursine** control center. Everything except containers.

## 2. Installer specification

The distribution **starts from the installer**. No correct installer exists in the Linux world; FeralOS ships its own instead of adopting one.

| Requirement | Spec |
|---|---|
| Interface | **TUI** — unites ease of use with **headless** installs |
| Correctness | The install it produces is the product; validated by the QEMU harness on every change |
| Organization | Phase-isolated: `main.go` entry only → hostPhase (writes) → chrootPhase (runs) → firstBoot binaries; internal package rule enforced |
| Readability | Understandable by reading — one orchestrator file with the 17-step flow (see `docs/INSTALLER.md`) |
| Modifiability | `internal_rules.yaml` drives packages, services, runlevels, tunables — most installer changes are data, not code |
| Replicability | **Unattended mode** (`variables.yaml`) → enterprise deployments through Ansible |
| Testability | QEMU harness: 30 fixtures, every DE, every feature — **new things are tested before shipping** |

## 3. Package policy specification

1. **Alpine first, minimal patches.** Upstream Alpine software is tested and well-patched — no forks, no vendoring.
2. **Own packages only where the alternative is logically wrong**: the `*-dinit` service suite (supervised, restart=on-failure semantics), the dbus routing (every D-Bus activation starts a dinit unit — ADR-010), **ursine** (control center: core + daemon + CLI; the daemon is queryable from GUI or web — one code path, always the same result; replaces Webmin without Cockpit's dependency bloat; built on the CLI tools where Linux has always excelled).
3. **apk, forever.** apk guarantees a distribution that keeps working over the entire life of an installation. `internal_rules.yaml` and the removal policy keep configs working: remove a component, re-add it later — the configuration survives.
4. **Full software coverage**: flatpak (proprietary) + nix (widest repository) integrated first-class; core stays musl/apk.
5. **Full desktop environments are not shipped** as FeralOS products (KDE/COSMIC only as conveniences): no control over installed content, broken pieces, DE-bundled software worse than best-of-breed standalone alternatives.

## 4. Desktop specification

- **Composition, not monolith**: independent best-of-breed modules (niri, wayle, raffi, veila, stasis, oo7, matugen, ursine — Rust) wired to feel like one coherent product. The user never notices the modules are independent.
- **Minimal-but-full**: the shipped desktop works completely out of the box; any piece can be removed and the rest keeps working; removing does not break the configuration.
- **Future**: remove **GTK** (not memory safe); eliminate every component dragging heavy dependencies from other desktop environments.
- **Future target**: **Smart TV** desktop (TV is a first-class target per §6).

## 5. Security and observability specification

| Area | Shipped today | Open TODO |
|---|---|---|
| Init/services | dinit (not systemd), supervised user services, restart=on-failure semantics | — |
| Boot | signed UKI, LUKS2 on root/swap/RAID1, Secure Boot binary (first boot), TPM auto-unlock | — |
| Auth/sudo | sudo-rs, pam stack audited (base-auth vendordir) | — |
| Networking | firewalld (nftables), per-service readiness gates | — |
| Hardening | AppArmor, earlyoom tiered victim selection, kernel cmdline discipline | — |
| **Standards conformance** | partial (hardening defaults) | **formal security-standards conformance — open TODO** |
| **Observability** | vector logging, ursine service views | **enterprise observability out of the box (metrics/tracing/alerting) — open TODO** |

The distribution is security-oriented **for real**, not by claim: the difference is enforced by rules (feature confinement, supervision audit, source pinning) rather than marketing.

## 6. Targets

Desktop · VM (unencrypted layout) · hypervisor (incus) · Smart TV (future). **Containers: out of scope** — Alpine Linux is the answer there.

## 7. Future specification (declared, not started)

1. **Memory-safe aports replacement** — same APKBUILD format, **bootstraps the distribution FROM NOTHING** (self-host from zero, like Chimera's cports: no host distro needed — Alpine lacks this, as do most distributions), isolated builds, LLVM with maximum static code. **Independent per-package tests** (know the weaknesses of what we ship) + **published-CVE integration**. Blocked on donations → infrastructure + maintainers.
2. **apk ported to Go** — remove one of the last critical non-memory-safe system components.
3. **Chroot installation like Arch Linux** — documented in the wiki, with most standard configuration moved into aports packages.
4. GTK removal; cross-DE heavy-dependency elimination.

## 8. Acceptance (how an AI verifies alignment)

- Every change keeps the installer harness green (currently 118/118 flagship, all DEs green).
- Every new component either comes from Alpine with minimal patches or is one of the specified own packages.
- Every new feature is testable through the harness before shipping.
- Documentation updated in the same change (Documentation Rule); `AGENTS.md` stays the index.
- Nothing in this spec regresses: memory-safety choice, minimal-patches policy, removal-without-breaking, perceived-speed choices (dinit/TPM/greetd/niri).
