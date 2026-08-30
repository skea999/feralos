# Webmin — Web Administration Panel

> Web-based system administration for FeralOS. Access server management from any browser.

## Overview

Webmin provides a web interface for managing:
- Firewall (FirewallD)
- Packages (apk)
- Disk/Filesystems (btrfs)
- SMART disk health
- Processes
- Cron jobs
- Logs
- Custom commands (btrbk, nix)

## Installation

Webmin is installed automatically when `enableWebmin: true` in `variables.yaml`.

### Manual Installation

```bash
# Install dependencies
apk add perl perl-io-socket-ssl net-tools procps cronie

# Download and run setup
curl -sSL https://raw.githubusercontent.com/webmin/webmin/master/setup.sh -o /tmp/webmin-setup.sh
sh /tmp/webmin-setup.sh --unattended

# Enable service
rc-update add webmin default
rc-service webmin start
```

### Configuration

Edit `/etc/webmin/miniserv.conf`:

```ini
# Listen on localhost only (default)
bind=127.0.0.1

# Listen on all interfaces (remote access)
bind=0.0.0.0
```

## Access

- **URL:** `https://localhost:10000`
- **User:** `root`
- **Password:** root password from installation

### Firefox PWA

When installed via the installer, a `.desktop` file is created at `~/.local/share/applications/webmin.desktop`. This allows launching Webmin as a standalone app from the niri launcher.

## Modules

| Module | Description | Configuration |
|--------|-------------|---------------|
| FirewallD | Manage firewalld rules, zones, services | `/etc/webmin/firewalld/config` |
| Software Packages | Install/remove/update apk packages | `/etc/webmin/software/config` |
| Disk/Filesystems | Mount/unmount, fstab management | `/etc/webmin/mount/config` |
| Custom Commands | Run btrbk, nix, custom scripts | `/etc/webmin/custom/commands/` |
| SMART Status | Monitor disk health | `/etc/webmin/smart-status/config` |
| Process Monitoring | View/kill processes | `/etc/webmin/proc/config` |
| Cron Jobs | Manage scheduled tasks | `/etc/webmin/cron/config` |
| Log Viewer | View system logs | `/etc/webmin/logviewer/config` |

## Custom Commands for btrbk

Create scripts in `/etc/webmin/custom/commands/`:

```bash
# /etc/webmin/custom/commands/btrbk-snapshot
#!/bin/sh
btrbk run --config /etc/btrbk/btrbk.conf

# /etc/webmin/custom/commands/btrbk-list
#!/bin/sh
btrbk list snapshots --config /etc/btrbk/btrbk.conf

# /etc/webmin/custom/commands/nix-update
#!/bin/sh
nix profile upgrade --all
```

Make executable:
```bash
chmod +x /etc/webmin/custom/commands/btrbk-*
chmod +x /etc/webmin/custom/commands/nix-*
```

## Security

- **HTTPS only** — Webmin uses SSL by default
- **Root access** — Webmin runs as root, be careful with permissions
- **Firewall** — If `webminListenAll: true`, ensure firewall allows port 10000
- **Fail2Ban** — Install fail2ban to protect against brute force attacks

## Troubleshooting

```bash
# Check Webmin status
rc-service webmin status

# View Webmin logs
cat /var/log/webmin/miniserv.log

# Restart Webmin
rc-service webmin restart
```
