# Vision — FeralOS

Source: `docs/intent/goals.md` (mission + objectives) + `CONTEXT.md#Project Vision`. This file holds the vision statement + the memory-safe replacement table.

## FeralOS

**FeralOS** is a new Linux distribution based on **Alpine Linux**, focused on **memory safety** and **security**.

- **Base**: Alpine (musl, apk), keeps Alpine package repositories.
- **Memory safety**: Replace traditional C components with modern, memory-safe alternatives in Rust/Go where possible.
- **Secondary packages**: **Nix** as secondary package manager for software not in Alpine repos or requiring glibc (without polluting base system).
- **Init**: **dinit** replaces OpenRC (better dependencies, parallel startup — shipped, `enableDinit` toggle, OpenRC co-installed as fallback).
- **Repo**: **feralos-aports** — custom signed Alpine-style repository for FeralOS services and packages (LIVE on GitHub Pages, CI client-verified).
- **Security**: the distribution is security-oriented for real — hardening by default, supervised services, formal standards conformance + enterprise observability out of the box are open TODOs (see `docs/intent/goals.md#Objectives`).
- **Perceived speed**: software that never hangs — dinit parallel boot, TPM auto-unlock even with full-disk encryption, greetd, niri. Kernel tuning / FPS chasing is explicitly not a goal.

## Memory-Safe Replacements

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|---------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | sudo-rs | Rust |
| rsyslog/logrotate | vector | Rust |
| bash/zsh | fish | Rust |
| lxd | incus | Go |
| systemd/openrc | dinit (future) | C++ |
| grep (GNU) | ripgrep (optional) | Rust |
| find (GNU) | fd (optional) | Rust |

> Some C components remain when no memory-safe alternative exists or the component is minimal/audited. See `CONTEXT.md#Memory-Safe Alternatives`, `docs/memory_safe_alternatives.md` (detailed inventory) and `CONTEXT.md`.

## Distribution Goal

A complete, refined distribution for **desktop, VM, hypervisor and TV** — everything except containers. Standard core, pluggable data: the installer is the starting point (everything testable before shipping), dinit + the custom repo carry the system, `booster` replaces `mkinitfs`, signed **UKI** replaces GRUB, **LUKS2** on RAID/Swap, **Btrfs RAID1** on LUKS.

## References

- `docs/intent/goals.md` — mission + objectives (single source for goals)
- `docs/intent/spec-distro.md` — the distribution specification
- `CONTEXT.md#Project Vision` · `AGENTS.md#Architecture Principle` + `AGENTS.md#Roadmap`
- `docs/architecture/overview.md`
