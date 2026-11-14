#!/bin/sh
# Install FeralOS with the harness (first call) or boot the last installed
# disk (later calls), inside the tests/vm-desktop container, with the desktop
# on VNC at 127.0.0.1:5900 in the container. Blocks while the VM runs.
#   FERALOS_FIXTURE    variables file, relative to the repository
#   FERALOS_REINSTALL  non-empty: install again even if a run exists
#   FERALOS_CPUS / FERALOS_MEMORY   virtual machine size (MiB)
#   FERALOS_DISK       target disk size (the full niriWayle app set overflows 16G)
set -eu
repo=$(cd "$(dirname "$0")/../.." && pwd)
fixture=$repo/${FERALOS_FIXTURE:-tests/fixtures/variables/btrfs_standard_niriwayle_test.yaml}
cpus=${FERALOS_CPUS:-8}
memory=${FERALOS_MEMORY:-6144}
disk=${FERALOS_DISK:-32G}
artifacts=$repo/tests/artifacts/harness
log() { printf '%s vm-desktop: %s\n' "$(date -u +%H:%M:%S)" "$*"; }
# One run per box: two would fight over the forwarded ports and swtpm.
if pgrep -x skea_harness >/dev/null || pgrep -x debug_run >/dev/null; then
  log "another run is active in this box (box ps); stop it first"; exit 1
fi

# No render node on a GPU-less host: QEMU draws its GL window on Xvfb.
# The screen matches the guest's 1280x800 and matchbox maximizes the QEMU
# window onto it (SDL full-screen needs a window manager too). The usb-tablet
# gives the guest an absolute pointer, so VNC and xdotool clicks land.
export DISPLAY=:99 SKEA_QEMU_DISPLAY=sdl,gl=on GOFLAGS=-buildvcs=false \
  SKEA_QEMU_EXTRA_ARGS="-device qemu-xhci -device usb-tablet"
if [ ! -e /tmp/.X11-unix/X99 ]; then
  Xvfb :99 -screen 0 1280x800x24 -nolisten tcp >/tmp/xvfb.log 2>&1 &
  until [ -e /tmp/.X11-unix/X99 ]; do sleep 0.2; done
fi
pgrep -x matchbox-window >/dev/null \
  || { setsid matchbox-window-manager -use_titlebar no >/tmp/wm.log 2>&1 & }
pgrep -x x11vnc >/dev/null \
  || x11vnc -display :99 -localhost -rfbport 5900 -forever -shared -nopw -quiet -bg -o /tmp/x11vnc.log
log "display :99 up, VNC on 127.0.0.1:5900"

# QEMU passes key positions, so Xvfb must use the guest's layout: a US screen
# in front of an Italian guest types ' for -. keymap is a bkeymaps name
# (it, us, de...), whose first part is the xkb layout.
layout() {
  kb=$(sed -n "s/^ *keymap: *//p" "$1" | tr -d "\"'" | head -1)
  setxkbmap -display :99 "${kb%%-*}" 2>/dev/null || setxkbmap -display :99 us
  log "keyboard layout ${kb:-us}"
}

log "building harness and debug_run"
(cd "$repo/tests/harness" && go build -o skea_harness ./cmd/skea_harness)
(cd "$repo/tests/debug_run" && go build -o debug_run .)

last=; for d in "$artifacts"/runs/*/; do [ -d "$d" ] && last=$d; done  # glob order = timestamp order
if [ -z "${FERALOS_REINSTALL:-}" ] && [ -n "$last" ] && [ -f "$last/verify.md" ]; then
  log "booting installed disk of $last"
  layout "$last/variables.yaml"
  cd "$repo/tests/debug_run"
  exec ./debug_run --run-dir "$last" --cpus "$cpus" --memory "$memory"
fi

cd "$repo/tests/harness"
if [ ! -f "$artifacts/cache/runner-images/skea-runner.qcow2" ]; then
  log "preparing the runner image"
  ./skea_harness prepare-runner
fi
log "installing from $fixture"
layout "$fixture"
./skea_harness run -f "$fixture" --keep-target-running --cpus "$cpus" --target-memory "$memory" --disk-size "$disk" \
  || log "harness run exit $? (see the run's verify.md)"
# comm is truncated to 15 characters: qemu-system-x86_64 shows as qemu-system-x86
pgrep -x qemu-system-x86 >/dev/null || { log "no VM running"; exit 1; }
log "VM running, desktop on VNC"
while pgrep -x qemu-system-x86 >/dev/null; do sleep 30; done
log "VM stopped"
