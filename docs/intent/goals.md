# Goals — FeralOS

**Single source for what FeralOS is and what every AI/human works toward.** Read this before any task.

## Mission

FeralOS is a **complete, refined, production-quality Linux distribution** based on Alpine Linux: **maximally memory-safe**, **genuinely security-oriented** (not security-claiming), with **enterprise observability out of the box**, **perceived-fast** (software that never hangs), **lightweight but full-featured**, suitable for **every purpose except containers** (where Alpine Linux itself is unbeatable). The distribution is refined and perfected: quality bar is "finished product", not prototype.

## Objectives (in order of priority)

1. **Memory safety first, continuously.** Replace attack-exposed C components with memory-safe alternatives (Rust/Go) whenever a **new, fast, correct** replacement exists — and keep updating forever as the ecosystem matures. Prefer the **safest** technology, never the newest for its own sake, never legacy, never "industry standard but weaker" (examples of our choices: **Wayland** not X11, **dinit** not systemd/OpenRC, the **FeralOS dbus routing** per ADR-010, musl not glibc, uutils/sudo-rs/fish/vector per the replacement table).

2. **Actually secure, not security-claiming.** Every other distribution claims security; FeralOS ships it: conformance to security standards + hardening by default. **TODO (not yet done): formal security-standards conformance and enterprise observability out of the box.** These are tracked as open work.

3. **Speed you can feel.** NOT kernel tuning and NOT max-FPS chasing (CachyOS-style micro-gains are pointless work for minimal benefit). The software must be **fast and never hang** — that is what makes the user perceive the distribution as fast. Choices that follow from this: **dinit** (parallel boot, correct dependencies), **TPM auto-unlock** even with full-disk encryption (no password friction at boot), **greetd**, **niri** (Rust compositor, fast and stable).

4. **Minimal core, full system.** Base = Alpine repositories (audited, stripped packages). **apk guarantees the distribution keeps working forever** — unlike package managers that do many things but break over the life of an installation. **Full desktop environments are discouraged**: with a full DE you lose control over what gets installed, you always get broken pieces, and DE-bundled software is worse than the best standalone alternatives. FeralOS instead ships a **coherent, custom-made desktop composition** (niriWayle: independent best-of-breed modules that feel like one product) and the **complete software set working out of the box** — the user can remove anything they do not need and the remaining configurations keep working if they change their mind.

5. **All the software, no compromise.** Flatpak (**most proprietary software**) and Nix (**most software overall**) are integrated first-class so nothing available for Linux is out of reach — without sacrificing the memory-safe musl core.

6. **Everything starts from the installer.** No correct installer exists in the Linux world — that is why FeralOS begins here, even though much of the system ships through aports. The installer must be: **TUI** (ease of use + headless installs), **correct**, **well organized**, **readable** (understandable by reading, no archaeology), **modifiable** — `internal_rules.yaml` drives practically everything — and **unattended** so enterprise deployments replicate it through Ansible scripts. Starting from the installer also means **everything is testable before shipping**: the QEMU harness exercises practically the whole system automatically.

7. **Alpine software with minimal patches.** Upstream Alpine software is tested and well-patched — we do not fork it. We ship **few packages of our own** (`*-dinit` service suite, the dbus routing, **ursine**) only where the alternatives are **logically wrong** (stack choice or organization). Ursine is the model: core + daemon + CLI, the daemon queryable from GUI or web — one code path, always the same result; it replaces Webmin without Cockpit's dependency bloat, leaning on the CLI tools where Linux has always excelled.

8. **Runs everywhere (except containers).** Desktop, VM, hypervisor, TV. The container space belongs to Alpine Linux — we do not compete there.

## Future work (declared goals — not yet started)

- **Chroot installation like Arch Linux**, documented in the wiki, moving most of the standard configuration into aports packages.
- **Memory-safe aports replacement**: supports the **same APKBUILD format**, can **bootstrap the distribution**, **isolated builds**, **LLVM with maximum static linking** — the cports approach (Chimera). Includes **independent per-package code tests** (know the weaknesses of every shipped package) and **published-CVE integration**. Blocked on: donations → infrastructure + maintainers.
- **apk ported to Go**: eliminate one of the last critical non-memory-safe components of the system.
- **Desktop**: remove **GTK** (not memory safe) from the stack; eliminate every piece of software that drags **heavy dependencies from other desktop environments**.
- **Smart TV desktop**: a FeralOS desktop for smart TVs.

## Out of scope

- **Containers** (Alpine Linux is the answer there).
- Kernel tuning / FPS chasing as a feature.
- Full desktop environments as shipped products (KDE/COSMIC ship only as installable conveniences, never the FeralOS experience).
- Security that is only claimed in the README.

## Metrics

| Metric | Target | How |
|--------|--------|-----|
| Memory-safe share of the stack | Maximize, continuously | Replacement table (`docs/intent/vision.md`, `CONTEXT.md`) — revisited every time a new safe alternative appears |
| Security | Conform to security standards; observability OOTB | **Open TODO** — hardening exists, formal conformance + observability stack pending |
| Perceived speed | Software never hangs; fast boot | dinit parallel boot, TPM unlock, niri/greetd; harness boot-time checks |
| Full software coverage | Everything installable, nothing broken by default | apk (Alpine) + flatpak + nix; remove-without-breaking policy |
| Verification | Everything testable before shipping | QEMU harness (118+ checks, all DEs), 30 fixtures, per-commit docs/static gates |
| Install | TUI + headless + unattended (Ansible) | `variables.yaml` unattended mode; harness = automated install proof |

## References

- `README.md` (user-facing framing) · `ROADMAP.md` (phases + future) · `docs/intent/vision.md` (memory-safe replacement table)
- `docs/intent/spec-distro.md` (the distribution specification) · `CONTEXT.md` (vocabulary)
- Rules this goal produced: `AGENTS.md#Activatable Feature Confinement`, `AGENTS.md#D-Bus Activation Rule (ADR-010)`, `AGENTS.md#aports Source Pinning`
