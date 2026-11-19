// PASSIVE autologin timing for the login_readiness verify check (niri
// family). The harness target boots with the TEST-ONLY greetd
// [initial_session] autologin, so the login happens DURING BOOT with no
// interaction — and the check measures HOW LONG it took, passively, from
// kernel timestamps (no typing, no greetd restarts, no SSH-driven input —
// the typed-credentials approach is gone: a greeter cycle spawned during
// the boot storm does not accept keyboard input for ~30-75s, run28):
//
//	t0 = greetd service start    (/proc/<pid>/stat field 22 — starttime
//	                              ticks since boot, kernel-precise)
//	t1 = user session runtime dir (/run/user/<uid> mtime — created by
//	                              elogind PAM at login; epoch → boot-
//	                              relative via /proc/uptime math)
//	t2 = wayle-session script up  (its /proc starttime — the script holds
//	                              the greetd session lifetime)
//	t3 = niri compositor up       (user niri /proc starttime)
//	t4 = wayle shell running      (pgrep)
//
// Reported deltas: greetd->runtime-dir (PAM handoff), greetd->session
// script, greetd->niri compositor, greetd->wayle shell (TOTAL desktop).
// A slow stage localizes the delay; the total >= 60s flags SLOW.
// /run/user/<uid>/wayle-session.log (the session script's own milestones, r9+)
// is appended to the detail for the intra-session breakdown.
//
// All probes run in ONE SSH exec (atomic snapshot). Everything is
// post-hoc /proc + /run state: the login happened minutes before this
// check runs, so the numbers are stable — no race, no input window.
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"feralos/tests/harness/internal/guest"
)

const (
	// loginSlowTotalSeconds — the total greetd->desktop above which the
	// check flags SLOW (and the marker loop fails it). The user's
	// real-machine symptom is a 30-60s gray window; 60s is the gate.
	loginSlowTotalSeconds = 60.0
	// loginWaitPoll — if wayle is not up yet when the check runs (early
	// verify), re-probe at this interval before reporting MISSING.
	loginWaitPoll = 2 * time.Second
	// loginWaitBudget — max time to wait for the session to appear.
	loginWaitBudget = 90 * time.Second
)

// verifyLoginReadiness measures the autologin: greetd service start ->
// user session up. Pure /proc + /run/user reading over SSH — the timing
// numbers are ALWAYS in the detail (success AND failure).
func verifyLoginReadiness(ctx context.Context, opts guest.Options, cfg *VerifyConfig) (detail string, flowErr error) {
	uidOut, err := guest.OutputOK(ctx, opts, "id -u "+shellQuote(cfg.InstalledUser))
	if err != nil {
		return "", fmt.Errorf("resolve uid of %s: %w", cfg.InstalledUser, err)
	}
	uid := strings.TrimSpace(uidOut)

	// Atomic snapshot: one SSH exec gathers every marker with labels.
	snapshotCmd := fmt.Sprintf(`g=$(pgrep greetd | head -1); [ -n "$g" ] && echo "greetd_ticks=$(awk '{print int($22/100)}' /proc/$g/stat 2>/dev/null)"
s=$(pgrep -f '[w]ayle-session' | head -1); [ -n "$s" ] && echo "session_ticks=$(awk '{print int($22/100)}' /proc/$s/stat 2>/dev/null)"
n=$(pgrep niri | head -1); [ -n "$n" ] && echo "niri_ticks=$(awk '{print int($22/100)}' /proc/$n/stat 2>/dev/null)"
[ -d /run/user/%s ] && echo "runtime_epoch=$(stat -c %%Y /run/user/%s)"
echo "uptime=$(cut -d. -f1 /proc/uptime)"
echo "now=$(date +%%s)"
pgrep -f '[w]ayle$' >/dev/null && echo "wayle_up=1" || echo "wayle_up=0"`, uid, uid)

	// Poll briefly: by the time the verify runs the session is long up —
	// the poll only covers a pathological early race.
	var snap map[string]string
	deadline := time.Now().Add(loginWaitBudget)
	for {
		out, err := guest.OutputOK(ctx, opts, snapshotCmd)
		if err != nil {
			return "", fmt.Errorf("login snapshot: %w", err)
		}
		snap = map[string]string{}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
				snap[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		if snap["wayle_up"] == "1" || time.Now().After(deadline) {
			break
		}
		if err := sleepCtx(ctx, loginWaitPoll); err != nil {
			return "", err
		}
	}

	greetdTicks, _ := strconv.ParseFloat(snap["greetd_ticks"], 64)
	sessionTicks, sessionOK := parseTick(snap["session_ticks"])
	niriTicks, niriOK := parseTick(snap["niri_ticks"])
	uptimeNow, _ := strconv.ParseFloat(snap["uptime"], 64)
	nowEpoch, _ := strconv.ParseFloat(snap["now"], 64)
	bootEpoch := nowEpoch - uptimeNow

	// runtime dir mtime (epoch) -> boot-relative seconds.
	runtimeBootRel := -1.0
	runtimeOK := snap["runtime_epoch"] != ""
	if runtimeOK {
		runtimeEpoch, _ := strconv.ParseFloat(snap["runtime_epoch"], 64)
		runtimeBootRel = runtimeEpoch - bootEpoch
	}

	if snap["greetd_ticks"] == "" {
		return "greetd process MISSING (not running?) — cannot measure", nil
	}

	detail = fmt.Sprintf("greetd start: %.0fs after boot\n", greetdTicks)
	if runtimeOK {
		detail += fmt.Sprintf("greetd->runtime-dir (PAM login): %.2fs\n", runtimeBootRel-greetdTicks)
	} else {
		detail += "greetd->runtime-dir: MISSING (no /run/user/<uid>)\n"
	}
	if sessionOK {
		detail += fmt.Sprintf("greetd->wayle-session script: %.2fs\n", sessionTicks-greetdTicks)
	} else {
		detail += "greetd->wayle-session script: MISSING\n"
	}
	if niriOK {
		detail += fmt.Sprintf("greetd->niri compositor: %.2fs\n", niriTicks-greetdTicks)
	} else {
		detail += "greetd->niri compositor: MISSING\n"
	}

	if snap["wayle_up"] != "1" {
		detail += "wayle shell MISSING (desktop never came up)\n"
		return detail, nil
	}

	// Total desktop: the best available session marker — the runtime dir
	// (PAM) is the login event; wayle up is confirmed now. The desktop is
	// UP, so total = runtime-dir delta (the login latency the user feels).
	if runtimeOK {
		total := runtimeBootRel - greetdTicks
		detail += fmt.Sprintf("total greetd->desktop (PAM): %.2fs\n", total)
		if total >= loginSlowTotalSeconds {
			detail += fmt.Sprintf("login SLOW (total >= %.0fs)\n", loginSlowTotalSeconds)
		}
	}
	// Intra-session breakdown from the session script's own milestones.
	if out, err := guest.OutputOK(ctx, opts, fmt.Sprintf("tail -20 /run/user/%s/wayle-session.log 2>/dev/null", uid)); err == nil && strings.TrimSpace(out) != "" {
		detail += "--- /run/user/<uid>/wayle-session.log ---\n" + strings.TrimSpace(out)
	}
	return detail, nil
}

// parseTick parses a /proc starttime seconds value ("empty" = absent).
func parseTick(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// sleepCtx sleeps d or returns early when ctx is canceled/expired.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
