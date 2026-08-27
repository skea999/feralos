# Vision — FeralOS

Source: `CONTEXT.md#Project Vision` + `PROJECT.md#Vision` (double docs — `AGENTS.md` is source of truth, this copy is reference).

## FeralOS

**FeralOS** is a new Linux distribution based on **Alpine Linux**, focused on **memory safety** and **security**.

- **Base**: Alpine (musl, apk), keeps Alpine package repositories.
- **Memory safety**: Replace traditional C components with modern, memory-safe alternatives in Rust/Go where possible.
- **Secondary packages**: **Nix** as secondary package manager for software not in Alpine repos or requiring glibc (without polluting base system).
- **Future init**: **Dinit** replaces OpenRC (better dependencies, parallel startup) after base installer stable.
- **Future repo**: **Custom Alpine-style repository** for FeralOS-specific services and packages not upstream.

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

> Some C components remain when no memory-safe alternative exists or component is minimal/audited. See `CONTEXT.md#Memory-Safe Alternatives` + `PROJECT.md#Memory-Safe Alternatives`.

## Distribution Goal

Standard core, pluggable data — installer is first step, then Dinit + custom repo → complete bootable distribution. `booster` replaces `mkinitfs`, signed **UKI** replaces GRUB, **LUKS2** on RAID/Swap, **Btrfs RAID1** on LUKS.

## References

- `CONTEXT.md#Project Vision`
- `PROJECT.md#Vision` + `PROJECT.md#Architecture Principle`
- `AGENTS.md#Architecture Principle` + `AGENTS.md#Roadmap`
- `docs/PROJECT.md` (copy of vision + architecture)
- `docs/architecture/overview.md`
