#!/bin/sh
# niri_realhw_check.sh — FeralOS niri runtime verification on REAL hardware.
#
# Runs ON the target, as root, AFTER the user has logged into the graphical
# session (niri). Complements the harness QEMU runtime checks — same
# assertions (session, wayland socket, IPC, user dinit stack, wayvnc,
# screenshot) plus pre-session system checks (DRM, greetd, elogind,
# turnstiled).
#
# Usage: sh niri_realhw_check.sh [username]
#   username defaults to the graphical session user, detected via
#   loginctl (wayland session preferred) with `who` as fallback.
#
# Exit: 0 = all checks PASS, 1 = some FAIL, 2 = usage/detection failure.

set -u

TOTAL=0
FAILED=0

pass() {
	echo "PASS: $1"
	TOTAL=$((TOTAL + 1))
}

fail() {
	echo "FAIL: $1"
	TOTAL=$((TOTAL + 1))
	FAILED=$((FAILED + 1))
}

# detect_session_user prints the graphical session user: prefer a wayland
# session (loginctl), then any session, then the first `who` line.
detect_session_user() {
	if command -v loginctl >/dev/null 2>&1; then
		for sid in $(loginctl list-sessions --no-legend 2>/dev/null | awk '{print $1}'); do
			stype=$(loginctl show-session "$sid" -p Type 2>/dev/null | cut -d= -f2)
			if [ "$stype" = "wayland" ]; then
				u=$(loginctl show-session "$sid" -p Name 2>/dev/null | cut -d= -f2)
				if [ -n "$u" ]; then
					echo "$u"
					return 0
				fi
			fi
		done
		u=$(loginctl list-sessions --no-legend 2>/dev/null | awk 'NR==1{print $3}')
		if [ -n "$u" ]; then
			echo "$u"
			return 0
		fi
	fi
	u=$(who 2>/dev/null | awk 'NR==1{print $1}')
	[ -n "$u" ] && { echo "$u"; return 0; }
	return 1
}

if [ "$(id -u)" != "0" ]; then
	echo "FAIL: run as root"
	exit 2
fi

USER_NAME="${1:-}"
if [ -z "$USER_NAME" ]; then
	USER_NAME=$(detect_session_user) || {
		echo "FAIL: no username given and graphical session user not detectable"
		exit 2
	}
fi

USER_UID=$(id -u "$USER_NAME" 2>/dev/null) || {
	echo "FAIL: unknown user $USER_NAME"
	exit 2
}
XDGRUN="/run/user/$USER_UID"

echo "=== niri realhw check: user=$USER_NAME uid=$USER_UID ==="

echo "--- system (pre-session) ---"
if [ -e /dev/dri/card0 ]; then
	pass '/dev/dri/card0 present'
else
	fail '/dev/dri/card0 missing'
fi
if [ -e /dev/dri/renderD128 ]; then
	pass '/dev/dri/renderD128 present'
else
	fail '/dev/dri/renderD128 missing'
fi
drm_driver=$(readlink /sys/class/drm/card0/device/driver 2>/dev/null)
if [ -n "$drm_driver" ]; then
	pass "DRM driver: $(basename "$drm_driver")"
else
	fail 'DRM driver link unresolved'
fi
pgrep -x greetd >/dev/null && pass 'greetd running' || fail 'greetd not running'
pgrep -x elogind >/dev/null && pass 'elogind running' || fail 'elogind not running'
pgrep -x turnstiled >/dev/null && pass 'turnstiled running' || fail 'turnstiled not running'

echo "--- session ---"

# logind session type must be wayland
session_type=''
if command -v loginctl >/dev/null 2>&1; then
	for sid in $(loginctl list-sessions --no-legend 2>/dev/null | awk '{print $1}'); do
		if [ "$(loginctl show-session "$sid" -p Name 2>/dev/null | cut -d= -f2)" = "$USER_NAME" ]; then
			session_type=$(loginctl show-session "$sid" -p Type 2>/dev/null | cut -d= -f2)
			break
		fi
	done
fi
[ "$session_type" = "wayland" ] && pass "logind session type: $session_type" ||
	fail "logind session type not wayland (${session_type:-none})"

if pgrep -u "$USER_NAME" -x niri >/dev/null; then
	pass 'niri user session running'
else
	fail 'no niri session for user'
fi

if [ -e "$XDGRUN/wayland-1" ]; then
	pass 'wayland-1 socket OK'
else
	fail 'no wayland socket'
fi

niri_socket=$(ls "$XDGRUN"/niri*.sock 2>/dev/null | head -1)
if [ -n "$niri_socket" ] &&
	XDG_RUNTIME_DIR="$XDGRUN" WAYLAND_DISPLAY=wayland-1 NIRI_SOCKET="$niri_socket" \
		niri msg outputs >/dev/null 2>&1; then
	pass 'niri IPC OK'
else
	fail "niri IPC failed (sock=$niri_socket)"
fi

# dms shell: process-level check (deeper than the dms.user service entry in
# the dinit stack below). dms-session wrapper execs `dms run`; quickshell is
# the actual shell process — match both.
i=0
dms_ok=''
while [ $i -lt 30 ]; do
	if pgrep -u "$USER_NAME" -f 'dms run|quickshell' >/dev/null; then
		dms_ok=1
		break
	fi
	sleep 1
	i=$((i + 1))
done
[ -n "$dms_ok" ] && pass 'dms shell running' || fail 'dms not running'

# user dinit session stack: every boot.d user service started. dinitctl list
# renders started services as `[{+}] name (pid: ...)` — '{+' only appears
# when running. ERE anchors the name so '] niri' cannot match
# '] niri-shutdown'; adjust if the dinitctl output format differs.
if [ -S /run/dinitctl ]; then
	out=$(XDG_RUNTIME_DIR="$XDGRUN" dinitctl --user list 2>/dev/null) || out=''
	missing=''
	for s in niri dbus.user pipewire.user pipewire-pulse.user wireplumber.user polkit-agent.user dms.user wayvnc.user; do
		echo "$out" | grep -Eq '\{\+.*\] '"$s"'( |$)' || missing="$missing $s"
	done
	if [ -z "$missing" ]; then
		pass 'user dinit stack running'
	else
		fail "user services not running:$missing"
	fi
else
	fail 'no /run/dinitctl (not a dinit system)'
fi

i=0
wayvnc_ok=''
while [ $i -lt 30 ]; do
	if netstat -tln 2>/dev/null | grep -q ':5900'; then
		wayvnc_ok=1
		break
	fi
	sleep 1
	i=$((i + 1))
done
[ -n "$wayvnc_ok" ] && pass 'wayvnc listening on 5900' || fail 'wayvnc not listening'

# screenshot: real-rendering proof (same two CLI shapes as the harness —
# adjust the action name if `niri msg action --help` rejects it).
shot_ok=''
if [ -n "$niri_socket" ]; then
	rm -f /tmp/niri-shot.png
	XDG_RUNTIME_DIR="$XDGRUN" WAYLAND_DISPLAY=wayland-1 NIRI_SOCKET="$niri_socket" \
		niri msg action screenshot-screen /tmp/niri-shot.png >/dev/null 2>&1 ||
		XDG_RUNTIME_DIR="$XDGRUN" WAYLAND_DISPLAY=wayland-1 NIRI_SOCKET="$niri_socket" \
			niri msg action screenshot-screen >/dev/null 2>&1
	f=/tmp/niri-shot.png
	[ -s "$f" ] || f=$(ls "/home/$USER_NAME/Pictures/Screenshots/"*.png 2>/dev/null | tail -1)
	if [ -s "$f" ]; then
		pass "screenshot OK: $f ($(wc -c < "$f") bytes)"
		shot_ok=1
	fi
fi
[ -n "$shot_ok" ] || fail 'screenshot failed'

echo "=== summary: $((TOTAL - FAILED))/$TOTAL checks passed ==="
if [ "$FAILED" -eq 0 ]; then
	exit 0
fi
exit 1
