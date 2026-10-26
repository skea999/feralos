# FeralOS Package Inventory — Memory Safety Analysis

**Last updated**: 2026-08-06 (from `apk info` output, 598 packages)
**Summary table**: the condensed memory-safe alternatives table lives in `CONTEXT.md` ("Memory-Safe Alternatives") — this file is the detailed per-package inventory behind it.

## Summary

FeralOS replaces traditional C components with memory-safe alternatives where possible. This document tracks every installed package, its language, and replacement status.

## Memory-Safe Replacements (Rust/Go)

| Package | Language | Replaces | Notes |
|---------|----------|----------|-------|
| uutils | Rust | busybox (C) | Core utilities |
| uutils-coreutils | Rust | coreutils (C) | GNU coreutils replacement |
| uutils-coreutils-hostname | Rust | hostname (C) | |
| sudo-rs | Rust | sudo (C) | Memory-safe sudo |
| doas | C (minimal) | sudo (C) | OpenBSD doas, small TCB |
| vector | Rust | rsyslog/logrotate (C) | Logging pipeline |
| micro | Go | vim/nano (C) | Text editor |
| ripgrep | Rust | grep (GNU C) | Fast grep |
| fd | Rust | find (GNU C) | Fast find |
| bat | Rust | cat (C) | Cat with syntax highlighting |
| dust | Rust | du (C) | Disk usage |
| bottom | Rust | top/htop (C) | Process monitor |
| procs | Rust | ps (C) | Process viewer |
| bandwhich | Rust | iftop/nethogs (C) | Bandwidth monitor |
| gping | Rust | ping (C) | Graphical ping |
| sd | Rust | sed (C) | Stream editor |
| viddy | Rust | watch (C) | Command watcher |
| fish | Rust | bash/zsh (C/C++) | Interactive shell |
| incus | Go | lxd (C) | Container/VM manager |

## Core System Packages

| Package | Language | Notes |
|---------|----------|-------|
| alpine-base | Shell/C | Alpine meta-package |
| alpine-baselayout | C | Filesystem layout |
| alpine-baselayout-data | C | Layout data |
| alpine-conf | Shell | Alpine configuration |
| alpine-keys | C | Package signing keys |
| alpine-release | C | Release info |
| apk-tools | C | Package manager |
| busybox | C | Minimal utilities (being replaced by uutils) |
| busybox-binsh | C | Default shell |
| busybox-ifupdown | C | Network scripts |
| busybox-mdev-openrc | C | Device manager |
| busybox-openrc | C | Init scripts |
| busybox-suid | C | SUID utilities |
| musl | C | C library (minimal, audited) |
| musl-utils | C | Musl utilities |
| musl-locales | C | Locale support |
| musl-locales-lang | C | Locale data |
| musl-fts | C | File tree traversal |
| musl-obstack | C | Object stack |
| openrc | C | Init system (dinit now shipped and selectable via `enableDinit`; OpenRC remains the fallback) |
| openrc-user | C | User services |
| mdev-conf | C | mdev configuration |
| scanelf | C | ELF scanner |

## Shell & Terminal

| Package | Language | Notes |
|---------|----------|-------|
| fish | Rust | Friendly interactive shell (rewritten in Rust) |
| fish-tools | Rust | Fish utilities |
| less | C | Pager (dependency of bat; replaced by the `less-dummy` wrapper package v999 since 2026-10-09 — POSIX-sh wrapper dumping via `bat --color=always --paging=never`; the older symlink→bat approach is obsolete) |
| readline | C | Line editing |
| ncurses-terminfo | C | Terminal info |
| ncurses-terminfo-base | C | Base terminfo |

## Filesystem & Storage

| Package | Language | Notes |
|---------|----------|-------|
| btrfs-progs | C | Btrfs utilities |
| btrfs-progs-openrc | C | Btrfs init scripts |
| btrfs-progs-udev | C | Btrfs udev rules |
| btrbk | Perl | Btrfs backup/snapshot tool |
| cryptsetup | C | LUKS encryption |
| cryptsetup-libs | C | Cryptsetup library |
| cryptsetup-openrc | C | Cryptsetup init scripts |
| device-mapper | C | Device mapper |
| device-mapper-libs | C | DM library |
| device-mapper-udev | C | DM udev rules |
| libfdisk | C | Disk utilities library |
| libblkid | C | Block device ID |
| libmount | C | Mount utilities |
| libsmartcols | C | Column output |
| util-linux-misc | C | Miscellaneous utils |
| uuidgen | C | UUID generator |
| zstd | C | Zstandard compression |
| zstd-libs | C | Zstd library |
| lz4-libs | C | LZ4 compression |
| lzo | C | LZO compression |
| xz | C | XZ compression |
| xz-libs | C | XZ library |
| zlib | C | zlib compression |
| zlib-ng | C | zlib-ng (optimized) |

## Boot & EFI

| Package | Language | Notes |
|---------|----------|-------|
| booster | Go | Initramfs generator |
| efi-mkkeys | Go | EFI key generation |
| efi-mkuki | Go | UKI creation |
| efibootmgr | C | EFI boot manager |
| efitools | C | EFI tools |
| secureboot-hook | C | Secure Boot hook |
| systemd-efistub | C | EFI stub |
| ovmf | C | UEFI firmware |
| efivar-libs | C | EFI variables |

## Networking

| Package | Language | Notes |
|---------|----------|-------|
| openssh | C | SSH server/client |
| openssh-client-common | C | SSH client |
| openssh-client-default | C | SSH client default |
| openssh-keygen | C | SSH key generation |
| openssh-server | C | SSH server |
| openssh-server-common | C | SSH server common |
| openssh-server-common-openrc | C | SSH init scripts |
| openssh-sftp-server | C | SFTP server |
| bridge | C | Network bridge |
| bridge-utils | C | Bridge utilities |
| vlan | C | VLAN support |
| iproute2 | C | Network configuration |
| iproute2-minimal | C | Minimal iproute2 |
| iproute2-ss | C | Socket statistics |
| iproute2-tc | C | Traffic control |
| iptables | C | Firewall |
| iptables-openrc | C | Firewall init scripts |
| dnsmasq | C | DNS/DHCP server |
| dnsmasq-common | C | dnsmasq common |
| dnsmasq-openrc | C | dnsmasq init scripts |
| ethtool | C | Network tool |
| netcat-openbsd | C | Network utility |
| libcurl | C | URL transfer |
| libssh | C | SSH library |
| libssh2 | C | SSH2 library |
| libssl3 | C | SSL/TLS library |
| libcrypto3 | C | Crypto library |
| ca-certificates | C | CA certificates |
| ca-certificates-bundle | C | CA bundle |
| nghttp2-libs | C | HTTP/2 library |
| libpsl | C | Public suffix list |

## Hardware & System

| Package | Language | Notes |
|---------|----------|-------|
| pciutils | C | PCI utilities |
| pciutils-libs | C | PCI library |
| usbutils | C | USB utilities |
| libusb | C | USB library |
| cpufrequtils | C | CPU frequency |
| cpufrequtils-openrc | C | CPU freq init scripts |
| smartmontools | C | SMART monitoring |
| smartmontools-openrc | C | SMART init scripts |
| dmidecode | C | DMI decode |
| lm-sensors | C | Hardware sensors |
| lm-sensors-libs | C | Sensors library |
| hwdata-pci | Data | PCI IDs |
| hwdata-usb | Data | USB IDs |
| kbd | C | Keyboard utilities |
| kbd-bkeymaps | Data | Keymaps |
| kbd-misc | C | Keyboard misc |
| kbd-openrc | C | Keyboard init scripts |
| linux-firmware | Binary | Firmware files |
| linux-firmware-* | Binary | Specific firmware |
| amd-ucode | Binary | AMD microcode |

## Init & Services

| Package | Language | Notes |
|---------|----------|-------|
| chrony | C | NTP client |
| chrony-common | C | Chrony common |
| chrony-openrc | C | Chrony init scripts |
| cronie | C | Cron daemon |
| cronie-openrc | C | Cron init scripts |
| dbus | C | Message bus |
| dbus-daemon-launch-helper | C | D-Bus helper |
| dbus-libs | C | D-Bus library |
| dbus-openrc | C | D-Bus init scripts |
| eudev | C | Device manager |
| eudev-libs | C | eudev library |
| eudev-openrc | C | eudev init scripts |
| udev-init-scripts | C | udev scripts |
| udev-init-scripts-openrc | C | udev init scripts |
| turnstile | C | Session tracker (desktop only) |
| acpid | C | ACPI daemon |
| acpid-openrc | C | ACPI init scripts |
| irqbalance | C | IRQ balancing |
| irqbalance-openrc | C | IRQ init scripts |
| earlyoom | Go (port) | OOM killer |
| earlyoom-openrc | C | OOM init scripts |
| rasdaemon | C | RAS daemon |
| rasdaemon-openrc | C | RAS init scripts |
| lxc | C | Linux containers |
| lxc-libs | C | LXC library |
| lxc-openrc | C | LXC init scripts |

## Memory-Safe Monitoring

| Package | Language | Notes |
|---------|----------|-------|
| vector | Rust | Logging pipeline |
| vector-openrc | C | Vector init scripts |
| bottom | Rust | Process monitor |
| bandwhich | Rust | Network monitor |
| procs | Rust | Process viewer |
| gping | Rust | Graphical ping |
| viddy | Rust | Command watcher |

## Container & Virtualization

| Package | Language | Notes |
|---------|----------|-------|
| qemu | C | QEMU emulator |
| qemu-system-x86_64 | C | QEMU x86_64 |
| qemu-img | C | QEMU disk images |
| qemu-modules | C | QEMU modules |
| incus-feature | Go | Incus containers |
| incus-feature-agent | Go | Incus agent |
| incus-feature-agent-openrc | C | Incus agent init |
| incus-feature-client | Go | Incus client |
| incus-feature-openrc | C | Incus init scripts |
| incus-feature-utils | Go | Incus utilities |
| incus-feature-vm | Go | Incus VM support |

## Graphics & Display (if desktop)

| Package | Language | Notes |
|---------|----------|-------|
| mesa | C | OpenGL implementation |
| mesa-egl | C | EGL library |
| mesa-gbm | C | GBM library |
| mesa-gl | C | OpenGL library |
| mesa-gles | C | OpenGL ES |
| libdrm | C | DRM library |
| libx11 | C | X11 library |
| libxcb | C | XCB library |
| wayland-libs-client | C | Wayland client |
| wayland-libs-cursor | C | Wayland cursor |
| wayland-libs-egl | C | Wayland EGL |
| gtk+3.0 | C | GTK3 toolkit |
| vte3 | C | Terminal emulator |
| pango | C | Text layout |
| cairo | C | 2D graphics |
| harfbuzz | C | Text shaping |
| freetype | C | Font rendering |
| fontconfig | C | Font configuration |
| font-terminus | Data | Terminus font |

## Libraries (Minimal/Audited)

| Package | Language | Notes |
|---------|----------|-------|
| glib | C | GLib utility library |
| icu-libs | C | ICU libraries |
| json-c | C | JSON library |
| jansson | C | JSON library |
| yaml | C | YAML library |
| pcre2 | C | Regex library |
| libxml2 | C | XML library |
| libffi | C | Foreign function interface |
| libstdc++ | C++ | C++ standard library |
| libgcc | C | GCC support library |
| libgomp | C | OpenMP library |
| libunwind | C | Stack unwinding |
| liburing | C | io_uring library |
| libseccomp | C | Seccomp library |
| libcap2 | C | Capabilities library |
| libcap-ng | C | Capabilities library |
| libattr | C | Extended attributes |
| acl | C | Access control lists |
| acl-libs | C | ACL library |
| libbz2 | C | bzip2 library |
| libcom_err | C | Error handling |
| libdb | C | Berkeley DB |
| gdbm | C | GNU DBM |
| sqlite-libs | C | SQLite library |

## Security

| Package | Language | Notes |
|---------|----------|-------|
| openssl | C | TLS/SSL toolkit |
| gpg | C | GNU Privacy Guard |
| gpg-agent | C | GPG agent |
| gpgme | C | GPG Made Easy |
| gnupg-dirmngr | C | Directory manager |
| gnupg-gpgconf | C | GPG config |
| gnupg-keyboxd | C | Keybox daemon |
| gpgsm | C | S/MIME handler |
| libgcrypt | C | Crypto library |
| libgpg-error | C | GPG error library |
| libksba | C | X.509 library |
| libassuan | C | IPC library |
| npth | C | Thread library |
| pinentry | C | PIN entry |
| libtasn1 | C | ASN.1 library |
| nettle | C | Crypto library |
| gmp | C | Multi-precision arithmetic |
| p11-kit | C | PKCS#11 kit |

## Build & Development

| Package | Language | Notes |
|---------|----------|-------|
| binutils | C | Binary utilities |
| sbsigntool | C | Secure Boot signing |
| pkgconf | C | Package config |
| python3 | C | Python interpreter |

## Documentation

| Package | Language | Notes |
|---------|----------|-------|
| man-db | C | Man page viewer |
| man-db-lang | C | Man page translations |
| groff | C | Formatter |

## Remaining C Components (No Memory-Safe Alternative)

These C packages remain because no memory-safe alternative exists or the component is minimal/audited:

| Package | Reason |
|---------|--------|
| musl | C library, minimal, audited |
| busybox | Being replaced by uutils |
| openrc | Future: dinit (C++) |
| btrfs-progs | No Rust alternative |
| cryptsetup | No Rust alternative |
| iptables | No Rust alternative (nftables is kernel) |
| QEMU | No Rust alternative |
| mesa | OpenGL, complex |
| GTK | UI toolkit, complex |
| All firmware | Binary data |

---

## Notes

- **598 total packages** installed
- **19 memory-safe replacements** (Rust/Go) actively replacing C alternatives
- **~200 C libraries** (minimal, audited, or no alternative)
- **~150 firmware packages** (binary data, not code)
- **~50 desktop packages** (optional, complex UIs)
- **~30 container/VM packages** (QEMU/Incus, Incus is Go)

## Replacement Strategy

1. **High priority**: uutils, sudo-rs, vector, micro, fish, ripgrep, fd, bat, bottom, procs, dust, bandwhich, gping, sd, viddy
2. **Medium priority**: chrony → ntpd-rs (when packaged)
3. **Low priority**: BusyBox (partially replaced by uutils)
4. **No replacement**: musl, btrfs-progs, cryptsetup, QEMU, mesa, GTK

## Known Issues

### Package Masking (Resolved — superseded 2026-10-09)
- **Issue**: Alpine apk masking via `/etc/apk/world` conflicts with explicit package dependencies
- **Example**: `bat` depends on `less` — masking `less` prevents `bat` from installing
- **Original resolution**: dummy `less` package (v999) with symlink `/usr/bin/less` → `/usr/bin/bat` — BROKEN in practice: bat re-execs its hardcoded `less -R` pager, hitting itself (`unexpected argument '-R'`).
- **Current resolution**: `less-dummy` package (v999) — regular-file POSIX-sh wrapper that DROPS less flags and dumps once via `bat --color=always --paging=never` (see the Shell & Terminal table). Symlink approach obsolete.

### Vector VRL Syntax Error (Fixed)
- **Issue**: `parse_syslog(.message)` in syslog_parser transform is fallible
- **Fix**: `parse_syslog(.message) ?? {}` fallback (E103 resolved)
- **Status**: Vector fully working — socket on `/run/vector/syslog.sock`, logs in `/var/log/`

### uutils-coreutils-more Conflict (Resolved)
- **Issue**: `uutils-coreutils-more` and `util-linux-misc` both provide `/usr/bin/more`
- **Root cause**: `incus-feature-vm` depends on `util-linux-misc`, which conflicts with uutils more
- **Resolution**: Created empty dummy `util-linux-misc` (v999) via `providePackages` in internal_rules.yaml. Dummy satisfies dependency without file conflicts. `uutils-coreutils-more` provides real `/usr/bin/more`.

### Ripgrep as grep Drop-in (Won't Fix)
- **Issue**: ripgrep (`rg`) doesn't support POSIX BRE (`\+`)
- **Root cause**: `efi-mkuki` uses `grep -q '^[0-9]\+\.[0-9][0-9A-Za-z_.-]*$'` for kernel version validation. `rg` uses Rust regex which doesn't support `\+`.
- **Resolution**: Ripgrep NOT used as grep drop-in. Kept as optional interactive tool only. GNU grep removed — BusyBox grep provides POSIX BRE. Dummy packages only replace `less` → `bat` (true drop-in).
