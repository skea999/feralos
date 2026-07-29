#!/bin/bash
#
# FeralOS Installer - Interactive Setup
# Clones the repository, prompts for configuration, compiles the Go installer,
# and launches it.
#

set -euo pipefail

repoUrl="https://github.com/skem/skeaAlpineInstaller.git"
repoBranch="latest-stable"

if [ "$(id -u)" != "0" ]; then
    echo "ERROR: This script must be run as root (sudo bash setup.sh)"
    exit 1
fi

# Validate live environment — Linux Mint or Debian are supported
if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "${ID}" in
        linuxmint|debian) ;;
        *)
            echo "ERROR: Unsupported live environment '${ID}'."
            echo "Only Linux Mint and Debian are supported as live environments."
            exit 1
            ;;
    esac
else
    echo "ERROR: Cannot detect live environment (/etc/os-release not found)."
    echo "Only Linux Mint and Debian are supported as live environments."
    exit 1
fi

cat << 'EOF'
   _____                   _               
  |  ___|_ _ _ __ __ _  __| | ___  _ __   
  | |_ / _` | '__/ _` |/ _` |/ _ \| '_ \  
  |  _| (_| | | | (_| | (_| | (_) | | | | 
  |_|  \__,_|_|  \__,_|\__,_|\___/|_| |_| 
    ____           _       _   ____                    
   / ___|___  _ __(_)_ __ | | |  _ \ ___  _ __ ___  ___ 
  | |   / _ \| '__| | '_ \| | | |_) / _ \| '_ ` _ \/ _ \
  | |__| (_) | |  | | | | | | |  __/ (_) | | | | | |  __/
   \____\___/|_|  |_|_| |_|_| |_|   \___/|_| |_| |_|\___|
                                                           
   FeralOS Installer
   Alpine Linux • btrfs • LUKS • booster • UKI
EOF
echo
echo "This will clone the repository, configure, and install a hardened Alpine Linux system."
echo "You will be prompted for each setting. Press Enter to accept the default."
echo

# Install dependencies
echo "Installing dependencies..."
if command -v apt >/dev/null 2>&1; then
    apt update
    apt install -y golang git curl tar dosfstools rsync efitools sbsigntool btrfs-progs cryptsetup
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y golang git curl tar dosfstools rsync efitools sbsigntool btrfs-progs cryptsetup
else
    echo "ERROR: No supported package manager found."
    exit 1
fi

echo "Cloning repository..."
installDir=$(mktemp -d /tmp/feralos-installer.XXXXXX)

if command -v git >/dev/null 2>&1; then
    git clone --branch "$repoBranch" "$repoUrl" "$installDir"
elif command -v curl >/dev/null 2>&1; then
    curl -fSL "${repoUrl%.git}/archive/${repoBranch}.tar.gz" -o "$installDir/archive.tar.gz"
    tar -xzf "$installDir/archive.tar.gz" -C "$installDir" --strip-components=1
else
    echo "ERROR: Neither git nor curl is available."
    exit 1
fi

# Create artifacts directory with timestamp
timestamp=$(date +%Y%m%d_%H%M%S)
artifactsDir="$installDir/tests/artifacts/feralos_installer-$timestamp"
mkdir -p "$artifactsDir"

variablesFile="$installDir/src/variables/variables.yaml"

if [ ! -f "$variablesFile" ]; then
    echo "ERROR: variables.yaml not found at $variablesFile"
    exit 1
fi

get_var() {
    local varName="$1"
    # Strip inline comments (# ...) and quotes from YAML values
    local value
    value=$(grep -E "^  ${varName}:" "$variablesFile" | head -1 | sed 's/^[^:]*: *//' | sed "s/['\"]//g" | sed 's/ #.*//' | xargs)
    if [ -z "$value" ]; then
        value=$(grep -E "^${varName}:" "$variablesFile" | head -1 | sed 's/^[^:]*: *//' | sed "s/['\"]//g" | sed 's/ #.*//' | xargs)
    fi
    echo "$value"
}

get_var_list() {
    local varName="$1"
    # Get list items (lines starting with - under the variable), strip quotes
    grep -A 10 "^  ${varName}:" "$variablesFile" | grep "^ *- " | sed 's/^ *- *//' | sed "s/['\"]//g" | sed 's/ #.*//'
}

set_var() {
    local varName="$1"
    local value="$2"
    local quoteChar="${3:-\"}"
    if [ "$quoteChar" = "'" ]; then
        local escapedValue
        escapedValue=$(printf '%s' "$value" | sed "s/'/'\\\\''/g")
        sed -i "s|^  *${varName}:.*|  ${varName}: '${escapedValue}'|" "$variablesFile"
    else
        local escapedValue
        escapedValue=$(printf '%s' "$value" | sed 's/[&/\\]/\\&/g')
        sed -i "s|^  *${varName}:.*|  ${varName}: \"${escapedValue}\"|" "$variablesFile"
    fi
}

set_var_bool() {
    local varName="$1"
    local value="$2"
    sed -i "s|^  *${varName}:.*|  ${varName}: ${value}|" "$variablesFile"
}

prompt() {
    local varName="$1"
    local promptText="$2"
    local defaultVal="$3"
    local isPassword="${4:-no}"

    local displayDefault="$defaultVal"
    [ -z "$displayDefault" ] && displayDefault="(empty)"

    local input
    if [ "$isPassword" = "yes" ]; then
        printf "%s [%s]: " "$promptText" "$displayDefault"
        read -r -s input
        printf "\n"
    else
        printf "%s [%s]: " "$promptText" "$displayDefault"
        read -r input || true
    fi

    [ -z "$input" ] && input="$defaultVal"

    local quoteChar="\""
    if [ "$varName" = "password" ]; then
        quoteChar="'"
    fi

    set_var "$varName" "$input" "$quoteChar"
}

prompt_yesno() {
    local varName="$1"
    local promptText="$2"
    local defaultVal="$3"

    local input
    printf "%s [%s]: " "$promptText" "$defaultVal"
    read -r input || true

    [ -z "$input" ] && input="$defaultVal"

    case "$input" in
        y|Y|yes|YES|Yes) input="true" ;;
        n|N|no|NO|No) input="false" ;;
        *) input="$defaultVal" ;;
    esac

    set_var_bool "$varName" "$input"
}

disk1Default=$(get_var_list "disks" | head -1)
disk2Default=$(get_var_list "disks" | tail -1)
enableEncryptionDefault=$(get_var "enableEncryption")
enableSecurebootDefault=$(get_var "enableSecureboot")
enableTPMDefault=$(get_var "enableTPM")
enableHardeningDefault=$(get_var "enableHardening")
enableZswapDefault=$(get_var "enableZswap")
enableNixDefault=$(get_var "enableNix")
passwordDefault=$(get_var "password")
userNameDefault=$(get_var "userName")
timeZoneDefault=$(get_var "timezone")
localeDefault=$(get_var "locale")
keymapDefault=$(get_var "keymap")
mirrorDefault=$(get_var "mirror")
branchDefault=$(get_var "branch")
sshAuthorizedKeyDefault=$(get_var "sshAuthorizedKey")
diskLayoutDefault=$(get_var "diskLayout")

echo "=== Disk Configuration ==="
echo "(Verify your disk names first with: fdisk -l)"
echo

# Read disk values manually since they're a YAML list
printf "Primary disk [%s]: " "$disk1Default"
read -r disk1Input || true
[ -z "$disk1Input" ] && disk1Input="$disk1Default"

printf "Secondary disk (for RAID1) [%s]: " "$disk2Default"
read -r disk2Input || true
[ -z "$disk2Input" ] && disk2Input="$disk2Default"

if [ "$disk1Input" = "$disk2Input" ]; then
    echo "ERROR: Primary and secondary disks must be different."
    exit 1
fi

# Rewrite the disks list in YAML
sed -i "/^  disks:/,/^[^ ]/{/^  disks:/!{/^  *- /d;};}" "$variablesFile"
sed -i "/^  disks:/a\\    - \"${disk1Input}\"" "$variablesFile"
sed -i "/^  disks:/a\\    - \"${disk2Input}\"" "$variablesFile"

echo
echo "=== Feature Toggles ==="
echo

prompt_yesno "enableEncryption" "Enable LUKS2 disk encryption" "$enableEncryptionDefault"
prompt_yesno "enableSecureboot" "Enable Secure Boot (UKI signing)" "$enableSecurebootDefault"
prompt_yesno "enableTPM" "Enable TPM2 auto-unlock" "$enableTPMDefault"
prompt_yesno "enableHardening" "Enable security hardening" "$enableHardeningDefault"
prompt_yesno "enableZswap" "Enable zswap compressed swap" "$enableZswapDefault"
prompt_yesno "enableNix" "Enable Nix package manager" "$enableNixDefault"

echo
echo "=== Credentials ==="
echo

prompt "password" "Root and user password" "$passwordDefault" "yes"
prompt "userName" "Regular username" "$userNameDefault"

echo
echo "=== Locale & Keyboard ==="
echo

prompt "timezone" "Timezone (e.g., Europe/Rome, America/New_York)" "$timeZoneDefault"
prompt "locale" "Language locale (e.g., it, en, de, fr)" "$localeDefault"
prompt "keymap" "Console keymap (e.g., it, us, de, fr)" "$keymapDefault"

echo
echo "=== Disk Layout ==="
echo

prompt "diskLayout" "Disk layout (btrfsStandardServer, btrfsStandard)" "$diskLayoutDefault"

echo
echo "=== SSH (optional) ==="
echo

prompt "sshAuthorizedKey" "SSH public key for passwordless access" "$sshAuthorizedKeyDefault"

echo
echo "=== Advanced (optional) ==="
echo

prompt "mirror" "Alpine package mirror" "$mirrorDefault"
prompt "branch" "Alpine release branch" "$branchDefault"

echo
echo

summaryDisk1=$(get_var_list "disks" | head -1)
summaryDisk2=$(get_var_list "disks" | tail -1)
summaryEnableEncryption=$(get_var "enableEncryption")
summaryEnableSecureboot=$(get_var "enableSecureboot")
summaryEnableTPM=$(get_var "enableTPM")
summaryEnableHardening=$(get_var "enableHardening")
summaryEnableZswap=$(get_var "enableZswap")
summaryEnableNix=$(get_var "enableNix")
summaryUserName=$(get_var "userName")
summaryTimezone=$(get_var "timezone")
summaryLocale=$(get_var "locale")
summaryKeymap=$(get_var "keymap")
summaryDiskLayout=$(get_var "diskLayout")
summarySshAuthorizedKey=$(get_var "sshAuthorizedKey" || true)
summaryMirror=$(get_var "mirror")
summaryBranch=$(get_var "branch")

cat << EOF
Configuration Summary:
  disk1:               $summaryDisk1
  disk2:               $summaryDisk2
  enableEncryption:    $summaryEnableEncryption
  enableSecureboot:    $summaryEnableSecureboot
  enableTPM:           $summaryEnableTPM
  enableHardening:     $summaryEnableHardening
  enableZswap:         $summaryEnableZswap
  enableNix:           $summaryEnableNix
  userName:            $summaryUserName
  password:            [hidden]
  timezone:            $summaryTimezone
  locale:              $summaryLocale
  keymap:              $summaryKeymap
  diskLayout:          $summaryDiskLayout
  sshAuthorizedKey:    ${summarySshAuthorizedKey:-(not set)}
  mirror:              $summaryMirror
  branch:              $summaryBranch

EOF

confirmInput="no"
printf "Proceed with installation? (WARNING: this will ERASE both disks) [%s]: " "no"
read -r confirmInput || true
[ -z "$confirmInput" ] && confirmInput="no"
case "$confirmInput" in
    y|Y|yes|YES|Yes) confirmInput="true" ;;
    *) confirmInput="false" ;;
esac

if [ "$confirmInput" != "true" ]; then
    echo "Installation cancelled."
    exit 0
fi

echo "Configuration written to variables.yaml"

# Source harness overrides if present (adds sshAuthorizedKey, defaultNetworkInterface, etc.)
if [ -n "${SKEA_RUNTIME_OVERRIDES_FILE:-}" ] && [ -f "${SKEA_RUNTIME_OVERRIDES_FILE}" ]; then
    echo "Applying harness overrides from ${SKEA_RUNTIME_OVERRIDES_FILE}..."
    . "${SKEA_RUNTIME_OVERRIDES_FILE}"
    # Apply overrides to variables.yaml
    if [ -n "${sshAuthorizedKey:-}" ]; then
        set_var "sshAuthorizedKey" "$sshAuthorizedKey"
    fi
    if [ -n "${defaultNetworkInterface:-}" ]; then
        set_var "defaultNetworkInterface" "$defaultNetworkInterface"
    fi
    if [ -n "${extraKernelArgs:-}" ]; then
        echo "Extra kernel args: ${extraKernelArgs}"
    fi
    echo "Harness overrides applied."
fi

echo
echo "Compiling installer..."
echo

cd "$installDir/src"
go build -o "$installDir/feralos_installer" .

echo
echo "Starting installer..."
echo

cd "$installDir"
./feralos_installer -config src/variables/variables.yaml

echo
echo "Installation complete. Reboot and remove the Live USB."
