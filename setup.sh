#!/bin/bash
#
# FeralOS Installer - Setup
# Builds the Go installer and runs it.
#
# Usage:
#   sudo bash setup.sh                                    # Interactive
#   sudo bash setup.sh --test /path/to/repo -config file  # Test (harness)
#   sudo bash setup.sh -config file.yaml                  # Non-interactive
#

set -euo pipefail

repoUrl="https://gitea.com/sketa999/skeaAlpineInstaller.git"
repoBranch="main"
testMode=false
testRepo=""
configFile=""

# --- Parse args ---
while [ $# -gt 0 ]; do
    case "$1" in
        -config)
            configFile="$2"; shift 2 ;;
        --test)
            testMode=true
            testRepo="$2"; shift 2 ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: setup.sh [--test /path/to/repo] [-config file.yaml]"
            exit 1 ;;
    esac
done

# --- Validate ---
if [ "$(id -u)" != "0" ]; then
    echo "ERROR: Run as root — sudo bash setup.sh"
    exit 1
fi

if [ "$testMode" = true ] && [ -z "$configFile" ]; then
    echo "ERROR: --test requires -config <file.yaml>"
    exit 1
fi

# --- Locate repository ---
if [ "$testMode" = true ]; then
    installDir="$testRepo"
    echo "[TEST] Using local repo: $installDir"
elif [ -n "${SKEA_HARNESS_LOCAL_REPO:-}" ] && [ -d "$SKEA_HARNESS_LOCAL_REPO/src" ]; then
    installDir="$SKEA_HARNESS_LOCAL_REPO"
    echo "[HARNESS] Using local repo: $installDir"
else
    installDir="/tmp/feralos-installer"
    if [ -d "$installDir/.git" ]; then
        echo "Updating repository..."
        cd "$installDir" && git pull --ff-only 2>/dev/null || true
    else
        echo "Cloning repository..."
        rm -rf "$installDir"
        git clone --branch "$repoBranch" "$repoUrl" "$installDir"
    fi
fi

# --- Validate ---
if [ ! -d "$installDir/src" ]; then
    echo "ERROR: $installDir/src not found"
    exit 1
fi

# --- Install host deps (Debian live + harness runner) ---
# Cloud-init only installed tar/curl/parted for repo copy; golang + btrfs/crypt etc. come here
# so real hardware (git clone) and harness (local tar copy) share same path.
echo "Installing host dependencies..."
export DEBIAN_FRONTEND=noninteractive
apt update -y
apt install -y golang-go btrfs-progs cryptsetup curl tar dosfstools rsync efitools sbsigntool ca-certificates

# --- Build installer (runner-independent: regenerate sums for runner's Go version) ---
echo "Building installer..."
cd "$installDir/src"
go env -w GOPROXY=https://proxy.golang.org,direct
go env -w GOSUMDB=off
go clean -modcache
rm -f go.sum
go mod tidy
go env -w GOSUMDB=sum.golang.org
go build -o "$installDir/feralos_installer" .

# --- Run installer ---
echo
cd "$installDir"
if [ -n "$configFile" ]; then
    ./feralos_installer -config "$configFile"
else
    ./feralos_installer
fi

echo
echo "Installation complete. Reboot and remove the Live USB."
