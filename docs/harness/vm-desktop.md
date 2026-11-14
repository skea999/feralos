# Harness — vm-desktop (FeralOS desktop in a container, over VNC)

`tests/vm-desktop/` runs the whole FeralOS lifecycle inside one container on a
host with no GPU: the harness installs FeralOS into a disk image, then the
installed system boots as a full QEMU virtual machine whose desktop you watch
and drive over VNC. Built for the boxd build server (the Medion), works on any
podman host that hands the container `/dev/kvm`.

## How the display works without a GPU

The harness's normal display (`-display egl-headless,gl=on`) needs a host DRM
render node; a GPU-less host has none (`egl: no drm render node available`).
Here instead:

1. `Xvfb :99` (1280x800, the guest's resolution) is a virtual X screen.
2. QEMU draws the guest's `virtio-vga-gl` with `-display sdl,gl=on` on that
   screen; Mesa llvmpipe renders on the processor. The guest still sees a
   virgl GPU, so niri accepts it (niri refuses software renderers only inside
   the guest).
3. `matchbox-window-manager` maximizes QEMU's window onto the screen (SDL's
   own full-screen does nothing without a window manager).
4. `x11vnc` serves the screen on `127.0.0.1:5900` inside the container.
5. The guest gets a `usb-tablet` (absolute pointer), so clicks land where
   aimed, and Xvfb takes the guest's keyboard layout (`setxkbmap`, from the
   `keymap` variable): QEMU passes key positions, so a US screen in front of
   an Italian guest would type `'` for `-`.

Two harness and debug_run environment variables make this possible, both
unset by default (behavior unchanged):

| Variable | Effect |
|---|---|
| `SKEA_QEMU_DISPLAY` | replaces the target's `-display` value (default `egl-headless,gl=on`, debug_run `egl-headless`) |
| `SKEA_QEMU_EXTRA_ARGS` | extra target QEMU arguments, split on spaces (here `-device qemu-xhci -device usb-tablet`) |

## Run it (boxd)

From the repository root of a worktree:

```sh
box --host medion --pod feralos-vm --kvm \
  --image-file tests/vm-desktop/Containerfile --detach sh tests/vm-desktop/start.sh
box log <run> --host medion --pod feralos-vm      # progress
```

- First call: `prepare-runner` (once) + `skea_harness run --keep-target-running`
  with the niriWayle dinit fixture, about 6 minutes on the Medion with a warm
  package cache; the full 114-check verification runs on this display too.
- Later calls: boot the last verified run's disk with debug_run (about 1.5
  minutes, TPM unlocks the disk).
- `start.sh` refuses to start while another run is active in the box (they
  would fight over the forwarded ports and swtpm). Stop the VM with
  `box --no-sync ... sh -c 'pkill -TERM -x debug_run'` (clean guest poweroff).
- `--detach` is required: boxd 0.3.1 ends a foreground call's command when the
  calling process goes away.

Environment: `FERALOS_FIXTURE` (variables file, relative to the repository),
`FERALOS_REINSTALL` (non-empty: install again), `FERALOS_CPUS` (8),
`FERALOS_MEMORY` (6144 MiB), `FERALOS_DISK` (32G: the full niriWayle app set
overflows the 16G harness default and btrfs goes read-only).

## Watch it

Tunnel the container's VNC port to your machine through `box` (it passes stdin
and stdout through), from the same worktree:

```sh
socat TCP-LISTEN:5901,bind=127.0.0.1,fork,reuseaddr \
  EXEC:"box --host medion --pod feralos-vm --kvm --no-sync --ps-after 0 socat - TCP\:127.0.0.1\:5900"
vncviewer localhost:5901
```

## Watch it in a browser (noVNC)

The image carries the noVNC web client (v1.7.0, pinned by sha256) and
`tests/vm-desktop/novnc`, a small Go server that serves it on
`127.0.0.1:6080` in the container and bridges its WebSocket to x11vnc (Go
stand-in for the Python websockify). It accepts same-origin WebSockets only,
so another page open in the browser cannot drive the desktop. Forward the
port and open the page:

```sh
socat TCP-LISTEN:6080,bind=127.0.0.1,fork,reuseaddr \
  EXEC:"box --host medion --pod feralos-vm --kvm --no-sync --ps-after 0 socat - TCP\:127.0.0.1\:6080"
xdg-open "http://localhost:6080/vnc.html?autoconnect=1&resize=scale"
```

`box forward` (boxd 0.4.0) replaces the socat line.

## Drive it from an agent

- `tests/vm-desktop/shot.sh [out.png]`: PNG of the screen (`xwd` of Xvfb).
  QEMU's own `screendump` is useless with this device: the guest draws through
  virgl and the 2D surface keeps the last firmware frame.
- `tests/vm-desktop/input.sh click X Y | move X Y | key KEYS... | type TEXT`:
  `xdotool` on the screen, coordinates in guest pixels; `click` pauses 0.3 s
  between move and press so the guest sees the pointer arrive first.

Look, act, look again:

```sh
B="box --host medion --pod feralos-vm --kvm --no-sync --ps-after 0"
$B sh tests/vm-desktop/input.sh key super+t
$B --pull tests/artifacts/vm-desktop/screen.png sh tests/vm-desktop/shot.sh
```

`--pull` needs the local folder (`mkdir -p tests/artifacts/vm-desktop`). Files
under `tests/artifacts/` are not synced into the box (ignored files): send a
script over stdin instead (`$B ssh ... sh -s < local-script.sh`). Guest SSH
from inside the box: `ssh -i tests/artifacts/harness/cache/keys/id_ed25519 -p
2224 root@127.0.0.1` (debug_run boots) or port 2223 (harness boots).

## Verified (2026-11-14, Medion, boxd 0.3.1)

- Harness run `2026-11-14T05-33-16Z` (fixture `btrfs_standard_niriwayle_test`):
  **114/114** on this display.
- From a clean container: boot through debug_run, layout `it`, typing
  `echo ok-[x]{y}@#~/ | tr a-z A-Z` printed `OK-[X]{Y}@#~/`, a click on the bar
  clock opened the Wayle calendar, the VNC tunnel answered `RFB 003.008`.
- Not verified: smoothness of a long interactive VNC session.
