#!/bin/sh
# Cache connectivity debug and fallback for harness test environment.
# Runs on the target before 1-secure-boot-sbctl.sh.
# The harness cache proxy can have stale APKINDEX (package listed but
# file unavailable from upstream). Always switch to official Alpine repos.

echo "=== Cache Debug ==="
echo "Repositories:"
cat /etc/apk/repositories

echo ""
echo "Switching to official Alpine repos to avoid stale cache index..."
cat > /etc/apk/repositories <<'REPO'
https://dl-cdn.alpinelinux.org/alpine/v3.23/main
https://dl-cdn.alpinelinux.org/alpine/v3.23/community
REPO
apk update 2>&1
echo "Repositories switched to official mirrors."
echo "=== Cache Debug Complete ==="

echo ""
echo "=== UEFI Secure Boot State ==="
if [ -d /sys/firmware/efi/efivars ]; then
    pkVar=$(ls /sys/firmware/efi/efivars/PK-* 2>/dev/null | head -1)
    sbVar=$(ls /sys/firmware/efi/efivars/SecureBoot-* 2>/dev/null | head -1)
    smVar=$(ls /sys/firmware/efi/efivars/SetupMode-* 2>/dev/null | head -1)
    
    if [ -n "$pkVar" ]; then
        echo "PK EXISTS (User Mode) — sbctl enrollment will fail"
        dd if="$pkVar" bs=1 skip=4 count=16 2>/dev/null | od -tx1 -An | head -1
    else
        echo "NO PK (Setup Mode) — sbctl enrollment should work"
    fi
    
    if [ -n "$sbVar" ]; then
        sbRaw=$(dd if="$sbVar" bs=1 skip=4 count=1 2>/dev/null | od -An -t d1 | tr -d ' ')
        echo "SecureBoot=$sbRaw (1=enabled, 0=disabled)"
    fi
    
    if [ -n "$smVar" ]; then
        smRaw=$(dd if="$smVar" bs=1 skip=4 count=1 2>/dev/null | od -An -t d1 | tr -d ' ')
        echo "SetupMode=$smRaw (1=Setup, 0=User)"
    fi
else
    echo "efivars not available (system booted without UEFI?)"
fi
echo "=== UEFI State Debug Complete ==="
