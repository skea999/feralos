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
// IN-PLACE — no login.target cascade, no sshd kill. The [initial_session]
// stanza (stripped or restored per flow state) is re-read by the NEW
// greetd on respawn.
//
// AUTOLOGIN INTERACTION: the harness target boots with the TEST-ONLY
// greetd [initial_session] autologin (enableNiriTestAutologin), so the
// user session is already active at verify time and regreet never
// spawns. This flow therefore:
//  1. records the pre-strip baseline (active/absent — NEVER skips),
//  2. strips the [initial_session] stanza from /etc/greetd/config.toml,
//  3. kill-restarts greetd → a REAL greeter cycle with regreet,
//  4. measures the real login (password+Enter re-sent up to 3 sends
//     total, 15s after the first send then 10s apart, inside the 60s
//     desktop budget — fresh regreet GTK may reject early input),
//  5. RESTORES the stanza on EVERY path (defer-style, matching
//     enableNiriTestAutologin's exact format + duplicate guard):
//     success leaves the logged-in session running (a running greetd does
//     not re-autologin, so the restore is config-only); failure restores
//     the stanza AND kill-restarts greetd so the autologin session comes
//     back for the later DE runtime checks.
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
	// loginMeasuredMarker — present in the detail ONLY when the desktop
	// actually came up; distinguishes success (session left running,
	// config-only restore) from marker failures (restore + kill-restart
	// so autologin comes back).
	loginMeasuredMarker = "login flow measured (session left running)"
	// Password retry: first full typing, then password+Enter re-sends at
	// +15s and +25s (max 3 sends total) — regreet GTK init can ignore
	// early keyboard input.
	loginMaxPasswordSends = 3
	loginRetryFirstAfter  = 15 * time.Second
	loginRetrySpacing     = 10 * time.Second
)

// verifyLoginReadiness drives one REAL login through greetd/regreet into
// the installed user's desktop and reports the greetd->regreet,
// regreet->typing, credentials->desktop and total timings.
func verifyLoginReadiness(ctx context.Context, opts guest.Options, cfg *VerifyConfig) (detail string, flowErr error) {
	qmpPath := cfg.QMPPath
	if qmpPath == "" && cfg.RunDir != "" {
		qmpPath = filepath.Join(cfg.RunDir, "target-qmp.sock")
	}
	if qmpPath == "" {
		return "QMP socket unavailable, login flow skipped", nil
	}
	if _, err := os.Stat(qmpPath); err != nil {
		return "QMP socket unavailable, login flow skipped", nil
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

	// DE-aware session command for the [initial_session] stanza — mirrors
	// the RunConfig switch (service.go enableNiriTestAutologin call site):
	// niriWayle → "wayle-session", niri → "niri-session".
	sessionCmd := "niri-session"
	if cfg.DesktopEnvironment == desktopNiriWayle {
		sessionCmd = "wayle-session"
	}

	// Baseline BEFORE the strip (provenance only — never skips): with the
	// test autologin the session is active here; the greetd kill-restart
	// below kills it and the strip prevents the re-autologin, giving a
	// fresh greeter either way.
	baselineOut, err := guest.OutputOK(ctx, opts, fmt.Sprintf("test -e /run/user/%s && echo active || echo free", uid))
	if err != nil {
		return "", fmt.Errorf("probe existing user session: %w", err)
	}
	preStrip := "absent"
	if strings.Contains(baselineOut, "active") {
		preStrip = "active"
	}

	// Strip + restart + restore lifecycle. The stripped flag gates the
	// deferred restore so an early error (before any mutation) touches
	// nothing.
	stripped := false
	defer func() {
		if !stripped {
			return
		}
		if restoreErr := restoreAutologinStanza(ctx, opts, sessionCmd, cfg.InstalledUser); restoreErr != nil {
			detail += "\nautologin stanza restore failed: " + restoreErr.Error()
			return
		}
		flowSucceeded := flowErr == nil && strings.Contains(detail, loginMeasuredMarker)
		if !flowSucceeded {
			// Failure path: bring the autologin session back for the
			// later checks (kill-restart reads the restored stanza).
			if restartErr := killGreetdRestart(ctx, opts); restartErr != nil {
				detail += "\nautologin greetd restart failed: " + restartErr.Error()
			}
		}
	}()

	if err := stripAutologinStanza(ctx, opts); err != nil {
		return "", err
	}
	stripped = true

	// Restart the greeter cycle — t0 = greetd kill + smooth-recovery
	// respawn (NEVER dinitctl: login.target cascade, see header). This
	// also kills the pre-strip autologin session.
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
		detail = fmt.Sprintf("pre-strip user session: %s\n", preStrip) +
			fmt.Sprintf("greetd->regreet: greetd up (pid %s) but regreet process MISSING %.0fs after restart\n"+
				"greetd/regreet process MISSING", greetdPID, time.Since(t0Wall).Seconds())
		return detail, nil
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
		for _, probe := range []string{
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
		} {
			if out, err := guest.OutputOK(ctx, opts, probe); err == nil && strings.TrimSpace(out) != "" {
				d += "\n--- " + probe + " ---\n" + strings.TrimSpace(out)
			}
		}
		return d
	}
	greetdToRegreet := ticksToSeconds(t1Ticks - t0Ticks)

	detail = fmt.Sprintf("pre-strip user session: %s\ngreetd->regreet: %.2fs\n", preStrip, greetdToRegreet)

	// Settle: let the greeter UI draw its input fields before typing.
	if err := sleepCtx(ctx, 2*time.Second); err != nil {
		return detail, err
	}

	// t2: credentials typed via the QMP keyboard into the graphical VT:
	// username + Enter, 1s for the password field, password + Enter.
	if err := qemu.SendText(qmpPath, cfg.InstalledUser); err != nil {
		return detail, fmt.Errorf("type username: %w", err)
	}
	if err := qemu.SendText(qmpPath, "\n"); err != nil {
		return detail, fmt.Errorf("send Enter after username: %w", err)
	}
	if err := sleepCtx(ctx, time.Second); err != nil {
		return detail, err
	}
	if err := sendPasswordEnter(qmpPath, password); err != nil {
		return detail, fmt.Errorf("type password: %w", err)
	}
	t2Wall := time.Now()
	detail += fmt.Sprintf("typing sent at: +%.2fs after regreet\n", t2Wall.Sub(t1Wall).Seconds())

	// t3: desktop up = user runtime dir (created by elogind PAM at login)
	// + wayle session processes. The [w]ayle pattern stops pgrep from
	// matching the probe's own command line. Password+Enter is re-sent up
	// to loginMaxPasswordSends total (first at t2, retries at +15s then
	// +10s apart) — a freshly spawned regreet may not accept input yet
	// (GTK init), and a re-send over the still-focused password field is
	// idempotent-safe.
	desktopCmd := fmt.Sprintf("test -d /run/user/%s && pgrep -f '[w]ayle' >/dev/null", uid)
	passwordSends := 1
	nextRetry := t2Wall.Add(loginRetryFirstAfter)
	desktopDeadline := time.Now().Add(loginDesktopTimeout)
	for {
		if out, err := guest.OutputOK(ctx, opts, desktopCmd); err == nil && strings.TrimSpace(out) == "" {
			t3Wall := time.Now()
			credentialsToDesktop := t3Wall.Sub(t2Wall).Seconds()
			total := greetdToRegreet + t3Wall.Sub(t1Wall).Seconds()
			detail += fmt.Sprintf("credentials->desktop: %.2fs\n", credentialsToDesktop)
			detail += fmt.Sprintf("total greetd->desktop: %.2fs\n", total)
			if passwordSends > 1 {
				detail += fmt.Sprintf("password re-sends: %d\n", passwordSends-1)
			}
			detail += loginMeasuredMarker + "\n"
			if total >= loginSlowTotalSeconds {
				detail += fmt.Sprintf("login SLOW (total >= %.0fs)\n", loginSlowTotalSeconds)
			}
			return detail, nil
		}
		if time.Now().After(nextRetry) && passwordSends < loginMaxPasswordSends {
			if sendErr := sendPasswordEnter(qmpPath, password); sendErr != nil {
				return detail, fmt.Errorf("resend password: %w", sendErr)
			}
			passwordSends++
			nextRetry = time.Now().Add(loginRetrySpacing)
		}
		if time.Now().After(desktopDeadline) {
			detail += fmt.Sprintf("desktop TIMEOUT (login failed) after %s (%d password sends)\n", loginDesktopTimeout, passwordSends)
			// Freeze-frame the greeter screen (PPM in the run dir) — visual
			// ground truth of what the login attempt left on screen.
			if qmpPath != "" {
				if dumpErr := qemu.Screendump(qmpPath, filepath.Join(cfg.RunDir, "greeter-timeout.ppm")); dumpErr != nil {
					detail += fmt.Sprintf("screendump failed: %v\n", dumpErr)
				} else {
					detail += "greeter screenshot: greeter-timeout.ppm (run dir)\n"
				}
			}
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

// stripAutologinStanza deletes the test-only [initial_session] stanza
// (header + command + user lines) from /etc/greetd/config.toml and fails
// unless the marker is really gone.
func stripAutologinStanza(ctx context.Context, opts guest.Options) error {
	cmd := `sed -i '/^\[initial_session\]$/,/^user = /d' /etc/greetd/config.toml && ! grep -q initial_session /etc/greetd/config.toml`
	if _, err := guest.OutputOK(ctx, opts, cmd); err != nil {
		return fmt.Errorf("strip [initial_session] from /etc/greetd/config.toml: %w", err)
	}
	return nil
}

// restoreAutologinStanza re-appends the [initial_session] stanza EXACTLY
// like enableNiriTestAutologin writes it (Go-built stanza, printf '%b',
// duplicate guard), so the next greetd respawn re-autologins the user.
func restoreAutologinStanza(ctx context.Context, opts guest.Options, sessionCommand, installedUser string) error {
	stanza := fmt.Sprintf("\n[initial_session]\ncommand = %q\nuser = %q\n", sessionCommand, installedUser)
	script := fmt.Sprintf(`set -eu
f=/etc/greetd/config.toml
if ! grep -q 'initial_session' "$f"; then
	printf '%%b' %s >> "$f"
fi
grep -q 'initial_session' "$f" || { echo 'initial_session marker missing after restore'; exit 1; }
echo 'test-only greetd autologin stanza restored'
`, shellQuote(stanza))
	if _, err := guest.OutputOK(ctx, opts, script); err != nil {
		return fmt.Errorf("restore [initial_session] stanza: %w", err)
	}
	return nil
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
