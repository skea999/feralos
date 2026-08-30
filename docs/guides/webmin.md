# Webmin — Web Administration Panel (Podman Container)

> Web-based system administration for FeralOS, running in a Podman container with minimal permissions.

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

Webmin runs in a Podman container with minimal capabilities for security.

## Installation

Webmin is installed automatically when `enableWebmin: true` in `variables.yaml`. This also enables `enablePodman: true`.

### Manual Installation

```bash
# Ensure podman is installed and running
rc-service podman start

# Pull Webmin image
podman pull docker.io/webmin/webmin:latest

# Stop existing container if any
podman rm -f webmin 2>/dev/null

# Run Webmin with minimal permissions
podman run -d \
  --name webmin \
  --hostname feralos-webmin \
  --read-only \
  --cap-drop ALL \
  --cap-add SYS_CHROOT \
  --cap-add NET_BIND_SERVICE \
  --cap-add AUDIT_WRITE \
  --cap-add SETUID \
  --cap-add SETGID \
  --cap-add FOWNER \
  --cap-add DAC_OVERRIDE \
  --cap-add SYS_ADMIN \
  -p 127.0.0.1:10000:10000 \
  -v /etc:/host/etc:ro \
  -v /var:/host/var:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /run:/host/run:ro \
  -v /boot:/host/boot:ro \
  -v /home:/host/home:rw \
  -v /root:/host/root:rw \
  -e WEBMIN_CONFIG=/host/etc/webmin \
  -e MINISERV_CONFIG=/host/etc/webmin/miniserv.conf \
  --restart unless-stopped \
  docker.io/webmin/webmin:latest
```

## Security Model

| Permission | Rationale |
|------------|-----------|
| `--read-only` | Immutable root filesystem |
| `--cap-drop ALL` | Remove all capabilities |
| `--cap-add SYS_CHROOT` | Webmin needs chroot for system management |
| `--cap-add NET_BIND_SERVICE` | Bind to port 10000 |
| `--cap-add SETUID/SETGID` | Switch users for admin tasks |
| `-v /etc:/host/etc:ro` | Read-only access to system config |
| `-v /home:/host/home:rw` | Read-write for user home management |

## Access

- **URL:** `https://localhost:10000`
- **User:** `root`
- **Password:** root password from installation

### Firefox PWA

A `.desktop` file is created at `~/.local/share/applications/webmin.desktop`. Launch Webmin as a standalone app from the niri launcher.

## Modules

| Module | Description | Configuration |
|--------|-------------|---------------|
| FirewallD | Manage firewalld rules, zones, services | Inside container: `/etc/webmin/firewalld/config` |
| Software Packages | Install/remove/update apk packages | Inside container: `/etc/webmin/software/config` |
| Disk/Filesystems | Mount/unmount, fstab management | Inside container: `/etc/webmin/mount/config` |
| Custom Commands | Run btrbk, nix, custom scripts | Inside container: `/etc/webmin/custom/commands/` |
| SMART Status | Monitor disk health | Inside container: `/etc/webmin/smart-status/config` |
| Process Monitoring | View/kill processes | Inside container: `/etc/webmin/proc/config` |
| Cron Jobs | Manage scheduled tasks | Inside container: `/etc/webmin/cron/config` |
| Log Viewer | View system logs | Inside container: `/etc/webmin/logviewer/config` |

## Container Management

```bash
# View Webmin container status
podman ps | grep webmin

# View Webmin logs
podman logs webmin

# Restart Webmin
podman restart webmin

# Stop Webmin
podman stop webmin

# Remove Webmin
podman rm -f webmin

# Update Webmin
podman pull docker.io/webmin/webmin:latest
podman rm -f webmin
# Re-run with same parameters
```

## Custom Commands for btrbk

Create scripts on the host and mount them into the container:

```bash
# Create commands directory on host
mkdir -p /etc/webmin/custom/commands

# /etc/webmin/custom/commands/btrbk-snapshot
#!/bin/sh
btrbk run --config /etc/btrbk/btrbk.conf

# /etc/webmin/custom/commands/btrbk-list
#!/bin/sh
btrbk list snapshots --config /etc/btrbk/btrbk.conf

chmod +x /etc/webmin/custom/commands/btrbk-*
```

Add volume mount to container run command:
```bash
-v /etc/webmin/custom/commands:/etc/webmin/custom/commands:ro
```

## Security

- **Container isolation** — Webmin runs in a Podman container, not directly on host
- **Minimal capabilities** — Only necessary Linux capabilities granted
- **Read-only root** — Container filesystem is immutable
- **HTTPS only** — Webmin uses SSL by default
- **Firewall** — If `webminListenAll: true`, ensure firewall allows port 10000

## Troubleshooting

```bash
# Check Webmin container status
podman ps -a | grep webmin

# View Webmin logs
podman logs webmin

# Restart Webmin
podman restart webmin

# Check container resource usage
podman stats webmin
```
