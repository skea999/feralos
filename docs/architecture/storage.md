# Storage Layouts

Source: `docs/INSTALLER.md#Storage Layouts` + `AGENTS.md` `EnableSwap`/`swapSize` notes (double docs — AGENTS.md is source of truth).

## Layouts

Selected by `conf.User.DiskLayout` in `hostPhase.Run()` → `hostPhase/disks/*`.

### btrfsStandard (1 disk)

- **Partitioning**: `p1` EFI 512M (vfat, `LABEL=EFI*`), `p2` LUKS2 rest (no swap partition)
- **Btrfs**: single mode on LUKS (`mkfs.btrfs -f -L myroot /dev/mapper/luks-*`)
- **Subvolumes**: 25 — `@`, `@home`, `@opt`, `@root`, `@srv`, `@tmp`, `@var*`, `@usr-local`, `@boot`, `@nix-*`, `@btrbk`, `@swap` (+ NoCoW: `@swap`, `@srv`, `@var-log`, `@var-lib-ex`, etc.)
- **Swap**: swapfile in `@swap` subvolume (`btrfs filesystem mkswapfile`), gated `EnableSwap`. Subvolume always exists; `swapSize` controls file size. `swap` service added to `boot` runlevel only when enabled.
- **EFI**: single `/boot/efi`

### btrfsUnencrypted (1 disk)

- **Partitioning**: `p1` EFI 512M (vfat, `LABEL=EFI*`), `p2` btrfs root remainder (no LUKS)
- **Btrfs**: single mode (`mkfs.btrfs -f -L myroot /dev/sda2`)
- **Subvolumes**: same set as btrfsStandard
- **Swap**: swapfile in `@swap` subvolume (`btrfs filesystem mkswapfile`), gated `EnableSwap`. Same behavior as btrfsStandard.
- **EFI**: single `/boot/efi`
- **Kernel cmdline**: `root=UUID=<btrfs-UUID> rootfstype=btrfs rootflags=subvol=@ ...` (no `rd.luks.name`, no `device=/dev/mapper/*`)
- **Modules**: no `dm_mod` or `dm_crypt`. Loads `btrfs`, `efivarfs`, `nvme`, `zstd`, `crc32c_cryptoapi`.

### btrfsRaid0 (2 disks)

- **Partitioning**: both disks same layout
  - `EnableSwap=true`: `p1` EFI 512M, `p2` LUKS swap (`swapSize/2` each), `p3` LUKS root remainder
  - `EnableSwap=false`: `p1` EFI 512M, `p2` LUKS root remainder
- **Btrfs**: pure RAID0 data+metadata (`mkfs.btrfs -d raid0 -m raid0` on LUKS roots)
- **Subvolumes**: same set as standard, `chattr +C` on NoCoW
- **Swap**: 2 LUKS partitions `p2`, keyfile `/etc/dmcrypt/swap.key` + `luksAddKey` + `/etc/conf.d/dmcrypt`, `mkswap -L swap-*`, fstab `/dev/mapper/swap-*` (`pri=2`)
- **EFI**: dual — `/boot/efi` + `/boot/efi2`

### btrfsStandardServer (2 disks)

- **Partitioning**: both disks — `p1` EFI 512M, `p2` LUKS RAID (`firstPartK`), `p3` LUKS swap (`secondPartK`), `p4` btrfs data remainder
- **Btrfs**: RAID1 data+metadata on LUKS `p2` (`mkfs.btrfs -d raid1 -m raid1`)
- **Subvolumes**: 21 on RAID1 (`@swap` always present)
- **Swap**: always LUKS — `p3` on both disks, same keyfile/dmcrypt path as raid0. `swap` service always in `boot` runlevel.
- **Data**: `p4` plain btrfs `LABEL=data-*` mounted `/data/data-*`
- **EFI**: dual — `/boot/efi` + `/boot/efi2`

## Common

- **LUKS**: LUKS2 on RAID and swap partitions. Root via `rd.luks.name=UUID=luks-UUID` cmdline + booster. Swap keyfile `/etc/dmcrypt/swap.key` (`dd urandom 2K`, `400`).
- **Btrfs**: `LABEL=myroot`. Compression per-subvolume (`compress=zstd:3` or `compress=none` + `nodatacow` for NoCoW via `chattr +C` on dir). `docs/INSTALLER.md` notes: `compress=` mount opt is FS-wide — first mount wins, so per-subvolume property is used.
- **Kernel cmdline**: `root=UUID=<btrfs-UUID> rootfstype=btrfs rootflags=subvol=@ ...` (+ `device=/dev/mapper/luks-*` for RAID), written to `/etc/kernel/cmdline` for UKI.
- **Modules**: `dm_mod`, `dm_crypt`, `crc32c_cryptoapi`, `zstd`, `btrfs`, `efivarfs`, `nvme`. (`btrfsUnencrypted` omits `dm_mod`/`dm_crypt`.)

## EnableSwap / swapSize

- `EnableSwap` (bool): gates swap creation. `btrfsStandard`, `btrfsUnencrypted`, and `btrfsRaid0` respect it; `btrfsStandardServer` always creates LUKS swap.
- `swapSize` (string): `auto` = 2×RAM via `utils/swap.ResolveSwapSizeMiB` (`src/utils/swap/swap.go`), or explicit `8G`/`4096M`/`K`/`T`/`P`. `btrfsRaid0` splits total across 2 partitions.
- `utils/swap` handles parsing, whitespace, unit conversion. `fstab_vs_mount` harness check validates result.

## fstab

Written by `hostPhase/disks/*/fstab.go` via `shell.ExecuteCmd`:

- Btrfs subvolumes: `LABEL=myroot  <path>  btrfs  <opts,subvol=@name>  0 0` (`nodatacow` for NoCoW)
- EFI: `LABEL=EFI*  /boot/efi[v2]  vfat  rw,noatime,...  0 2`
- Swap: `btrfsStandard`/`btrfsUnencrypted` → `/swap/swapfile  none  swap  defaults  0 0` (when enabled); `btrfsRaid0`/`Server` → `/dev/mapper/swap-*  none  swap  defaults,sw,pri=2  0 0` (busybox `swapon -a` needs mapper path, not LABEL)
- `btrfsStandardServer` also: `LABEL=data-*  /data/data-*  btrfs  defaults  0 0`

## References

- `AGENTS.md#Architecture Principle` and `AGENTS.md#EnableSwap` notes
- `docs/INSTALLER.md#Disk Layouts`, `#LUKS Encryption`, `#Key Components`
- `src/hostPhase/disks/btrfsStandard/partitioning.go:18`, `fstab.go:10`
- `src/hostPhase/disks/btrfsRaid0/partitioning.go:18`, `fstab.go:10`
- `src/hostPhase/disks/btrfsStandardServer/partitioning.go:18`, `fstab.go:10`
- `src/utils/swap/swap.go`
