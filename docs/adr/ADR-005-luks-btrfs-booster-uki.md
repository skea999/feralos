# ADR-005: LUKS2 + Btrfs RAID1 + booster + UKI

- **Status:** Accepted
- **Date:** 2026-08-26
- **Deciders:** FeralOS team

## Context

Need hardened boot with full-disk encryption, mirrored data, fast initramfs, and verified boot. Legacy GRUB + mkinitfs lack UKI/measured boot and per-device encryption flexibility. Sources: `AGENTS.md#Architecture Principle`, `docs/INSTALLER.md#LUKS Encryption` + `#Boot Process`, `docs/architecture/storage.md`, `docs/architecture/firstboot.md`.

## Decision

- **LUKS2** on RAID partitions and swap (btrfsStandardServer always; btrfsStandard/btrfsRaid0 gated `EnableSwap`). Per-device keyfile `/etc/dmcrypt/swap.key` (dd urandom 2K, 0400) + `luksAddKey`, `rd.luks.name=UUID=luks-UUID` cmdline.
- **Btrfs RAID1** (or RAID0/single per layout, `LABEL=myroot`) with 21–25 subvolumes (`@`, `@home`, `@boot`, `@nix-*`, `@swap`...), per-subvolume `compress=zstd:3` / `nodatacow` + `chattr +C`.
- **booster** replaces mkinitfs — builds initramfs with `dm_mod`, `dm_crypt`, `btrfs`, `zstd`, `efivarfs`.
- **UKI** replaces GRUB — `efi-mkuki` + `sbsign`, kernel cmdline at `/etc/kernel/cmdline`, fallback `BOOTX64.EFI`, mirrored to `/boot/efi2` + EFI boot entries. EFI sync hook rsyncs `/boot/efi`→`/boot/efi2`.

## Consequences

- Encrypted swap + data at rest; measured boot via UKI signing; `sbctl` + TPM2 binding via firstBoot binaries (`clevis`, `tpm2-tools`).
- Dual EFI partitions required; booster/UKI toolchain replaces GRUB config entirely.
- Layout-specific fstab uses `/dev/mapper/swap-*` (busybox `swapon -a` needs mapper path).

## References

- `AGENTS.md#Architecture Principle`, `docs/INSTALLER.md#Disk Layouts` + `#Boot Process` + `#Security Features`, `docs/architecture/storage.md`, `src/hostPhase/disks/*/partitioning.go`, `src/chrootPhase/internal/uki.go`
