#!/bin/sh
# TPM auto-unlock enrollment (Clevis TPM2)
# Run this AFTER Secure Boot enrollment is complete (1-secure-boot-sbctl.sh or 1-secure-boot.sh).
# Run this after first boot to enable TPM auto-unlock of LUKS root volumes.

set -e

echo "=== FeralOS — TPM Enrollment ==="

# Check if TPM is available
if [ ! -e /dev/tpmrm0 ]; then
    echo "Warning: TPM not found at /dev/tpmrm0"
    echo "TPM auto-unlock will not be configured."
    exit 0
fi

echo "TPM detected, binding LUKS volumes..."

# Get LUKS UUIDs from kernel cmdline rd.luks.name= parameters
cryptRootUUIDs=""
if [ -f /etc/kernel/cmdline ]; then
    cryptRootUUIDs=$(tr ' ' '\n' < /etc/kernel/cmdline | sed -n 's/^rd\.luks\.name=\([^ =]*\)=.*/\1/p')
fi

if [ -z "$cryptRootUUIDs" ]; then
    echo "Error: No LUKS UUIDs found in kernel cmdline."
    exit 1
fi

# Check if any device needs binding
needsBinding=no
for uuid in $cryptRootUUIDs; do
    device=$(findfs "UUID=$uuid" 2>/dev/null)
    [ -b "$device" ] || continue
    if ! clevis luks list -d "$device" 2>/dev/null | grep -q "tpm2"; then
        needsBinding=yes
        break
    fi
done

if [ "$needsBinding" = "no" ]; then
    echo "All root LUKS devices already have Clevis TPM2 bindings."
    exit 0
fi

# Get temporary key file (created during install) or prompt for password
passFile="/etc/cryptsetup-tpm2-key"
temporaryPassFile=""
if [ ! -f "$passFile" ]; then
    temporaryPassFile=$(mktemp)
    chmod 600 "$temporaryPassFile"
    if [ -n "${SKEA_LUKS_PASSWORD:-}" ]; then
        printf '%s' "$SKEA_LUKS_PASSWORD" > "$temporaryPassFile"
    else
        printf 'Enter existing LUKS passphrase: ' >&2
        stty -echo
        IFS= read -r luksPassword
        stty echo
        printf '\n' >&2
        printf '%s' "$luksPassword" > "$temporaryPassFile"
        unset luksPassword
    fi
    passFile="$temporaryPassFile"
fi

# Derive @edge repo URLs from the current main repository mirror (not hardcoded).
# Strips the last two path components (e.g. /v3.23/main or /latest-stable/main).
MIRROR_BASE=$(grep -m1 -E '^https?://' /etc/apk/repositories | head -1 | sed -E 's|/[^/]+/[^/]+$||')
echo "@edge ${MIRROR_BASE}/edge/main" >> /etc/apk/repositories
echo "@edge ${MIRROR_BASE}/edge/community" >> /etc/apk/repositories
echo "@edge ${MIRROR_BASE}/edge/testing" >> /etc/apk/repositories
cat /etc/apk/repositories
apk update
apk add --no-cache clevis@edge
sed -i "/^clevis@edge$/d" /etc/apk/world
cat /etc/apk/world
sed -i "/^@edge/d" /etc/apk/repositories
cat /etc/apk/repositories


# Bind each LUKS device to TPM
for uuid in $cryptRootUUIDs; do
    device=$(findfs "UUID=$uuid" 2>/dev/null)
    [ -b "$device" ] || continue

    if clevis luks list -d "$device" 2>/dev/null | grep -q "tpm2"; then
        echo "$device already has a Clevis TPM2 binding, skipping."
        continue
    fi

    echo "Binding $device to Clevis TPM2..."
    clevis luks bind -y -d "$device" -k "$passFile" tpm2 '{"pcr_ids":"7"}'
done

# Remove temporary key if it was created during install
if [ "$passFile" = "/etc/cryptsetup-tpm2-key" ]; then
    allRemoved=yes
    for uuid in $cryptRootUUIDs; do
        device=$(findfs "UUID=$uuid" 2>/dev/null)
        [ -b "$device" ] || continue
        if ! cryptsetup luksRemoveKey --key-file "$passFile" "$device"; then
            echo "Warning: failed to remove temporary Clevis binding key from $device" >&2
            allRemoved=no
        fi
    done

    if [ "$allRemoved" = "yes" ]; then
        rm -f /etc/cryptsetup-tpm2-key
    fi
fi

# Clean up temporary passfile if created
if [ -n "$temporaryPassFile" ] && [ -f "$temporaryPassFile" ]; then
    rm -f "$temporaryPassFile"
fi

# Rebuild initramfs
echo "Rebuilding initramfs..."
alpineKernel=$(ls /lib/modules/ | head -1)
booster build --force --kernel-version "$alpineKernel" /boot/booster-lts.img

# Rebuild signed UKI
echo "Rebuilding signed UKI..."
cmdLine=$(cat /etc/kernel/cmdline)
efi-mkuki -c "$cmdLine" -o /boot/efi/Alpine/linux-lts.efi /boot/vmlinuz-lts /boot/booster-lts.img
sbsign --key /etc/uefi-keys/db.key --cert /etc/uefi-keys/db.crt --output /boot/efi/Alpine/linux-lts.efi /boot/efi/Alpine/linux-lts.efi

# Mirror to fallback ESP
cp -af /boot/efi/Alpine/linux-lts.efi /boot/efi/EFI/BOOT/BOOTX64.EFI

echo ""
echo "=== TPM binding complete ==="
echo "Reboot to test automatic LUKS unlock via Clevis TPM2."
echo ""
echo "If TPM fails, the system will fall back to passphrase."
