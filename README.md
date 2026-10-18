# FeralOS

**Experimental Linux distribution based on Alpine Linux.**

FeralOS is not another clone. It's a different approach: take Alpine's solid foundation (musl, apk) and rebuild the userland with memory-safe alternatives. The init system is Dinit instead of OpenRC, with the ability to choose between both. The goal is a secure, modern distribution that's actually good for desktop use — something Alpine has always struggled with.

## Getting Started

1. Download a Linux Mint live ISO
2. Write to USB and boot
3. Open a terminal and run:

```bash
wget https://raw.githubusercontent.com/skea999/feralos/main/setup.sh
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
- Shipping **niriWayle** as the flagship desktop: niri compositor + Wayle shell, Veila lock screen, Raffi launcher, oo7 secrets — see below
- Also installable for convenience: **niri** (DMS shell), **KDE Plasma**, **COSMIC**
- Remote access via **wayvnc** (user dinit, loopback), firewall via firewalld

**niriWayle is the true FeralOS experience.** Choose `desktopEnvironment: niriWayle` for the complete desktop: niri with the Wayle shell (bar, notifications, OSD, wallpaper), Veila lock screen, Raffi launcher and oo7 secrets with automatic keyring unlock at login. It is the only desktop environment developed and verified as a full daily-driver experience — the other environments ship for user convenience and will never be as ready-to-use for every need as niriWayle is.

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

## Project Status

**Experimental.** FeralOS is in active development. The installer works and passes all automated tests (up to 102 checks per fixture via the QEMU harness; the flagship niriWayle fixture runs 102/102), but expect rough edges on real hardware. Everything needs more testing on real hardware before production use.

### Installer Status

- **btrfsStandard**: Working
- **btrfsStandardServer**: Working
- **btrfsRaid0**: Working
- **btrfsStandardUnencrypted**: Working
- **Desktop environments**: **niriWayle** (flagship/recommended), niri, KDE Plasma, COSMIC — Working
- **Secure Boot**: Working (signed UKI)
- **TPM**: Working (automatic LUKS unlock via Clevis)
- **Nix**: Working (package manager for software not in Alpine repos)
- **Podman + Webmin**: Working (containers + web admin UI)

### FeralOS Package Repository

[feralos-aports](https://github.com/skea999/feralos-aports) builds and publishes **signed APK packages** (abuild → GitHub Pages, CI-verified client installs).

Two distinct goals — don't confuse them:

1. **Today — delta repository (live)**: per rule 0 we use Alpine packages whenever they exist, so our repository hosts **only what Alpine is missing** or what needs FeralOS patches (e.g. `dinit-chimera`; later turnstile + patched polkit).
2. **Future — full repository** (requires maintainers): a complete FeralOS-built package set that would replace the Alpine repositories entirely. **Not started** — rule 0 exists precisely because we are not there yet.

### Dinit Init System — Available

Dinit replaces OpenRC when `enableDinit: true` (the configuration template default): the Chimera boot suite (`dinit-chimera`) and the full `-dinit` service suite are packaged, signed and published in our repository. OpenRC stays installed and enabled either way — the active init is a kernel-cmdline choice (`init=`), so a Dinit install remains OpenRC-bootable as a fallback.

We're looking for contributors, especially people interested in:
- Testing on real hardware
- Dinit boot testing

## Contributing

Open to contributors. Read [CONTRIBUTING.md](CONTRIBUTING.md) for further details.

## Authors

- skea999 (sket999@disroot.org)

## License

ISC License. See [LICENSE](LICENSE) for details.
