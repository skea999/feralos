package internal

import (
	"feralos/src/utils/config"
	"feralos/src/utils/shell"
	"fmt"
	"strings"
)

// SetupSecurityStack configures the BASE security stack (docs/architecture/
// security_stack.md): auditd (kernel audit framework), AIDE (file integrity),
// fail2ban (brute-force, nftables banaction). Packages + service enablement
// are rules-driven (internal_rules.yaml standardPackages + services map); the
// conversion loop (packages.go) installs the -dinit counterparts; THIS owner
// file writes only the config files those services consume. clamav is
// EnableHardening-gated and owned by features/hardening.go — never here.
//
// Called as chroot_phase step 2b (right after SetupLogging — both need the
// packages from step 1 already installed).
func SetupSecurityStack(conf *config.Config, pkgList string) error {
	chroot := conf.Internal.ChrootDir

	if installed(pkgList, "audit") {
		if err := writeAuditRules(chroot); err != nil {
			return err
		}
	} else {
		fmt.Println("[INFO] audit not installed, skipping audit rules")
	}

	if installed(pkgList, "fail2ban") {
		if err := writeFail2banJail(chroot); err != nil {
			return err
		}
	} else {
		fmt.Println("[INFO] fail2ban not installed, skipping jail config")
	}

	if installed(pkgList, "aide") {
		if err := writeAideConfig(chroot); err != nil {
			return err
		}
	} else {
		fmt.Println("[INFO] aide not installed, skipping integrity config")
	}

	fmt.Println("[INFO] Security stack configured.")
	return nil
}

// BaselineAide generates the AIDE baseline database IN-CHROOT at the very end
// of ChrootPhase.Run (step 18, after the edge install — the last step that
// can touch watched paths): the baseline must capture the FINAL filesystem
// state or every install-time change would surface as first-boot drift.
// Outcome is verified by DB-FILE EXISTENCE, not by the command exit: in
// best-effort runs ExecuteCmd swallows failures (error=nil) — the harness
// check on /var/lib/aide/aide.db is the strict signal, this log is honest.
func BaselineAide(conf *config.Config, pkgList string) {
	if !installed(pkgList, "aide") {
		fmt.Println("[INFO] aide not installed, skipping baseline database")
		return
	}
	chroot := conf.Internal.ChrootDir
	if _, err := shell.ExecuteCmd(chroot, "aide --init && test -f /var/lib/aide/aide.db.new && mv /var/lib/aide/aide.db.new /var/lib/aide/aide.db"); err != nil {
		fmt.Printf("[WARNING] AIDE baseline generation failed: %v\n", err)
	}
	// Outcome via ls OUTPUT, not exit code: best-effort runs (harness) swallow
	// command failures — even a `test -f` reports error=nil there, and an
	// exit-based check would print a false success (run7, line 5074).
	out, _ := shell.ExecuteCmd(chroot, "ls /var/lib/aide/aide.db 2>/dev/null")
	if !strings.Contains(out, "aide.db") {
		fmt.Printf("[WARNING] AIDE baseline database MISSING (integrity monitoring inactive until the daily check writes one)\n")
		return
	}
	fmt.Println("[INFO] AIDE baseline database generated.")
}

// installed reports whether pkg appears in the `apk info` listing (the
// SetupLogging gate pattern — line-delimited names).
func installed(pkgList, pkg string) bool {
	return strings.Contains(pkgList, "\n"+pkg+"\n") ||
		strings.HasPrefix(pkgList, pkg+"\n") ||
		strings.HasSuffix(pkgList, "\n"+pkg)
}

// writeAuditRules writes /etc/audit/audit.rules — a LEAN baseline (not the
// full CIS sets from /usr/share/audit-rules: a desktop doesn't need the
// syscall volume). Watches are existence-filtered (auditctl -R errors would
// abort the rule load on missing paths). -e 2 makes the ruleset immutable
// until reboot: auditd restarts keep the kernel rules, and the auditd dinit
// wrapper's best-effort reload never kills the daemon. auditd.conf (package
// defaults) owns the log rotation — max_log_file_action=ROTATE,
// /var/log/audit/.
func writeAuditRules(chroot string) error {
	rules := `# FeralOS audit rules — lean baseline (docs/architecture/security_stack.md).
# Written by the installer; loaded by auditctl -R (dinit wrapper) and the
# OpenRC initd (RULEFILE_STARTUP). -e 2 = immutable until reboot: edit ONLY
# with a planned reboot in mind (auditctl reloads fail with EPERM otherwise).
-D
-b 8192
-f 1
--backlog_wait_time 60000

# Identity databases
-w /etc/passwd -p wa -k identity
-w /etc/group -p wa -k identity
-w /etc/shadow -p wa -k identity
-w /etc/gshadow -p wa -k identity
-w /etc/dinit.d/ -p wa -k system_init

# Kernel module load/unload (finit_module is b64-only on some kernels)
-a always,exit -F arch=b64 -S init_module,finit_module,delete_module -k modules
-a always,exit -F arch=b32 -S init_module,delete_module -k modules

# Time change
-a always,exit -F arch=b64 -S adjtimex,settimeofday,clock_settime -k time-change
-a always,exit -F arch=b32 -S adjtimex,settimeofday,stime,clock_settime -k time-change

# Immutable ruleset (must stay LAST)
-e 2
`

	// Existence-filtered extras (sudo-rs/sudo or doas layouts differ; sshd
	// only when the openssh stack is installed). ONE always-successful
	// command (per-path `test -e` probes would each log [ERROR][IGNORED] on
	// missing paths — pure log noise in best-effort runs). Trailing slash =
	// dir watch, no slash = file watch (auditctl semantics).
	probe := "for p in /etc/sudoers /etc/sudoers.d /etc/doas.d /etc/ssh; do [ -e \"$p\" ] && echo \"$p\"; done"
	extras := ""
	if out, err := shell.ExecuteCmd(chroot, probe); err == nil {
		for _, p := range strings.Split(strings.TrimSpace(out), "\n") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			watch := p
			key := "sudoers"
			switch p {
			case "/etc/sudoers":
				watch = "/etc/sudoers"
			case "/etc/ssh":
				watch, key = "/etc/ssh/", "sshd_config"
			case "/etc/sudoers.d", "/etc/doas.d":
				watch = p + "/"
			}
			extras += fmt.Sprintf("-w %s -p wa -k %s\n", watch, key)
		}
	}
	if extras != "" {
		rules = strings.Replace(rules,
			"-w /etc/dinit.d/ -p wa -k system_init\n",
			"-w /etc/dinit.d/ -p wa -k system_init\n\n# Sudo/ssh configuration (existence-filtered at install)\n"+extras,
			1)
	}

	if err := shell.WriteFile(chroot, "/etc/audit/audit.rules", []byte(rules), 0640); err != nil {
		return fmt.Errorf("failed to write audit rules: %w", err)
	}
	return nil
}

// writeFail2banJail drops the FeralOS jail config into /etc/fail2ban/jail.d/
// (drop-in dir — the package jail.d files stay intact). nftables-native
// banaction: the base install has NO iptables (firewalld is nftables-only and
// fail2ban's f2b nftables table is independent of firewalld's tables — the
// coexistence is validated by the harness). backend=polling: the log file is
// vector's sink (/var/log/auth.log); sshd logs facility auth → auth.log.
// On the very first boot fail2ban may start before vector created auth.log —
// smooth-recovery + restart=on-failure self-heal that race.
func writeFail2banJail(chroot string) error {
	jail := `# FeralOS fail2ban jail (written by the installer — security stack).
[DEFAULT]
banaction = nftables-multiport
banaction_allports = nftables-allports
usedns = no
bantime = 1h
findtime = 10m
maxretry = 5
ignoreip = 127.0.0.1/8 ::1

[sshd]
enabled = true
port = ssh
logpath = /var/log/auth.log
backend = polling
maxretry = 5
`
	if err := shell.WriteFile(chroot, "/etc/fail2ban/jail.d/50-feralos.conf", []byte(jail), 0644); err != nil {
		return fmt.Errorf("failed to write fail2ban jail: %w", err)
	}
	return nil
}

// writeAideConfig writes /etc/aide.conf (the package ships none — upstream
// installs only a sample under doc/), the state dirs, and the daily cron
// check. FeroLS hashes sha512 (memory-safe algo choice is moot here — the
// attr set follows the upstream default + sha512 per the docs).
func writeAideConfig(chroot string) error {
	conf := `# FeralOS AIDE configuration (written by the installer — security stack).
database_in=file:/var/lib/aide/aide.db
database_out=file:/var/lib/aide/aide.db.new
database_new=file:/var/lib/aide/aide.db.new
report_url=file:/var/log/aide/aide.log
gzip_dbout=no

# Attribute set: permissions, inode, links, user, group, size, block count,
# mtime, sha512. Not watched (by design): ctime/atime churn, acl/xattrs.
FeroLS = p+i+n+u+g+s+b+m+sha512

# Watched roots: system config + binaries + boot artifacts.
/etc FeroLS
/usr/bin FeroLS
/usr/sbin FeroLS
/boot FeroLS
/lib FeroLS

# Never watch (high-churn, non-security surface):
!/var
!/tmp
!/home
!/root
!/nix
`
	if err := shell.WriteFile(chroot, "/etc/aide.conf", []byte(conf), 0644); err != nil {
		return fmt.Errorf("failed to write aide config: %w", err)
	}
	if _, err := shell.ExecuteCmd(chroot, "mkdir -p /var/lib/aide /var/log/aide"); err != nil {
		return fmt.Errorf("failed to create aide dirs: %w", err)
	}

	cron := `#!/bin/sh
# FeralOS AIDE daily integrity check (cronie daily — security stack).
# Report goes to /var/log/aide/aide.log (vector aide_logs source); a
# non-zero exit surfaces in the cronie logs — diff BEFORE touching anything:
#   aide --check | less    /    aide --update && mv /var/lib/aide/aide.db.new /var/lib/aide/aide.db
exec /usr/bin/aide --check
`
	if err := shell.WriteFile(chroot, "/etc/periodic/daily/aide-check", []byte(cron), 0755); err != nil {
		return fmt.Errorf("failed to write aide daily check: %w", err)
	}
	return nil
}
