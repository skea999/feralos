# FeralOS

**Experimental Linux distribution based on Alpine Linux.**

FeralOS is not another clone. It's a different approach: take Alpine's solid foundation (musl, apk) and rebuild the userland with memory-safe alternatives. The init system will be Dinit instead of OpenRC, with the ability to choose between both. The goal is a secure, modern distribution that's actually good for desktop use — something Alpine has always struggled with.

## What Makes FeralOS Different

### Memory Safety First

Traditional Linux distributions ship thousands of lines of C code in core utilities. C is the root cause of entire classes of vulnerabilities: buffer overflows, use-after-free, format string attacks. FeralOS replaces the most attack-exposed components with memory-safe alternatives written in Rust and Go.

| Traditional (C) | FeralOS Alternative | Language |
|-----------------|---------------------|----------|
| coreutils | uutils-coreutils | Rust |
| vim/nano | micro | Go |
| sudo | sudo-rs | Rust |
| rsyslog/logrotate | vector | Rust |
| bash/zsh | fish | Rust |
| mkinitfs | booster | Go |
| grub | signed UKI (sbctl) | — |

This isn't about rewriting everything. It's about replacing the components that matter most for security and reliability. As the ecosystem grows, more programs will be replaced.

### Runtime Efficiency Over Minimal Footprint

Alpine Linux is known for being lightweight. FeralOS doesn't chase the smallest image size. Instead, it focuses on **runtime efficiency** — doing more with less at execution time.

**Example: initramfs generation**

Alpine uses mkinitfs, which loads a broad set of modules "just in case." FeralOS uses **booster**, which analyzes your system and loads only the modules you actually need. The result: faster boot, less memory usage at runtime, smaller attack surface.

This philosophy applies across the stack: fewer things running, fewer things loaded, fewer things that can break or be exploited.

### musl + Modern Tooling

FeralOS inherits Alpine's musl libc — smaller, simpler, more auditable than glibc. Combined with memory-safe userland tools and modern components (booster for initramfs, signed UKI for Secure Boot, LUKS2 for encryption), the result is a system that's both lean and hardened.

FeralOS is not immutable — the system is fully modifiable. However, it's designed around a recommended approach: core system components are installed via apk to benefit from musl's security and Alpine's minimal design, while everything else is installed via Nix. This gives us the best of both worlds: a hardened, musl-based core with full glibc compatibility for user applications through Nix.

## Use Cases

### Desktop

FeralOS is designed to be a real desktop distribution. Alpine has always been server-focused, with poor support for desktop environments and daily use. FeralOS fixes this by:

- Replacing coreutils with uutils (Rust, no more `ls` segfaults)
- Using fish as default shell (Rust, modern, user-friendly)
- Using micro as editor (Go, intuitive, fast)
- Integrating niri (Wayland compositor) + wayvnc (VNC server)

The `btrfsStandard` layout is ideal for desktop machines: single disk, Btrfs with subvolumes, LUKS2 encryption, swapfile on dedicated subvolume.

### Virtual Machines

For virtual machines, encryption is often unnecessary and adds overhead. The `btrfsStandardUnencrypted` layout provides the same Btrfs subvolume structure without LUKS2 encryption, making it ideal for VMs where the hypervisor handles security at the host level.

Layout: single virtual disk, Btrfs with subvolumes, no encryption, swapfile on dedicated subvolume.

### Virtualization Host

FeralOS works as a KVM/QEMU hypervisor. The `btrfsStandardServer` layout provides:

- RAID1 for OS disks (redundancy)
- Btrfs with subvolumes for snapshots
- Incus pre-installed for container/VM management

For production environments, we recommend a 6-disk configuration:

```
Disk 1-2: OS (RAID1, btrfsStandardServer)
Disk 3-4: Incus storage (RAID1, dedicated)
Disk 5-6: LINSTOR/Pirateus + ZFS (RAID1, distributed storage)
```

Stack: **k3s** + **LINSTOR/Pirateus** as distributed storage backend for **Incus**, with **ZFS** as the underlying storage for Pirateus. This gives you a hyperconverged infrastructure that competes with Proxmox and VMware, but with a modern, secure foundation.

## Prerequisites

### btrfsStandard (Desktop)
- 1 disk, 8 GB minimum
- UEFI system (for Secure Boot)
- Network connection (for package installation)

### btrfsStandardUnencrypted (Virtual Machines)
- 1 virtual disk, 8 GB minimum
- Network connection (for package installation)

### btrfsStandardServer (Virtualization Host)
- 2 disks, 8 GB minimum each
- UEFI system (for Secure Boot)
- Network connection (for package installation)

### btrfsRaid0 (High Performance)
- 2 disks, 8 GB minimum each
- UEFI system (for Secure Boot)
- Network connection (for package installation)

## Getting Started

1. Download a Linux Mint live ISO
2. Write to USB and boot
3. Open a terminal and run:

```bash
wget https://gitea.com/sketa999/skeaAlpineInstaller/raw/branch/main/setup.sh
sudo bash setup.sh
```

The script will automatically clone the repository, build the installer, and run it.

### Options

```bash
# Interactive (asks for configuration)
sudo bash setup.sh

# Non-interactive (use existing config)
sudo bash setup.sh -config variables.yaml
```

### Using the Generator

To create a custom `variables.yaml` configuration:

```bash
sudo bash setup.sh
# Then run the generator inside the installer directory
./feralos_create_config
```

## Project Status

**Experimental.** FeralOS is in active development. The installer works and passes all automated tests (22/22 checks via QEMU harness), but expect rough edges on real hardware. Everything needs more testing on real hardware before production use.

### Installer Status

- **btrfsStandard**: Working
- **btrfsStandardServer**: Working
- **btrfsRaid0**: Planned
- **btrfsStandardUnencrypted**: Planned
- **Secure Boot**: Working (signed UKI via sbctl)
- **TPM**: Working (automatic LUKS unlock via Clevis)
- **Nix**: Working (package manager for software not in Alpine repos)

Currently uses official Alpine repositories. In the future, if the project gains maintainers, FeralOS may host its own repositories with custom packages and configurations.

We're looking for contributors, especially people interested in:
- Testing on real hardware

## Contributing

Open to contributors. Read [CONTRIBUTING.md](CONTRIBUTING.md) for further details.

## Authors

- skea999 (skea999@cryptolab.net)

## License

ISC License. See [LICENSE](LICENSE) for details.
