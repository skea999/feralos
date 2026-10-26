#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: debugTargetTpmBoot.sh --run-dir PATH [options]

Boot an already installed harness target outside the main harness so TPM/OVMF
issues can be reproduced in isolation.

Options:
  --run-dir PATH        Existing harness run directory containing install disks.
  --name NAME           Session name prefix. Default: debug-target
  --ssh-port PORT       Host SSH forward. Default: 2299
  --with-tpm            Attach swtpm and QEMU TPM device.
  --tpm-device DEVICE   QEMU TPM frontend. Default: tpm-crb
  --without-kvm         Disable -enable-kvm for the repro VM.
  --fresh-vars          Copy fresh OVMF vars instead of reusing run OVMF_VARS.fd.
  --vars-path PATH      Explicit OVMF vars template to copy when --fresh-vars is used.
  --help                Show this message.

Outputs are written under <run-dir>/debug-<timestamp>/.
EOF
}

run_dir=""
name_prefix="debug-target"
ssh_port=2299
with_tpm=0
tpm_device="tpm-tis"
use_kvm=1
fresh_vars=0
vars_template=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --run-dir)
      run_dir="$2"
      shift 2
      ;;
    --name)
      name_prefix="$2"
      shift 2
      ;;
    --ssh-port)
      ssh_port="$2"
      shift 2
      ;;
    --with-tpm)
      with_tpm=1
      shift
      ;;
    --tpm-device)
      tpm_device="$2"
      shift 2
      ;;
    --without-kvm)
      use_kvm=0
      shift
      ;;
    --fresh-vars)
      fresh_vars=1
      shift
      ;;
    --vars-path)
      vars_template="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$run_dir" ]]; then
  echo "--run-dir is required" >&2
  usage >&2
  exit 1
fi

run_dir=$(realpath "$run_dir")
if [[ ! -d "$run_dir" ]]; then
  echo "Run directory not found: $run_dir" >&2
  exit 1
fi

detect_ovmf_code() {
  local candidates=(
    /usr/share/OVMF/OVMF_CODE_4M.snakeoil.fd
    /usr/share/OVMF/OVMF_CODE_4M.fd
    /usr/share/OVMF/OVMF_CODE.fd
    /usr/share/ovmf/OVMF.fd
    /usr/share/edk2/ovmf/OVMF_CODE.fd
  )
  local candidate
  for candidate in "${candidates[@]}"; do
    [[ -f "$candidate" ]] && printf '%s\n' "$candidate" && return 0
  done
  return 1
}

detect_ovmf_vars() {
  local candidates=(
    /usr/share/OVMF/OVMF_VARS_4M.snakeoil.fd
    /usr/share/OVMF/OVMF_VARS_4M.fd
    /usr/share/OVMF/OVMF_VARS.fd
    /usr/share/edk2/ovmf/OVMF_VARS.fd
  )
  local candidate
  for candidate in "${candidates[@]}"; do
    [[ -f "$candidate" ]] && printf '%s\n' "$candidate" && return 0
  done
  return 1
}

ovmf_code=$(detect_ovmf_code) || {
  echo "Unable to locate OVMF code image" >&2
  exit 1
}

timestamp=$(date -u +%Y-%m-%dT%H-%M-%SZ)
session_dir="$run_dir/${name_prefix}-${timestamp}"
mkdir -p "$session_dir"
socket_dir=$(mktemp -d "/tmp/skea-target-debug.XXXXXX")

if [[ $fresh_vars -eq 1 ]]; then
  if [[ -z "$vars_template" ]]; then
    vars_template=$(detect_ovmf_vars) || {
      echo "Unable to locate an OVMF vars template" >&2
      exit 1
    }
  fi
else
  vars_template="$run_dir/OVMF_VARS.fd"
fi

if [[ ! -f "$vars_template" ]]; then
  echo "OVMF vars template not found: $vars_template" >&2
  exit 1
fi

vars_path="$session_dir/OVMF_VARS.fd"
cp "$vars_template" "$vars_path"

disk1_path="$run_dir/install-disk-1.qcow2"
disk2_path="$run_dir/install-disk-2.qcow2"
for disk in "$disk1_path" "$disk2_path"; do
  [[ -f "$disk" ]] || {
    echo "Missing disk image: $disk" >&2
    exit 1
  }
done

firmware_log="$session_dir/ovmf-debug.log"
serial_log="$session_dir/serial.log"
qemu_log="$session_dir/qemu.log"
qmp_sock="$socket_dir/qmp.sock"
serial_sock="$socket_dir/serial.sock"

# shellcheck disable=SC2054  # QEMU -global/-chardev/-qmp take ONE comma-separated key=value argument each
qemu_args=(
  -name "$name_prefix"
  -machine q35,smm=on
  -m 4096
  -smp 2
  -nographic
  -chardev "socket,id=targetserial,path=$serial_sock,server=on,wait=off,logfile=$serial_log,logappend=on"
  -serial chardev:targetserial
  -qmp "unix:$qmp_sock,server=on,wait=off"
  -global driver=cfi.pflash01,property=secure,value=on
  -global isa-debugcon.iobase=0x402
  -debugcon "file:$firmware_log"
  -drive "if=pflash,format=raw,readonly=on,file=$ovmf_code"
  -drive "if=pflash,format=raw,file=$vars_path"
  -netdev "user,id=net0,hostfwd=tcp::${ssh_port}-:22"
  -device virtio-net-pci,netdev=net0
  -drive "if=none,id=targetdisk1,format=qcow2,file=$disk1_path"
  -device virtio-blk-pci,drive=targetdisk1,bootindex=1
  -drive "if=none,id=targetdisk2,format=qcow2,file=$disk2_path"
  -device virtio-blk-pci,drive=targetdisk2,bootindex=2
)

if [[ $with_tpm -eq 1 ]]; then
  tpm_state_dir="$session_dir/tpm-state"
  tpm_ctrl="$socket_dir/tpm.sock.ctrl"
  tpm_pid="$session_dir/tpm.pid"
  tpm_log="$session_dir/tpm.log"
  mkdir -p "$tpm_state_dir"
  # QEMU 10 uses CMD_SET_DATAFD: connect chardev to --ctrl socket only.
  # Do NOT pass --server: swtpm pre-sets mlp->fd from the server socket,
  # causing CMD_SET_DATAFD to fail with TPM_IOERROR.
  swtpm socket \
    --tpm2 \
    --daemon \
    --tpmstate "dir=$tpm_state_dir" \
    --ctrl "type=unixio,path=$tpm_ctrl" \
    --pid "file=$tpm_pid" \
    --log "file=$tpm_log" \
    --flags not-need-init,startup-clear
  qemu_args+=(
    -chardev "socket,id=chrtpm,path=$tpm_ctrl"
    -tpmdev "emulator,id=tpm0,chardev=chrtpm"
    -device "$tpm_device,tpmdev=tpm0"
  )
fi

if [[ $use_kvm -eq 1 ]]; then
  qemu_args+=( -enable-kvm )
fi

printf 'SESSION_DIR=%s\n' "$session_dir"
printf 'OVMF_CODE=%s\n' "$ovmf_code"
printf 'OVMF_VARS=%s\n' "$vars_path"
printf 'SOCKET_DIR=%s\n' "$socket_dir"
printf 'SSH_PORT=%s\n' "$ssh_port"
printf 'TPM=%s\n' "$with_tpm"
printf 'TPM_DEVICE=%s\n' "$tpm_device"
printf 'KVM=%s\n' "$use_kvm"

exec qemu-system-x86_64 "${qemu_args[@]}" >"$qemu_log" 2>&1