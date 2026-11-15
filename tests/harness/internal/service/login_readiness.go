package service

// Real-login timing flow for the login_readiness verify check (niri
// family). Unlike the other checks (single shell lines), this one needs
// MULTI-STEP SSH + QMP + polling, so it is special-cased in Verify like
// verifyOO7PamUnlock; verify_builders.go registers the placeholder
// "special-case: verifyLoginReadiness" so the loop still iterates it.
//
// Measured timeline:
//	t0 = greetd service (re)start   (wall now + greetd /proc starttime ticks)
//	t1 = regreet process up         (regreet /proc starttime ticks — precise)
//	t2 = credentials typed          (QMP input-send-event into the graphical VT)
//	t3 = desktop up                 (/run/user/<uid> + wayle processes)
//
// The timing numbers are ALWAYS in the detail so every run establishes
// the baseline. Failures ride marker words (MISSING/TIMEOUT/SLOW) in the
// detail so the service.go marker loop fails the check with the numbers
// preserved.
//
// AUTOLOGIN INTERACTION (critical): the harness target boots with the
// TEST-ONLY greetd [initial_session] autologin (enableNiriTestAutologin),
// so the user session is already active at verify time and regreet never
// spawns. This flow therefore:
//  1. records the pre-strip baseline (active/absent — NEVER skips),
//  2. strips the [initial_session] stanza from /etc/greetd/config.toml,
//  3. restarts greetd → a REAL greeter cycle with regreet,
//  4. measures the real login,
//  5. RESTORES the stanza on EVERY path (defer-style, matching
//     enableNiriTestAutologin's exact format + duplicate guard):
//     success leaves the logged-in session running (a running greetd does
//     not re-autologin, so the restore is config-only); failure restores
//     the stanza AND restarts greetd so the autologin session comes back
//     for the later DE runtime checks.
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
	loginDesktopTimeout   = 60 * time.Second
	loginDesktopInterval  = time.Second
	loginSlowTotalSeconds = 60.0
	loginProcTicksPerSec  = 100 // /proc starttime ticks per second (USER_HZ)
	// loginMeasuredMarker — present in the detail ONLY when the desktop
	// actually came up; distinguishes success (session left running,
	// config-only restore) from marker failures (restore + greetd
	// restart so autologin comes back).
	loginMeasuredMarker = "login flow measured (session left running)"
)

// verifyLoginReadiness drives one REAL login through greetd/regreet into
// the installed user's desktop and reports the greetd->regreet,
// regreet->typing, credentials->desktop and total timings.
func verifyLoginReadiness(ctx context.Context, opts guest.Options, cfg *VerifyConfig) (detail string, err error) {
	qmpPath := cfg.QMPPath
	if qmpPath == "" && cfg.RunDir != "" {
		qmpPath = filepath.Join(cfg.RunDir, "target-qmp.sock")
	}
	if qmpPath == "" {
		return "QMP socket unavailable, login flow skipped", nil
	}
	if _, serr := os.Stat(qmpPath); serr != nil {
		return "QMP socket unavailable, login flow skipped", nil
	}

	password, err := resolveInstalledPassword(cfg)
	if err != nil {
		return "", err
	}
	// Fail loud BEFORE typing: one unmapped char would otherwise leave
	// half-typed credentials sitting in the greeter fields.
	if _, uerr := qemu.TextQCodes(cfg.InstalledUser); uerr != nil {
		return "", fmt.Errorf("unsupported char: %w (username)", uerr)
	}
	if _, perr := qemu.TextQCodes(password); perr != nil {
		return "", fmt.Errorf("unsupported char: %w (password)", perr)
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
	// test autologin the session is active here; the greetd restart below
	// kills it and the strip prevents the re-autologin, giving a fresh
	// greeter either way.
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
		flowSucceeded := err == nil && strings.Contains(detail, loginMeasuredMarker)
		if !flowSucceeded {
			// Failure path: bring the autologin session back for the
			// later checks (stop+start with the restored stanza). Also
			// covers a half-failed restart (greetd left stopped).
			if restartErr := restartGreetd(ctx, opts); restartErr != nil {
				detail += "\nautologin greetd restart failed: " + restartErr.Error()
			}
		}
	}()

	if serr := stripAutologinStanza(ctx, opts); serr != nil {
		return "", serr
	}
	stripped = true

	// Restart the greeter cycle — t0 = greetd (re)start. This also kills
	// the pre-strip autologin session.
	if rerr := restartGreetd(ctx, opts); rerr != nil {
		return "", rerr
	}
	t0Wall := time.Now()

	greetdPID, t0Ticks, err := waitProcTicks(ctx, opts, "pgrep greetd | head -1", 5*time.Second, "")
	if err != nil {
		return "", fmt.Errorf("greetd pid/starttime after restart: %w", err)
	}

	// t1: regreet up (login screen process). Both endpoints are /proc
	// starttime ticks — kernel-precise, no wall-clock math.
	regreetPID, err := waitProcPID(ctx, opts, "pgrep regreet | head -1", loginRegreetTimeout, loginRegreetInterval, "")
	if err != nil {
		detail = fmt.Sprintf("pre-strip user session: %s\n", preStrip) +
			fmt.Sprintf("greetd->regreet: greetd up (pid %s) but regreet process MISSING %.0fs after restart\n"+
				"greetd/regreet process MISSING", greetdPID, time.Since(t0Wall).Seconds())
		return detail, nil
	}
	t1Wall := time.Now()
	t1Ticks, err := procStartTicks(ctx, opts, regreetPID)
	if err != nil {
		return "", fmt.Errorf("regreet starttime (pid %s): %w", regreetPID, err)
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
	if err := qemu.SendText(qmpPath, password); err != nil {
		return detail, fmt.Errorf("type password: %w", err)
	}
	if err := qemu.SendText(qmpPath, "\n"); err != nil {
		return detail, fmt.Errorf("send Enter after password: %w", err)
	}
	t2Wall := time.Now()
	detail += fmt.Sprintf("typing sent at: +%.2fs after regreet\n", t2Wall.Sub(t1Wall).Seconds())

	// t3: desktop up = user runtime dir (created by elogind PAM at login)
	// + wayle session processes. The [w]ayle pattern stops pgrep from
	// matching the probe's own command line.
	desktopCmd := fmt.Sprintf("test -d /run/user/%s && pgrep -f '[w]ayle' >/dev/null", uid)
	desktopDeadline := time.Now().Add(loginDesktopTimeout)
	for {
		if out, err := guest.OutputOK(ctx, opts, desktopCmd); err == nil && strings.TrimSpace(out) == "" {
			t3Wall := time.Now()
			credentialsToDesktop := t3Wall.Sub(t2Wall).Seconds()
			total := greetdToRegreet + t3Wall.Sub(t1Wall).Seconds()
			detail += fmt.Sprintf("credentials->desktop: %.2fs\n", credentialsToDesktop)
			detail += fmt.Sprintf("total greetd->desktop: %.2fs\n", total)
			detail += loginMeasuredMarker + "\n"
			if total >= loginSlowTotalSeconds {
				detail += fmt.Sprintf("login SLOW (total >= %.0fs)\n", loginSlowTotalSeconds)
			}
			return detail, nil
		}
		if time.Now().After(desktopDeadline) {
			detail += fmt.Sprintf("desktop TIMEOUT (login failed) after %s\n", loginDesktopTimeout)
			return detail, nil
		}
		if err := sleepCtx(ctx, loginDesktopInterval); err != nil {
			return detail, err
		}
	}
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
// duplicate guard), so a later greetd restart re-autologins the user.
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

// restartGreetd cycles the greetd service: sudo -n first
// (non-interactive), plain dinitctl fallback (root SSH needs neither).
func restartGreetd(ctx context.Context, opts guest.Options) error {
	if _, err := guest.OutputOK(ctx, opts, "sudo -n dinitctl stop greetd && sleep 1 && sudo -n dinitctl start greetd"); err != nil {
		if _, fallbackErr := guest.OutputOK(ctx, opts, "dinitctl stop greetd && sleep 1 && dinitctl start greetd"); fallbackErr != nil {
			return fmt.Errorf("restart greetd (sudo -n: %w, plain: %w)", err, fallbackErr)
		}
	}
	return nil
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
