package desktops

import (
	"feralos/src/utils/shell"
)

// installXdgSuWrapper writes the niri-family xdg-su wrapper — SHARED by
// niri (DMS) and niriWayle (both setups called it verbatim; the wrapper
// matches *niri* so both session types hit it). STAYS installer-side on
// purpose: abuild forbids packages from writing under /usr/local, and the
// /usr/local location is exactly what survives vendor xdg-su updates
// (the vendor binary stays at /usr/bin/xdg-su as the fallback).
func installXdgSuWrapper(chroot string) error {
	xdgSu := `#!/bin/sh
# xdg-su wrapper — for niri uses alacritty+su, other DEs call the original
# Not overwritten by updates (in /usr/local/bin)

case "$XDG_CURRENT_DESKTOP" in
    *niri*)
        cmd=
        user=
        while [ $# -gt 0 ]; do
            case "$1" in
                -c) cmd="$2"; shift 2 ;;
                -u) user="$2"; shift 2 ;;
                *) shift ;;
            esac
        done
        if [ -n "$cmd" ] && command -v alacritty >/dev/null 2>&1; then
            if [ -n "$user" ]; then
                exec alacritty -e su -c "$cmd" "$user"
            else
                exec alacritty -e su -c "$cmd"
            fi
        fi
        exit 1
        ;;
esac

exec /usr/bin/xdg-su "$@"
`
	if err := shell.WriteFile(chroot, "/usr/local/bin/xdg-su", []byte(xdgSu), 0755); err != nil {
		return err
	}
	return nil
}
