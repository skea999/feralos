#!/bin/bash
#
# FeralOS Installer - Setup
# Builds the Go installer and runs it.
#
# Usage:
#   sudo bash setup.sh                          # Interactive — Go installer prompts
#   sudo bash setup.sh -config variables.yaml   # Non-interactive — pass config file
#   sudo bash setup.sh --test /path/to/repo     # Test mode — use local repo
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
            echo "Usage: setup.sh [-config file.yaml] [--test /path/to/repo]"
            exit 1 ;;
    esac
done

if [ "$(id -u)" != "0" ]; then
    echo "ERROR: Run as root — sudo bash setup.sh"
    exit 1
fi

# --- Locate repository ---
if [ "$testMode" = true ]; then
    # Test mode: use local repo (harness copies repo to runner)
    installDir="$testRepo"
    echo "[TEST] Using local repo: $installDir"
elif [ -n "${SKEA_HARNESS_LOCAL_REPO:-}" ] && [ -d "$SKEA_HARNESS_LOCAL_REPO/src" ]; then
    # Harness mode: local repo already available
    installDir="$SKEA_HARNESS_LOCAL_REPO"
    echo "[HARNESS] Using local repo: $installDir"
else
    # Manual mode: clone or update
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

# --- Build installer ---
echo "Building installer..."
cd "$installDir/src"
go build -o "$installDir/feralos_installer" .

# --- Run installer ---
echo
cd "$installDir"
if [ -n "$configFile" ]; then
    # Non-interactive: pass config file to Go installer
    ./feralos_installer -config "$configFile"
else
    # Interactive: Go installer prompts for configuration
    ./feralos_installer
fi

echo
echo "Installation complete. Reboot and remove the Live USB."
