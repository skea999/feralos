package service

// Real-login timing flow for the login_readiness verify check (niri
// family). Unlike the other checks (single shell lines), this one needs
// MULTI-STEP SSH + QMP + polling, so it is special-cased in Verify like
// verifyOO7PamUnlock; verify_builders.go registers the placeholder
// "special-case: verifyLoginReadiness" so the loop still iterates it.
//
// Measured timeline:
//	t0 = greetd restart (kill + smooth-recovery respawn) (wall + /proc ticks)
//	t1 = regreet process up              (regreet /proc starttime ticks — precise)
//	t2 = credentials typed               (QMP input-send-event into the graphical VT)
//	t3 = desktop up                      (/run/user/<uid> + wayle processes)
//
// The timing numbers are ALWAYS in the detail so every run establishes
// the baseline. Failures ride marker words (MISSING/TIMEOUT/SLOW) in the
// detail so the service.go marker loop fails the check with the numbers
// preserved.
//
// RESTART MECHANISM (run20 ground truth): dinitctl stop greetd CASCADES
// to login.target (greetd is `before = login.target`) — stopping greetd
// stops login.target and the FULL dependent chain (sshd, cupsd, ...),
// which killed sshd and stranded 28 checks with "context deadline
// exceeded". greetd is therefore NEVER stopped/started via dinitctl
// anywhere in this flow: it is killed by pid (`/bin/busybox kill`, see
// killGreetdProc) and its dinit unit's smooth-recovery respawns it
// IN-PLACE — no login.target cascade, no sshd kill.
//
// BOOT-STORM INPUT WINDOW (run28 ground truth, measured on the run disk
// with debug_run): a greeter cycle spawned during the early-boot load
// storm (auditd+tracee+clamd+nix still loading) does NOT accept keyboard
// input for ~30-75s — the QMP events reach QEMU but nothing arrives at
// regreet (zero PAM events in auth.log), so password-only re-sends can
// never fix lost typing (the fields stay EMPTY). The typing therefore
// retries the FULL SEQUENCE (username+Enter, password+Enter) at ~20s
// spacing across the whole desktop budget — measured: a send landing
// past the storm window logs in within 30s, and a settled greeter logs
// in within 13s. QMP screendump is NOT usable as evidence on the harness
// QEMU config (egl-headless + virtio-vga-gl: `no surface` — GL surfaces
// cannot be captured) and was removed.
//
// Degenerate case: QMP socket unavailable (standalone `verify` without a
// run dir) reports informationally with PASS before touching anything.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"feralos/tests/harness/internal/guest"
	"feralos/tests/harness/internal/qemu"
)

const (
	loginRegreetTimeout   = 30 * time.Second
	loginRegreetInterval  = 500 * time.Millisecond
	loginDesktopTimeout   = 600 * time.Second
	loginDesktopInterval  = time.Second
	loginSlowTotalSeconds = 60.0
	loginProcTicksPerSec  = 100 // /proc starttime ticks per second (USER_HZ)
	// loginFlowBudget — the flow's OWN context budget (run27/28): a failing
	// login must degrade ONE check, not the whole verify. 9 min covers the
	// 600s desktop budget + waits; the shared verify budget is 15 min.
	loginFlowBudget = 9 * time.Minute
	// loginMeasuredMarker — present in the detail ONLY when the desktop
	// actually came up; distinguishes success (session left running,
	// config-only restore) from marker failures (restore + kill-restart
	// so autologin comes back).
	loginMeasuredMarker = "login flow measured (session left running)"
	// Password retry: FULL-SEQUENCE re-sends every 20s (max 8 = spans the
	// ~30-75s boot-storm input window measured in run28 and lands well
	// inside the 600s desktop budget).
	loginMaxSequenceSends = 8
	loginRetrySpacing     = 20 * time.Second
)

// verifyLoginReadiness drives one REAL login through greetd/regreet into
// the installed user's desktop and reports the greetd->regreet,
// regreet->typing, credentials->desktop and total timings.
func verifyLoginReadiness(ctx context.Context, opts guest.Options, cfg *VerifyConfig) (detail string, flowErr error) {
	// Own budget (see loginFlowBudget): a failing/slow login degrades ONE
	// check — the shared verify ctx keeps room for the remaining checks.
	var cancelFlow context.CancelFunc
	ctx, cancelFlow = context.WithTimeout(ctx, loginFlowBudget)
	defer cancelFlow()

	qmpPath := cfg.QMPPath
	if qmpPath == "" && cfg.RunDir != "" {
		qmpPath = filepath.Join(cfg.RunDir, "target-qmp.sock")
	}
	if qmpPath == "" {
		return "QMP socket unavailable, login flow skipped", nil
	}
	if _, err := os.Stat(qmpPath); err != nil {
		// Intentional informational skip (nilerr): no QMP socket = standalone
		// verify without a run dir — the flow cannot run, nothing failed.
		return "QMP socket unavailable, login flow skipped", nil //nolint:nilerr
	}

	password, flowErr := resolveInstalledPassword(cfg)
	if flowErr != nil {
		return "", flowErr
	}
	// Fail loud BEFORE typing: one unmapped char would otherwise leave
	// half-typed credentials sitting in the greeter fields.
	if _, err := qemu.TextQCodes(cfg.InstalledUser); err != nil {
		return "", fmt.Errorf("unsupported char: %w (username)", err)
	}
	if _, err := qemu.TextQCodes(password); err != nil {
		return "", fmt.Errorf("unsupported char: %w (password)", err)
	}

	uidOut, err := guest.OutputOK(ctx, opts, "id -u "+shellQuote(cfg.InstalledUser))
	if err != nil {
		return "", fmt.Errorf("resolve uid of %s: %w", cfg.InstalledUser, err)
	}
	uid := strings.TrimSpace(uidOut)

	// Restart the greeter cycle — t0 = greetd kill + smooth-recovery
	// respawn (NEVER dinitctl: login.target cascade, see header). With the
	// harness autologin REMOVED for the niri family (2026-11-18), the boot
	// greeter waits indefinitely and this kill starts a FRESH greeter
	// cycle the timing can be measured from.
	if err := killGreetdRestart(ctx, opts); err != nil {
		return "", err
	}
	t0Wall := time.Now()

	greetdPID, t0Ticks, flowErr := waitProcTicks(ctx, opts, "pgrep greetd | head -1", 5*time.Second, "")
	if flowErr != nil {
		return "", fmt.Errorf("greetd pid/starttime after restart: %w", flowErr)
	}

	// t1: regreet up (login screen process). Both endpoints are /proc
	// starttime ticks — kernel-precise, no wall-clock math.
	regreetPID, flowErr := waitProcPID(ctx, opts, "pgrep regreet | head -1", loginRegreetTimeout, loginRegreetInterval, "")
	if flowErr != nil {
		detail = fmt.Sprintf("greetd->regreet: greetd up (pid %s) but regreet process MISSING %.0fs after restart\n"+
			"greetd/regreet process MISSING", greetdPID, time.Since(t0Wall).Seconds())
		// Intentional informational outcome (nilerr): the measured FAILURE is
		// the result — the detail carries it, the marker loop flags it.
		return detail, nil //nolint:nilerr
	}
	t1Wall := time.Now()
	t1Ticks, flowErr := procStartTicks(ctx, opts, regreetPID)
	if flowErr != nil {
		return "", fmt.Errorf("regreet starttime (pid %s): %w", regreetPID, flowErr)
	}

	// Desktop TIMEOUT path: capture greetd session evidence into the detail
	// (session log FULL — niri's post-start lines are the diagnosis when the
	// compositor never presents a frame; QEMU screendump "no surface" =
	// no frame reached the display) + greeter environment probes (DRM
	// access, groups, compositor alive) + FONTCONFIG state (the prime
	// slow-login suspect: greeter cache ownership/size vs the font corpus).
	captureGreeterEvidence := func(d string) string {
		probes := []string{
			"cat /var/lib/greetd/session.log 2>/dev/null",
			"cat /etc/greetd/config.toml",
			"ls /usr/share/wayland-sessions/ 2>/dev/null",
			"pgrep -l regreet; pgrep -l niri",
			"id greeter",
			"ls -la /dev/dri/ 2>/dev/null",
			"ls /tmp/greetd-runtime 2>/dev/null | head -5",
			"ls -la /var/lib/greetd/.cache/fontconfig 2>/dev/null | head -8",
			"du -sh /var/lib/greetd/.cache 2>/dev/null",
			"find /usr/share/fonts -type f 2>/dev/null | wc -l",
			"ls -la /var/cache/fontconfig 2>/dev/null | head -6",
		}
		// The session milestone log (installer-created, /var/log).
		probes = append([]string{"tail -20 /var/log/wayle-session.log 2>/dev/null"}, probes...)
		for _, probe := range probes {
			if out, err := guest.OutputOK(ctx, opts, probe); err == nil && strings.TrimSpace(out) != "" {
				d += "\n--- " + probe + " ---\n" + strings.TrimSpace(out)
			}
		}
		return d
	}
	greetdToRegreet := ticksToSeconds(t1Ticks - t0Ticks)

	detail = fmt.Sprintf("greetd->regreet: %.2fs\n", greetdToRegreet)

	// Settle: let the greeter UI draw its input fields before typing.
	if err := sleepCtx(ctx, 2*time.Second); err != nil {
		return detail, err
	}

	// t2: credentials typed via the QMP keyboard into the graphical VT:
	// username + Enter, 1s for the password field, password + Enter.
	// FULL-SEQUENCE typing (run28): the re-sends must repeat the WHOLE
	// username+password sequence — a send landing inside the boot-storm
	// input window is lost ENTIRELY (fields stay empty), so password-only
	// re-sends can never recover it. Sends at ~+3s, then every 20s across
	// the desktop budget; the first send landing past the storm window
	// (~30-75s after respawn) logs in within 30s.
	typeFullSequence := func() error {
		if err := qemu.SendText(qmpPath, cfg.InstalledUser); err != nil {
			return fmt.Errorf("type username: %w", err)
		}
		if err := qemu.SendText(qmpPath, "\n"); err != nil {
			return fmt.Errorf("send Enter after username: %w", err)
		}
		if err := sleepCtx(ctx, time.Second); err != nil {
			return err
		}
		return sendPasswordEnter(qmpPath, password)
	}
	if err := typeFullSequence(); err != nil {
		return detail, fmt.Errorf("type credentials: %w", err)
	}
	t2Wall := time.Now()
	detail += fmt.Sprintf("typing sent at: +%.2fs after regreet\n", t2Wall.Sub(t1Wall).Seconds())

	// t3: desktop up = user runtime dir (created by elogind PAM at login)
	// + wayle session processes. The [w]ayle pattern stops pgrep from
	// matching the probe's own command line. FULL-SEQUENCE re-sends every
	// loginRetrySpacing seconds (loginMaxSequenceSends total) — sends
	// landing inside the boot-storm input window are lost wholesale
	// (run28: zero PAM events for 3 sends in 25s; a send at +75s logged
	// in within 30s).
	desktopCmd := fmt.Sprintf("test -d /run/user/%s && pgrep -f '[w]ayle' >/dev/null", uid)
	passwordSends := 1
	nextRetry := t2Wall.Add(loginRetrySpacing)
	desktopDeadline := time.Now().Add(loginDesktopTimeout)
	for {
		if out, err := guest.OutputOK(ctx, opts, desktopCmd); err == nil && strings.TrimSpace(out) == "" {
			t3Wall := time.Now()
			credentialsToDesktop := t3Wall.Sub(t2Wall).Seconds()
			total := greetdToRegreet + t3Wall.Sub(t1Wall).Seconds()
			detail += fmt.Sprintf("credentials->desktop: %.2fs\n", credentialsToDesktop)
			detail += fmt.Sprintf("total greetd->desktop: %.2fs\n", total)
			if passwordSends > 1 {
				detail += fmt.Sprintf("sequence re-sends: %d\n", passwordSends-1)
			}
			detail += loginMeasuredMarker + "\n"
			if total >= loginSlowTotalSeconds {
				detail += fmt.Sprintf("login SLOW (total >= %.0fs)\n", loginSlowTotalSeconds)
			}
			return detail, nil
		}
		if time.Now().After(nextRetry) && passwordSends < loginMaxSequenceSends {
			if sendErr := typeFullSequence(); sendErr != nil {
				return detail, fmt.Errorf("re-send credentials: %w", sendErr)
			}
			passwordSends++
			detail += fmt.Sprintf("sequence re-send #%d at +%.0fs after regreet\n", passwordSends, time.Since(t1Wall).Seconds())
			nextRetry = time.Now().Add(loginRetrySpacing)
		}
		if time.Now().After(desktopDeadline) {
			detail += fmt.Sprintf("desktop TIMEOUT (login failed) after %s (%d sequence sends)\n", loginDesktopTimeout, passwordSends)
			detail = captureGreeterEvidence(detail)
			return detail, nil
		}
		if err := sleepCtx(ctx, loginDesktopInterval); err != nil {
			return detail, err
		}
	}
}

// sendPasswordEnter types the password into the focused greeter field and
// presses Enter (used for the first send AND the retries — after a
// rejected/ignored first send the password field is still focused).
func sendPasswordEnter(qmpPath, password string) error {
	if err := qemu.SendText(qmpPath, password); err != nil {
		return err
	}
	return qemu.SendText(qmpPath, "\n")
}

// killGreetdRestart kills greetd and waits for smooth-recovery (greetd's
// dinit unit) to spawn a NEW greetd in place. NEVER use dinitctl stop/start
// here: greetd is `before = login.target`, so dinitctl stop greetd stops
// login.target and its FULL dependent chain (sshd, cupsd, ...) — run20
// lost sshd and 28 checks this way. A pure process kill does not touch
// login.target; the new greetd re-reads /etc/greetd/config.toml on respawn
// (stripped or restored stanza per flow state).
//
// KILL MECHANISM (run21 ground truth): a bare `sudo -n kill <pid>` over
// SSH returned exit 1 with NO error text (sudoers is NOPASSWD:ALL — not a
// sudo-password failure; likely PATH resolution of `kill` in the non-login
// SSH shell, a stale pid, or busybox-vs-procps ambiguity). The kill uses
// the explicit `/bin/busybox kill` binary (always present on Alpine base;
// procps is not), each attempt captures its rc via `echo "kill_rc=$?"`,
// and total failure returns an error carrying captured outputs + probes
// (id, sudo-ok, pgrep, which kill) so the next run diagnoses in one shot.
func killGreetdRestart(ctx context.Context, opts guest.Options) error {
	oldPID := ""
	if out, err := guest.OutputOK(ctx, opts, "pgrep greetd | head -1"); err == nil {
		oldPID = strings.TrimSpace(out)
	}

	if err := killGreetdProc(ctx, opts, oldPID); err != nil {
		return err
	}

	if _, err := waitProcPID(ctx, opts, "pgrep greetd | head -1", 10*time.Second, 200*time.Millisecond, oldPID); err != nil {
		return fmt.Errorf("greetd respawn after kill: %w", err)
	}
	return nil
}

// killGreetdProc kills the greetd process gpid (empty = not running —
// nothing to kill; the respawn wait in killGreetdRestart reports if no
// greetd appears). Sanity-checks the pid against /proc/<pid>/comm first
// (a stale pid would fail the kill with a bare exit 1), then walks
// busybox kill → /bin/kill -TERM → /bin/kill -KILL, each rc captured.
func killGreetdProc(ctx context.Context, opts guest.Options, gpid string) error {
	if gpid == "" {
		return nil
	}
	if _, err := strconv.Atoi(gpid); err != nil {
		return fmt.Errorf("greetd pid %q is not numeric — refusing to kill", gpid)
	}

	// Stale-pid guard: the pid must still BE greetd.
	commOut, commErr := guest.OutputOK(ctx, opts, fmt.Sprintf("cat /proc/%s/comm", gpid))
	comm := strings.TrimSpace(commOut)
	if commErr != nil {
		return fmt.Errorf("stale greetd pid %s: read /proc/%s/comm: %w", gpid, gpid, commErr)
	}
	if comm != "greetd" {
		return fmt.Errorf("stale greetd pid %s: /proc/%s/comm = %q, want greetd", gpid, gpid, comm)
	}

	// kill_rc is embedded in the output because OutputOK collapses the
	// remote exit code; every command ends in `echo`, so the ssh exit is
	// 0 either way — the captured kill_rc=N line is the real verdict.
	attempts := []string{
		fmt.Sprintf(`sudo -n /bin/busybox kill %q 2>&1; echo "kill_rc=$?"`, gpid),
		fmt.Sprintf(`sudo -n /bin/kill -TERM %q 2>&1; echo "kill_rc=$?"`, gpid),
		fmt.Sprintf(`sudo -n /bin/kill -KILL %q 2>&1; echo "kill_rc=$?"`, gpid),
	}
	captured := make([]string, 0, len(attempts))
	for _, cmd := range attempts {
		out, err := guest.OutputOK(ctx, opts, cmd)
		out = strings.TrimSpace(out)
		if err == nil && strings.Contains(out, "kill_rc=0") {
			return nil
		}
		entry := cmd + " -> " + out
		if err != nil {
			entry += " (ssh: " + err.Error() + ")"
		}
		captured = append(captured, entry)
	}

	// Total failure: one-shot diagnostic payload for the run artifacts.
	diag := append([]string{}, captured...)
	for _, probe := range []string{
		"id -u",
		"sudo -n true && echo sudo-ok",
		"pgrep greetd",
		"which kill",
	} {
		if out, err := guest.OutputOK(ctx, opts, probe); err == nil {
			diag = append(diag, probe+" -> "+strings.TrimSpace(out))
		} else {
			diag = append(diag, probe+" -> ssh error: "+err.Error())
		}
	}
	return fmt.Errorf("kill greetd failed (pid %s), all %d attempts:\n%s", gpid, len(attempts), strings.Join(diag, "\n"))
}

// waitProcPID polls `cmd` over SSH until it yields a pid different from
// excludePID ("" = accept any).
func waitProcPID(ctx context.Context, opts guest.Options, cmd string, timeout, interval time.Duration, excludePID string) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		out, err := guest.OutputOK(ctx, opts, cmd)
		pid := strings.TrimSpace(out)
		if err == nil && pid != "" && pid != excludePID {
			return pid, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("%s: process not found within %s", cmd, timeout)
		}
		if err := sleepCtx(ctx, interval); err != nil {
			return "", err
		}
	}
}

// waitProcTicks waits for the process, then reads its /proc starttime in
// clock ticks since boot.
func waitProcTicks(ctx context.Context, opts guest.Options, cmd string, timeout time.Duration, excludePID string) (string, int64, error) {
	pid, err := waitProcPID(ctx, opts, cmd, timeout, 200*time.Millisecond, excludePID)
	if err != nil {
		return "", 0, err
	}
	ticks, err := procStartTicks(ctx, opts, pid)
	if err != nil {
		return "", 0, err
	}
	return pid, ticks, nil
}

// procStartTicks reads /proc/<pid>/stat field 22 (starttime, clock ticks
// since boot). greetd/regreet comms contain no spaces, so awk $22 is safe.
func procStartTicks(ctx context.Context, opts guest.Options, pid string) (int64, error) {
	out, err := guest.OutputOK(ctx, opts, fmt.Sprintf("awk '{print $22}' /proc/%s/stat", pid))
	if err != nil {
		return 0, fmt.Errorf("read /proc/%s/stat: %w", pid, err)
	}
	ticks, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse starttime for pid %s (%q): %w", pid, strings.TrimSpace(out), err)
	}
	return ticks, nil
}

func ticksToSeconds(ticks int64) float64 {
	return float64(ticks) / float64(loginProcTicksPerSec)
}

// sleepCtx sleeps for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
