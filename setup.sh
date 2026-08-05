#!/bin/bash
#
# FeralOS Installer - Setup
# Reads configuration (interactive or non-interactive), builds the Go installer,
# and runs it.
#
# Non-interactive mode (harness):
#   SKEA_HARNESS_LOCAL_REPO=/path/to/repo bash setup.sh < answers.txt
#
# Interactive mode (manual):
#   sudo bash setup.sh
#

set -euo pipefail

repoUrl="https://gitea.com/sketa999/skeaAlpineInstaller.git"
repoBranch="main"

if [ "$(id -u)" != "0" ]; then
    echo "ERROR: Run as root — sudo bash setup.sh"
    exit 1
fi

# --- Locate repository ---
if [ -n "${SKEA_HARNESS_LOCAL_REPO:-}" ] && [ -d "$SKEA_HARNESS_LOCAL_REPO/src" ]; then
    installDir="$SKEA_HARNESS_LOCAL_REPO"
    echo "Using local repo: $installDir"
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
    cd "$installDir"
fi

variablesFile="$installDir/src/variables/variables.yaml"
if [ ! -f "$variablesFile" ]; then
    echo "ERROR: $variablesFile not found"
    exit 1
fi

# --- Helper functions ---
get_var() {
    local name="$1"
    grep -E "^  *${name}:" "$variablesFile" 2>/dev/null | head -1 \
        | sed 's/^[^:]*: *//' | sed "s/['\"]//g" | sed 's/ #.*//' | xargs
}

get_var_list() {
    local name="$1"
    grep -A 10 "^  *${name}:" "$variablesFile" 2>/dev/null | grep "^ *- " \
        | sed 's/^ *- *//' | sed "s/['\"]//g" | sed 's/ #.*//'
}

set_var() {
    local name="$1" value="$2"
    sed -i "s|^  *${name}:.*|  ${name}: \"${value}\"|" "$variablesFile"
}

set_var_bool() {
    local name="$1" value="$2"
    sed -i "s|^  *${name}:.*|  ${name}: ${value}|" "$variablesFile"
}

set_var_list() {
    local name="$1"; shift
    # Remove existing list items, then append new ones
    sed -i "/^  ${name}:/,/^[^ ]/{/^  ${name}:/!{/^  *- /d;};}" "$variablesFile"
    for item in "$@"; do
        sed -i "/^  ${name}:/a\\    - \"${item}\"" "$variablesFile"
    done
}

prompt() {
    local name="$1" text="$2" default="$3" isPassword="${4:-no}"
    local input
    if [ "$isPassword" = "yes" ]; then
        printf "%s [%s]: " "$text" "$default"
        read -r -s input; printf "\n"
    else
        printf "%s [%s]: " "$text" "$default"
        read -r input || true
    fi
    [ -n "$input" ] && default="$input"
    if [ "$name" = "password" ]; then
        sed -i "s|^  *${name}:.*|  ${name}: '${default}'|" "$variablesFile"
    else
        set_var "$name" "$default"
    fi
}

prompt_yn() {
    local name="$1" text="$2" default="$3"
    local input
    printf "%s [%s]: " "$text" "$default"
    read -r input || true
    [ -n "$input" ] && default="$input"
    case "$default" in
        y|Y|yes|YES|Yes) default="true" ;;
        n|N|no|NO|No) default="false" ;;
    esac
    set_var_bool "$name" "$default"
}

# --- Read defaults ---
disk1Default=$(get_var_list "disks" | head -1)
disk2Default=$(get_var_list "disks" | tail -1)

# --- Non-interactive mode (harness) ---
# Reads answers from stdin in fixed order, writes to variables.yaml.
if [ ! -t 0 ]; then
    echo "=== Non-interactive mode ==="
    read -r disk1; read -r disk2
    read -r enc; read -r sb; read -r tpm
    read -r hard; read -r zswap; read -r nix
    read -r pass; read -r user
    read -r tz; read -r locale; read -r keymap
    read -r layout; read -r sshkey
    read -r mirror; read -r branch
    read -r _confirm || true

    # Write disks
    if [ "$disk1" = "$disk2" ]; then
        set_var_list "disks" "$disk1"
    else
        set_var_list "disks" "$disk1" "$disk2"
    fi

    # Write features
    [ "$enc" = "yes" ]   && set_var_bool "enableEncryption" "true" || set_var_bool "enableEncryption" "false"
    [ "$sb"  = "yes" ]   && set_var_bool "enableSecureboot" "true" || set_var_bool "enableSecureboot" "false"
    [ "$tpm" = "yes" ]   && set_var_bool "enableTPM" "true" || set_var_bool "enableTPM" "false"
    [ "$hard" = "yes" ]  && set_var_bool "enableHardening" "true" || set_var_bool "enableHardening" "false"
    [ "$zswap" = "yes" ] && set_var_bool "enableZswap" "true" || set_var_bool "enableZswap" "false"
    [ "$nix" = "yes" ]   && set_var_bool "enableNix" "true" || set_var_bool "enableNix" "false"

    # Write credentials
    sed -i "s|^  *password:.*|  password: '${pass}'|" "$variablesFile"
    set_var "userName" "$user"

    # Write locale
    set_var "timezone" "$tz"
    set_var "locale" "$locale"
    set_var "keymap" "$keymap"

    # Write layout & SSH
    set_var "diskLayout" "$layout"
    set_var "sshAuthorizedKey" "$sshkey"
    [ -n "$mirror" ] && set_var "mirror" "$mirror"
    [ -n "$branch" ] && set_var "branch" "$branch"

    echo "Configuration written to variables.yaml"
else
    # --- Interactive mode ---
    cat << 'EOF'
   _____                   _
  |  ___|_ _ _ __ __ _  __| | ___  _ __
  | |_ / _` | '__/ _` |/ _` |/ _ \| '_ \
  |  _| (_| | | | (_| | (_| | (_) | | | |
  |_|  \__,_|_|  \__,_|\__,_|\___/|_| |_|

   FeralOS Installer
   Alpine Linux • btrfs • LUKS • booster • UKI
EOF
    echo

    echo "=== Disk Configuration ==="
    echo "(Verify disks with: fdisk -l)"
    printf "Primary disk [%s]: " "$disk1Default"; read -r d1; d1="${d1:-$disk1Default}"
    printf "Secondary disk (RAID1) [%s]: " "$disk2Default"; read -r d2; d2="${d2:-$disk2Default}"
    if [ "$d1" = "$d2" ]; then
        set_var_list "disks" "$d1"
    else
        set_var_list "disks" "$d1" "$d2"
    fi

    echo; echo "=== Features ==="
    prompt_yn "enableEncryption" "Enable LUKS2 encryption" "$(get_var enableEncryption)"
    prompt_yn "enableSecureboot" "Enable Secure Boot" "$(get_var enableSecureboot)"
    prompt_yn "enableTPM" "Enable TPM2 auto-unlock" "$(get_var enableTPM)"
    prompt_yn "enableHardening" "Enable security hardening" "$(get_var enableHardening)"
    prompt_yn "enableZswap" "Enable zswap" "$(get_var enableZswap)"
    prompt_yn "enableNix" "Enable Nix package manager" "$(get_var enableNix)"

    echo; echo "=== Credentials ==="
    prompt "password" "Root and user password" "$(get_var password)" "yes"
    prompt "userName" "Regular username" "$(get_var userName)"

    echo; echo "=== Locale ==="
    prompt "timezone" "Timezone (e.g. Europe/Rome)" "$(get_var timezone)"
    prompt "locale" "Language (e.g. it, en)" "$(get_var locale)"
    prompt "keymap" "Keymap (e.g. it, us)" "$(get_var keymap)"

    echo; echo "=== Disk Layout ==="
    prompt "diskLayout" "Layout (btrfsStandardServer, btrfsStandard)" "$(get_var diskLayout)"

    echo; echo "=== SSH (optional) ==="
    prompt "sshAuthorizedKey" "SSH public key" "$(get_var sshAuthorizedKey)"

    echo; echo "=== Mirror (optional) ==="
    prompt "mirror" "Alpine mirror" "$(get_var mirror)"
    prompt "branch" "Alpine branch" "$(get_var branch)"

    echo; echo "Proceed with installation? (WARNING: disks will be erased) [no]: "
    read -r confirm; confirm="${confirm:-no}"
    case "$confirm" in
        y|Y|yes|YES|Yes) ;;
        *) echo "Cancelled."; exit 0 ;;
    esac
fi

# --- Build & run installer ---
echo
echo "Building installer..."
cd "$installDir/src"
go build -o "$installDir/feralos_installer" .

echo
echo "Starting installer..."
cd "$installDir"
./feralos_installer -config src/variables/variables.yaml

echo
echo "Installation complete. Reboot and remove the Live USB."
