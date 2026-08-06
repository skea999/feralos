# First Boot Programs

Standalone Go programs for post-installation setup on real hardware.

## Programs

| Program | Purpose |
|---------|---------|
| `secureboot` | Secure Boot enrollment via sbctl |
| `tpm` | Clevis TPM2 binding for LUKS auto-unlock |

## Building

From `src/`:

```bash
# Build secureboot
go build -o /usr/local/bin/feralos-secureboot ./firstBoot/secureboot/

# Build tpm
go build -o /usr/local/bin/feralos-tpm ./firstBoot/tpm/
```

## Usage on Real Hardware

### Prerequisites

- FeralOS installed on target system
- Boot into the installed system (first boot)
- Root access

### Step 1: Secure Boot Enrollment

Run this on **first boot** (before enabling Secure Boot in BIOS):

```bash
# Copy binary to target
scp feralos-secureboot root@<target-ip>:/root/

# Run on target
ssh root@<target-ip>
/root/feralos-secureboot
```

**What it does:**
1. Installs `sbctl` temporarily
2. Mounts `efivarfs` if needed
3. Imports EFI certificates from `/etc/uefi-keys/`
4. Enrolls keys via `sbctl enroll-keys -m`
5. Removes `sbctl` after completion

**After enrollment:**
1. Reboot into BIOS
2. Enable Secure Boot: `BIOS → Security → Secure Boot → Enable: ON`
3. Set Secure Boot Mode: `Custom`
4. Save & Exit → Reboot

### Step 2: TPM Auto-Unlock Setup

Run this on **second boot** (after Secure Boot is enabled):

```bash
# Copy binary to target
scp feralos-tpm root@<target-ip>:/root/

# Run on target
ssh root@<target-ip>
/root/feralos-tpm
```

**What it does:**
1. Checks for TPM at `/dev/tpmrm0`
2. Reads LUKS UUIDs from `/etc/kernel/cmdline`
3. Adds `@edge` repo temporarily
4. Installs `clevis@edge`
5. Binds each LUKS device to TPM2 (empty PCR policy)
6. Removes temporary keyfile if present
7. Rebuilds initramfs and signed UKI

**After TPM binding:**
1. Reboot
2. System should auto-unlock LUKS without passphrase
3. If TPM fails, system falls back to passphrase prompt

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SKEA_LUKS_PASSWORD` | LUKS passphrase for TPM binding (avoids interactive prompt) |

Example:
```bash
SKEA_LUKS_PASSWORD="your-password" /root/feralos-tpm
```

## Troubleshooting

### Secure Boot

**Problem:** `sbctl enroll-keys` fails
**Solution:** Clear Secure Boot keys in BIOS first:
1. Reboot → BIOS
2. Security → Secure Boot
3. Custom Mode → Delete All Keys
4. Save & Exit → Reboot → Run `feralos-secureboot` again

**Problem:** System doesn't boot after enabling Secure Boot
**Solution:** 
1. Reboot → BIOS
2. Disable Secure Boot
3. Boot into system
4. Run `feralos-secureboot` again to verify enrollment
5. Re-enable Secure Boot

### TPM

**Problem:** `TPM not found at /dev/tpmrm0`
**Solution:** Ensure TPM is enabled in BIOS:
1. Reboot → BIOS
2. Security → TPM Configuration
3. Enable TPM Device
4. Save & Exit → Reboot

**Problem:** LUKS auto-unlock fails on boot
**Solution:**
1. Enter passphrase when prompted
2. Login and check TPM bindings:
   ```bash
   clevis luks list -d /dev/nvme0n1p3
   ```
3. If no `tpm2` binding, re-run `/root/feralos-tpm`

## Notes

- **PCR Policy:** Uses empty `{}` (TPM identity only, no PCR policy)
  - More reliable across reboots
  - PCR7 mismatch between bind time and boot time causes failures
- **Kernel:** Assumes `vmlinuz-lts` and `booster-lts.img`
- **EFI Keys:** Must exist at `/etc/uefi-keys/` (PK, KEK, db)

---

*See `AGENTS.md` for project context and `docs/USAGE.md` for general usage.*
