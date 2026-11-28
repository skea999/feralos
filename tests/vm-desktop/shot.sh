#!/bin/sh
# Save the VM screen (the Xvfb screen QEMU draws on) as PNG. QEMU's own
# screendump is useless here: with virtio-vga-gl the guest draws through
# virgl and the 2D surface keeps the last firmware frame.
#   box --host medion --pod feralos-vm --kvm --no-sync \
#     --pull tests/artifacts/vm-desktop/screen.png sh tests/vm-desktop/shot.sh
set -eu
repo=$(cd "$(dirname "$0")/../.." && pwd)
out=${1:-$repo/tests/artifacts/vm-desktop/screen.png}
mkdir -p "$(dirname "$out")"
xwd -root -display :99 -silent | xwdtopnm 2>/dev/null | pnmtopng >"$out"
echo "$out"
