# Fixtures — variables.yaml templates

Curated library covering all `variables.yaml` scenarios. Use with harness `-f`:

```bash
./skea-harness run -f tests/fixtures/variables/btrfsRaid0-minimal.yaml
./skea-harness run -f tests/fixtures/variables/btrfsRaid0-noswap.yaml
./skea-harness run -f tests/fixtures/variables/btrfsStandard-swap-explicit-8G.yaml
ls tests/fixtures/variables/
```

Harness loads via `internal/service/service.go:LoadVariablesFile` (`gopkg.in/yaml.v3`). Installer loads via `src/utils/config/config.go:LoadConfig`. Both accept absolute path via `-f` / `-config`.

## Matrix (19 templates)

| # | Template | diskLayout | enableSwap | swapSize | desktop | secureboot | TPM | hardening | zswap | nix | podman | webmin | Purpose |
|---|----------|------------|------------|----------|---------|------------|-----|-----------|-------|-----|--------|--------|---------|
| 1 | `btrfs_standard_minimal.yaml` | btrfsStandard | true | auto (2×RAM) | none | false | false | false | false | false | false | false | Baseline (mirrors `src/variables/variables.yaml`) |
| 2 | `btrfs_standard_noswap.yaml` | btrfsStandard | false | auto | none | false | false | false | false | false | false | false | @swap NoCoW without swapfile, `services_status` without swap |
| 3 | `btrfs_standard_swap_explicit_8g.yaml` | btrfsStandard | true | 8g | none | false | false | false | false | false | false | false | Parse `8g→8192`, `mkswapfile --size 8192M` |
| 4 | `btrfs_standard_swap_explicit_4096m.yaml` | btrfsStandard | true | 4096m | none | false | false | false | false | false | false | false | Parse `m` unit |
| 5 | `btrfs_standard_server_minimal.yaml` | btrfsStandardServer | true | auto | none | false | false | false | false | false | false | false | Server always LUKS swap (2 partitions, `minDiskSize/4`) |
| 6 | `btrfs_raid0_minimal.yaml` | btrfsRaid0 | true | auto | none | false | false | false | false | false | false | false | RAID0 p2 swap p3 root, `hasHealthyBtrfsRaid0Layout` |
| 7 | `btrfs_raid0_noswap.yaml` | btrfsRaid0 | false | auto | none | false | false | false | false | false | false | false | **Gap critical**: `rootPart=2` fdisk 2 partitions, no dmcrypt |
| 8 | `btrfs_raid0_swap_explicit_8g.yaml` | btrfsRaid0 | true | 8g | none | false | false | false | false | false | false | false | `swapSizeK=4194304K` per p2 |
| 9 | `btrfs_raid0_full.yaml` | btrfsRaid0 | true | auto | niri | true | true | true | true | true | false | false | Full stack: clevis 2 bindings, virtio-gpu, zswap |
| 10 | `btrfs_standard_secureboot_only.yaml` | btrfsStandard | true | auto | none | true | false | false | false | false | false | false | Isolate SecureBoot |
| 11 | `btrfs_standard_tpm_only.yaml` | btrfsStandard | true | auto | none | false | true | false | false | false | false | false | Isolate TPM |
| 12 | `btrfs_standard_hardening_only.yaml` | btrfsStandard | true | auto | none | false | false | true | false | false | false | false | Isolate hardening |
| 13 | `btrfs_standard_niri.yaml` | btrfsStandard | true | auto | niri | false | false | false | false | true | false | false | Desktop niri (auto-nix) |
| 14 | `full_stack.yaml` | btrfsStandard | true | auto | none | true | true | true | true | true | true | true | All true (replaces `tests/variables-full.yaml`) |
| 15 | `btrfs_unencrypted_minimal.yaml` | btrfsUnencrypted | true | auto | none | false | false | false | false | false | false | false | No LUKS, VMs/testing, direct btrfs on partition 2 |
| 16 | `btrfs_raid0_full_stack.yaml` | btrfsRaid0 | true | auto | none | true | true | true | true | true | true | true | RAID0 full stack with podman + webmin |
| 17 | `btrfs_standard_server_full_stack.yaml` | btrfsStandardServer | true | auto | none | true | true | true | true | true | true | true | Server full stack with podman + webmin |
| 18 | `btrfs_unencrypted_full_stack.yaml` | btrfsUnencrypted | true | auto | none | false | false | true | true | true | true | true | Unencrypted full stack (no secureboot/tpm) |
| 19 | `btrfs_standard_secureboot_tpm.yaml` | btrfsStandard | true | auto | none | true | true | false | false | false | false | false | SecureBoot + TPM combined |

## Tiering (CI cost)

- **Tier1 (CI gate, ~2h):** 1,2,6,7,15 — minimal + noswap per layout
- **Tier2 (nightly, ~4h):** 3,8,10,11,12
- **Tier3 (manual):** 4,5,9,13,14

## Conventions

- Base derived from `src/variables/variables.yaml` (53 lines). Keep `disks`, `userName`, `password`, `hostname`, `mirror`, `extraPackages` identical unless testing them.
- `swapSize` total split across partitions (raid0 p2 = `swapSize/2`). `auto` = `2× MemTotal` via `utils/swap.ResolveSwapSizeMiB`.
- `btrfsStandardServer` ignores `swapSize`/`enableSwap` gating (always 2 LUKS swap, mandatory by design).
- Naming `snake_case` matching `diskLayout_feature`.

## Legacy

Old `tests/harness/variables-*.yaml` kept as deprecated shims (symlink or header `DEPRECATED: use tests/fixtures/variables/`). `tests/variables-full.yaml` deprecated → `full_stack.yaml`.

## Validation

```bash
cd src && go vet ./...
cd src && go test ./utils/swap -v
for f in tests/fixtures/variables/*.yaml; do echo "== $f"; yq eval '.' "$f" >/dev/null && echo ok; done
```
