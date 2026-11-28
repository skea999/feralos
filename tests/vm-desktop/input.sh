#!/bin/sh
# Drive the VM desktop like a user, through the Xvfb screen QEMU draws on
# (coordinates are guest pixels: the screen is the guest's 1280x800).
#   input.sh click X Y        left click
#   input.sh move X Y         move the pointer
#   input.sh key KEYS...      xdotool key names, e.g. super+t ctrl+c Return
#   input.sh type TEXT        type text
# Pair with shot.sh to see the result:
#   box --host medion --pod feralos-vm --kvm --no-sync sh tests/vm-desktop/input.sh key super+t
set -eu
export DISPLAY=:99
cmd=${1:?usage: input.sh click|move|key|type ...}; shift
case $cmd in
  # the pause lets the guest see the pointer arrive before the press
  click) xdotool mousemove --sync "$1" "$2" sleep 0.3 click 1 ;;
  move)  xdotool mousemove --sync "$1" "$2" ;;
  key)   xdotool key --delay 80 "$@" ;;
  type)  xdotool type --delay 40 "$*" ;;
  *) echo "unknown command: $cmd" >&2; exit 2 ;;
esac
