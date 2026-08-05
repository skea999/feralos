#!/bin/sh
# Secure Boot setup using sbctl (from Alpine wiki: https://wiki.alpinelinux.org/wiki/UEFI_Secure_Boot)
# Replaces the efi-mkkeys + efitools manual enrollment flow with sbctl.

set -e

echo "=== FeralOS — Secure Boot sbctl Setup ==="

# --- 1. Install sbctl ---
echo "Installing sbctl..."
apk add sbctl

# --- 2. Ensure efivarfs is available ---
if ! mountpoint -q /sys/firmware/efi/efivars 2>/dev/null; then
    modprobe efivarfs 2>/dev/null || true
    mount -t efivarfs efivarfs /sys/firmware/efi/efivars 2>/dev/null || true
fi

# --- 3. Check Secure Boot state ---
echo ""
echo "Current Secure Boot status:"
sbctl status || echo "WARNING: sbctl status failed (efivarfs not mounted?)"

# --- 4. Convert existing efi-mkkeys certs to DER and copy to ESP ---
# The DER files can be enrolled via UEFI firmware browser as fallback.
echo ""
echo "Copying DER certificates to ESP..."
mkdir -p /var/lib/sbctl
mkdir -p /var/lib/sbctl/keys
mkdir -p /var/lib/sbctl/keys/PK
mkdir -p /var/lib/sbctl/keys/KEK
mkdir -p /var/lib/sbctl/keys/db
head -c 36 /proc/sys/kernel/random/uuid > /var/lib/sbctl/GUID

if [ -f /etc/uefi-keys/PK.crt ]; then
    if [ ! -f /var/lib/sbctl/keys/PK/PK.pem ]; then
        openssl x509 -in /etc/uefi-keys/PK.crt -out /var/lib/sbctl/keys/PK/PK.pem 2>/dev/null
    fi
    if [ ! -f /var/lib/sbctl/keys/PK/PK.key ]; then
        cp -a /etc/uefi-keys/PK.key /var/lib/sbctl/keys/PK/PK.key 2>/dev/null || true
    fi
    if [ ! -f /var/lib/sbctl/keys/KEK/KEK.pem ]; then
        openssl x509 -in /etc/uefi-keys/KEK.crt -out /var/lib/sbctl/keys/KEK/KEK.pem 2>/dev/null
    fi
    if [ ! -f /var/lib/sbctl/keys/KEK/KEK.key ]; then
        cp -a /etc/uefi-keys/KEK.key /var/lib/sbctl/keys/KEK/KEK.key 2>/dev/null || true
    fi
    if [ ! -f /var/lib/sbctl/keys/db/db.pem ]; then
        openssl x509 -in /etc/uefi-keys/db.crt -out /var/lib/sbctl/keys/db/db.pem 2>/dev/null
    fi
    if [ ! -f /var/lib/sbctl/keys/db/db.key ]; then
        cp -a /etc/uefi-keys/db.key /var/lib/sbctl/keys/db/db.key 2>/dev/null || true
    fi
    echo "Certs imported to /var/lib/sbctl/keys/"
else
    echo "No /etc/uefi-keys/ certs found — skipping import."
fi

# --- 5. Enroll keys ---
echo ""
echo "Enrolling keys — requires Secure Boot in Setup Mode (no Platform Key)."
echo "If this fails, clear Secure Boot keys in BIOS first (Custom Mode → Delete All Keys)."
sbctl enroll-keys -m || echo "WARNING: sbctl enroll-keys failed (Setup Mode needed?)"

# --- 6. Verify ---
echo ""
echo "=== Verification ==="
sbctl status || true
echo ""
sbctl verify || true
echo ""
echo "Signed files:"
sbctl list-files || true

apk del sbctl

echo ""
echo "=== Setup Complete ==="
echo ""
echo "IMPORTANT: Secure Boot must be enabled in BIOS after enrollment."
echo "  BIOS → Security → Secure Boot → Secure Boot Enable: ON"
echo "  Secure Boot Mode: Custom → Save & Exit → Reboot"
