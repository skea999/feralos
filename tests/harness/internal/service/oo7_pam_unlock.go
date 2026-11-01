package service

// oo7_pam_unlock — full login-unlock parity proof for the niriWayle oo7
// secrets stack.
//
// WHY A SPECIAL CHECK (runs 2026-10-17T17-* + live debug): the harness
// verify boot logs in via greetd [initial_session] autologin, and greetd
// 0.10.3 NEVER runs pam_authenticate for the initial session (its
// ConfigSession carries no password field; live pam_exec tripwires saw only
// account+session). PAM therefore has no authtok to stash, so pam_oo7 can
// never deliver an unlock request on a plain autologin boot — the old
// runtime grep could never pass no matter how correct the wiring was.
//
// The check therefore drives a REAL authenticated PAM login itself, on the
// very service greetd uses ("greetd"), exactly like a greeter login would.
// Transport history (runs 2026-10-17/18 + debug, source-verified):
//   - host-side `ssh -L` forward of the greetd IPC socket: DEAD — stock
//     Alpine sshd runs `AllowTcpForwarding no`, and OpenSSH's channel
//     admin layer (channel_disable_admin -> channel_connect_to_path)
//     denies ALL local forwards with it, unix/streamlocal included;
//     on top of that guest.Output's CombinedOutput merges the host-side
//     "Permanently added ..." warning into the socket path (`Bad local
//     forwarding specification`).
//   - greetd-IPC second start_session (create_session -> password ->
//     start_session): DEAD — greetd 0.10 manages ONE session: starting a
//     second session PAM-closes the autologin session c1, turnstile tears
//     down the whole user dinit (niri, secrets daemon, dbus...), the
//     pam_oo7 delivery thread races the dying/new daemon (2s retry window
//     vs +5.1s respawn) and loses; the second session itself dies when the
//     client disconnects and the greeter takes over — collateral FAILs on
//     every session-liveness check (random Go-map order).
//   - FINAL: direct PAM chain over plain exec SSH (python3 + ctypes
//     libpam: pam_start("greetd") -> pam_authenticate -> pam_open_session
//     -> close): runs against the LIVE system with the autologin session
//     untouched (daemon alive -> immediate connect, no race) and the
//     session count netting zero (turnstile/elogind open+close) — no
//     session kill, no greeter takeover, no collateral damage.
//
// pam_oo7 auto_start (in the greetd session stack) delivers the stashed
// login password to the supervised daemon's listener socket; the daemon's
// INFO line "Received unlock request" in secrets.log is the runtime proof
// (source-verified: pam_listener/mod.rs tracing::info!; requires
// RUST_LOG=info, shipped by the installer when enableDebug is on).
//
// Static asserts cover the wiring the daemon delivery depends on:
//   - pam_oo7.so on disk, greetd auth+session lines, passwd override
//   - /usr/bin/oo7-daemon recovery wrapper (aports oo7-user-dinit): the
//     pam_oo7 auto_start fallback execs this path when the listener socket
//     is missing — it must delegate to `dinitctl --user` so the daemon only
//     ever starts as a supervised user-dinit service (user rule: no
//     unsupervised session daemons, ever)
//   - /etc/dinit.d/config/secrets.conf present (+ RUST_LOG=info when the
//     fixture enables debug — the INFO line the runtime proof greps)
//   - the daemon's live PAM listener socket /run/user/$uid/oo7-pam.sock
//   - python3 present (the remote PAM client below)

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"feralos/tests/harness/internal/guest"

	"gopkg.in/yaml.v3"
)

// debugRUSTLogAssert returns the RUST_LOG=info secrets.conf assert when the
// fixture enables debug (the installer writes RUST_LOG=info only under
// enableDebug), or a no-op otherwise so the shell chain stays well-formed.
func debugRUSTLogAssert(cfg *VerifyConfig) string {
	if cfg.EnableDebug {
		return "grep -q 'RUST_LOG=info' /etc/dinit.d/config/secrets.conf || { echo 'RUST_LOG=info missing from secrets.conf (enableDebug)'; exit 1; };"
	}
	return "true;"
}

// resolveInstalledPassword returns the user's login password for the
// authenticated PAM login: explicit value when the run flow set it, else the
// variables.yaml copy the harness keeps in the run dir (standalone
// `verify` never sees RunConfig).
func resolveInstalledPassword(cfg *VerifyConfig) (string, error) {
	if cfg.InstalledPassword != "" {
		return cfg.InstalledPassword, nil
	}
	if cfg.RunDir != "" {
		if data, err := os.ReadFile(filepath.Join(cfg.RunDir, "variables.yaml")); err == nil {
			var vy VariablesYAML
			if yamlErr := yaml.Unmarshal(data, &vy); yamlErr == nil && vy.Variables.Password != "" {
				return vy.Variables.Password, nil
			}
		}
	}
	return "", fmt.Errorf("installed user password unavailable (empty InstalledPassword, no usable variables.yaml in run dir)")
}

// oo7UnlockRequestCountCmd builds the target-side count snippet for
// "Received unlock request" lines in secrets.log. The `c=$(...); echo
// ${c:-0}` shape collapses both grep's no-match exit (rc 1, prints "0") and
// a missing secrets.log (empty stdout) into a plain "0" — a bare
// `grep -c ... || echo 0` would print TWO zeros when there is no match
// (grep prints "0" AND exits 1, so the fallback appends another). Runs
// under the verify transport's `sh -lc`.
func oo7UnlockRequestCountCmd(user string) string {
	return fmt.Sprintf("c=$(grep -cF 'Received unlock request' /home/%s/.local/share/dinit/secrets.log 2>/dev/null); echo ${c:-0}", user)
}

// oo7UnlockRequestCount runs the snippet and parses the number. Parses the
// LAST non-empty line only: guest.Output merges the ssh client's stderr
// into the output (CombinedOutput), and depending on the local OpenSSH
// client a "Warning: Permanently added ... to the list of known hosts."
// line can precede the remote stdout (observed run 2026-10-18T15-03-04Z —
// Atoi choked on "Warning...\r\n0"). The snippet always prints the count
// as its own final line, so the last line is the count regardless of any
// ssh-client noise above it.
func oo7UnlockRequestCount(ctx context.Context, opts guest.Options, user string) (int, error) {
	out, err := guest.Output(ctx, opts, oo7UnlockRequestCountCmd(user))
	if err != nil {
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	n, convErr := strconv.Atoi(last)
	if convErr != nil {
		return 0, fmt.Errorf("unexpected count output %q: %w", strings.TrimSpace(string(out)), convErr)
	}
	return n, nil
}

// verifyOO7PamUnlock runs the static wiring asserts (command, registered
// under the check name) and then the authenticated PAM login (service
// "greetd") + log proof. Registered command = statics only, so a refactor
// that drops the special case still keeps the falsifiable static half.
func verifyOO7PamUnlock(ctx context.Context, opts guest.Options, cfg *VerifyConfig, staticCommand string) ([]byte, error) {
	staticOut, err := guest.Output(ctx, opts, staticCommand)
	if err != nil {
		return nil, fmt.Errorf("static asserts failed: %w", err)
	}

	password, err := resolveInstalledPassword(cfg)
	if err != nil {
		return nil, err
	}

	// Baseline for the enableDebug delta proof: count the delivery lines
	// BEFORE the login, so the poll below must observe an INCREMENT — a
	// stale "Received unlock request" line from an earlier proof on the
	// same boot can never pass the check trivially.
	unlockBefore := 0
	if cfg.EnableDebug {
		unlockBefore, err = oo7UnlockRequestCount(ctx, opts, cfg.InstalledUser)
		if err != nil {
			return nil, fmt.Errorf("secrets.log baseline count: %w", err)
		}
	}

	// REAL authenticated PAM login on service "greetd": full conversation
	// with the fixture password, then pam_open_session — pam_oo7 auto_start
	// delivers to the LIVE supervised daemon (autologin session untouched,
	// no race — see header). Runs inside the target over plain exec SSH.
	if err := pamLoginRemote(ctx, opts, cfg.InstalledUser, password); err != nil {
		return nil, fmt.Errorf("authenticated PAM login: %w", err)
	}

	var runtimeOut string
	if cfg.EnableDebug {
		// The daemon logs the delivery at INFO (RUST_LOG=info from
		// secrets.conf when enableDebug). secrets.log only gains lines from
		// the supervised user-dinit daemon, so a NEW line after the login =
		// end-to-end proof.
		proofDeadline := time.Now().Add(30 * time.Second)
		for {
			after, countErr := oo7UnlockRequestCount(ctx, opts, cfg.InstalledUser)
			if countErr == nil && after > unlockBefore {
				runtimeOut = "runtime proof OK (authenticated PAM login on service greetd -> pam_oo7 delivered the unlock request to the daemon)"
				break
			}
			if time.Now().After(proofDeadline) {
				return nil, fmt.Errorf("no new 'Received unlock request' in secrets.log after authenticated PAM login (RUST_LOG=info active?)")
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Second):
			}
		}
	} else {
		runtimeOut = "runtime log proof skipped (enableDebug=false: daemon logs no INFO lines); statics + live socket + authenticated login OK"
	}

	msg := strings.TrimSpace(string(staticOut)) + "; " + runtimeOut
	return []byte(msg), nil
}

// pamLoginSh builds the target-side script: it exports the credentials
// (base64 so arbitrary passwords survive any shell) and runs the python3
// PAM client from a quoted heredoc.
//
// The whole script travels base64-encoded (`printf %s '<b64>' | base64 -d |
// sh`): whatever login shell sits in front (fish -lc, sh -lc) only ever
// parses a plain pipeline with a single-quoted base64 blob — no heredoc,
// no globs, no shell metacharacters leak into the outer shell parser.
func pamLoginSh(username, password string) string {
	userB64 := base64.StdEncoding.EncodeToString([]byte(username))
	pwB64 := base64.StdEncoding.EncodeToString([]byte(password))

	script := fmt.Sprintf("SKEA_USER_B64='%s' SKEA_PW_B64='%s' python3 - <<'PY'\n%s\nPY\n",
		userB64, pwB64, pamLoginPy)

	b64 := base64.StdEncoding.EncodeToString([]byte(script))
	return fmt.Sprintf("printf %%s '%s' | base64 -d | sh", b64)
}

// pamLoginPy is the remote PAM client: ctypes libpam chain on the greetd
// service — pam_start -> pam_authenticate (pam_oo7 stashes the authtok) ->
// pam_open_session (pam_oo7 auto_start delivers to the live daemon; the
// daemon's listener accepts root or same-UID peers, we run as root) ->
// pam_close_session -> pam_end. Exit codes: 10 pam_start,
// 11 pam_authenticate, 12 pam_acct_mgmt, 13 pam_open_session,
// 14 pam_close_session, 15 pam_end; success prints the stdout marker
// PAM_LOGIN_OK.
//
// The PAM conversation callback must hand back malloc'd responses (PAM
// frees them with free()) — Python-owned arrays would corrupt the heap, so
// every string and the response array itself come from libc.malloc.
var pamLoginPy = `
import base64, ctypes, os, sys, time

PW = base64.b64decode(os.environ['SKEA_PW_B64'])
USERNAME = base64.b64decode(os.environ['SKEA_USER_B64']).decode()

PAM_SUCCESS = 0
PAM_PROMPT_ECHO_OFF = 1
PAM_PROMPT_ECHO_ON = 2
PAM_ERRMSG = 3
PAM_TEXT_INFO = 4

libc = ctypes.CDLL(None, use_errno=True)
libc.malloc.restype = ctypes.c_void_p
libc.malloc.argtypes = [ctypes.c_size_t]
libc.free.argtypes = [ctypes.c_void_p]

class PamMessage(ctypes.Structure):
    _fields_ = [('msg_style', ctypes.c_int), ('msg', ctypes.c_char_p)]

class PamResponse(ctypes.Structure):
    _fields_ = [('resp', ctypes.c_char_p), ('resp_retcode', ctypes.c_int)]

ConvFn = ctypes.CFUNCTYPE(
    ctypes.c_int,
    ctypes.c_int,
    ctypes.POINTER(ctypes.POINTER(PamMessage)),
    ctypes.POINTER(ctypes.POINTER(PamResponse)),
    ctypes.c_void_p,
)

class PamConv(ctypes.Structure):
    _fields_ = [('conv', ConvFn), ('appdata_ptr', ctypes.c_void_p)]

def _malloc_str(data):
    buf = libc.malloc(len(data) + 1)
    if not buf:
        return None
    ctypes.memmove(buf, data + b'\x00', len(data) + 1)
    return ctypes.cast(buf, ctypes.c_char_p)

@ConvFn
def _conv(num_msg, msg_p, resp_p, appdata):
    try:
        arr_buf = libc.malloc(ctypes.sizeof(PamResponse) * num_msg)
        if not arr_buf:
            return 5  # PAM_BUF_ERR
        arr = ctypes.cast(arr_buf, ctypes.POINTER(PamResponse))
        for i in range(num_msg):
            style = msg_p[i].contents.msg_style
            if style in (PAM_PROMPT_ECHO_OFF, PAM_PROMPT_ECHO_ON):
                arr[i].resp = _malloc_str(PW)
            else:
                arr[i].resp = None
            arr[i].resp_retcode = 0
        resp_p[0] = arr
        return PAM_SUCCESS
    except Exception:
        return 19  # PAM_CONV_ERR

# RTLD_GLOBAL is MANDATORY: pam_oo7.so (oo7 0.6.0-r4) carries no
# DT_NEEDED libpam.so.0 and leaves pam_get_item/pam_set_data/pam_get_data/
# pam_get_user undefined — its symbols resolve only from the process
# GLOBAL scope. With the ctypes default (RTLD_LOCAL) musl dlopen fails,
# PAM skips the optional module silently, no stash ever exists and the
# delivery never happens (chain still rc=0 — run 2026-10-18T02-28-16Z +
# live probe matrix E2 rc=28 "Module is unknown" vs E3 rc=0+delivery).
# greetd itself links libpam (NEEDED) so real logins are unaffected — the
# product PAM stack is correct as shipped.
libpam = ctypes.CDLL('libpam.so.0', use_errno=True, mode=ctypes.RTLD_GLOBAL)
libpam.pam_start.argtypes = [ctypes.c_char_p, ctypes.c_char_p,
                             ctypes.POINTER(PamConv),
                             ctypes.POINTER(ctypes.c_void_p)]
libpam.pam_start.restype = ctypes.c_int
libpam.pam_authenticate.argtypes = [ctypes.c_void_p, ctypes.c_int]
libpam.pam_authenticate.restype = ctypes.c_int
libpam.pam_acct_mgmt.argtypes = [ctypes.c_void_p, ctypes.c_int]
libpam.pam_acct_mgmt.restype = ctypes.c_int
libpam.pam_open_session.argtypes = [ctypes.c_void_p, ctypes.c_int]
libpam.pam_open_session.restype = ctypes.c_int
libpam.pam_close_session.argtypes = [ctypes.c_void_p, ctypes.c_int]
libpam.pam_close_session.restype = ctypes.c_int
libpam.pam_end.argtypes = [ctypes.c_void_p, ctypes.c_int]
libpam.pam_end.restype = ctypes.c_int
libpam.pam_strerror.argtypes = [ctypes.c_void_p, ctypes.c_int]
libpam.pam_strerror.restype = ctypes.c_char_p

def _fail(step, code, pamh):
    err = libpam.pam_strerror(pamh, code) if pamh else b'?'
    sys.stderr.write('%s failed: rc=%d %s\n' % (step, code, err.decode(errors='replace')))
    sys.exit(code_map[step])

code_map = {'pam_start': 10, 'pam_authenticate': 11, 'pam_acct_mgmt': 12,
            'pam_open_session': 13, 'pam_close_session': 14, 'pam_end': 15}

conv = PamConv(_conv, None)
pamh = ctypes.c_void_p()

rc = libpam.pam_start(b'greetd', USERNAME.encode(), ctypes.byref(conv),
                      ctypes.byref(pamh))
if rc != PAM_SUCCESS:
    _fail('pam_start', rc, None)

# Real login chain on the greetd service: authenticate stashes the authtok
# (pam_oo7 auth), open_session runs the session stack including the
# 'session optional pam_oo7.so auto_start' line (delivery to the live daemon).
rc = libpam.pam_authenticate(pamh, 0)
if rc != PAM_SUCCESS:
    _fail('pam_authenticate', rc, pamh)

rc = libpam.pam_acct_mgmt(pamh, 0)
if rc != PAM_SUCCESS:
    _fail('pam_acct_mgmt', rc, pamh)

rc = libpam.pam_open_session(pamh, 0)
if rc != PAM_SUCCESS:
    _fail('pam_open_session', rc, pamh)

# pam_oo7 delivers from a background thread (fork + setuid + connect +
# send) — give it time while the handle (and process) stays alive.
time.sleep(2)

rc = libpam.pam_close_session(pamh, 0)
if rc != PAM_SUCCESS:
    _fail('pam_close_session', rc, pamh)

rc = libpam.pam_end(pamh, rc if rc != PAM_SUCCESS else 0)
if rc != PAM_SUCCESS:
    sys.stderr.write('pam_end failed: rc=%d\n' % rc)
    sys.exit(15)

print('PAM_LOGIN_OK')
`

// pamLoginRemote runs the authenticated PAM login chain on the TARGET over
// plain exec SSH (no forwarding, no second greetd session — see header).
// guest.Output's CombinedOutput embeds remote stderr on failure, so every
// PAM step error surfaces in the returned Go error.
func pamLoginRemote(ctx context.Context, opts guest.Options, username, password string) error {
	out, err := guest.Output(ctx, opts, pamLoginSh(username, password))
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "PAM_LOGIN_OK") {
		return fmt.Errorf("PAM login chain did not complete (no PAM_LOGIN_OK marker): %s", strings.TrimSpace(string(out)))
	}
	return nil
}
