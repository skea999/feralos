# Webmin — Web Administration Panel (Podman Container)

> Web-based system administration for FeralOS, running in a Podman container with minimal permissions and persistent configuration.

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

Webmin runs in a Podman container with minimal capabilities. Configuration persists via a named volume.

## Installation

Webmin is installed automatically when `enableWebmin: true` in `variables.yaml`. This also enables `enablePodman: true`.

### Manual Installation

```bash
# Ensure podman is installed and running
rc-service podman start

# Pull Webmin image
podman pull docker.io/webmin/webmin:latest

# Create persistent volume
podman volume create webmin-data

# Stop existing container if any
podman rm -f webmin 2>/dev/null

# Run Webmin with minimal permissions and persistent volume
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
  -v webmin-data:/etc/webmin \
  -v /etc:/host/etc:ro \
  -v /var:/host/var:ro \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /run:/host/run:ro \
  -v /boot:/host/boot:ro \
  -v /home:/host/home:rw \
  -v /root:/host/root:rw \
  -e WEBMIN_CONFIG=/etc/webmin \
  -e MINISERV_CONFIG=/etc/webmin/miniserv.conf \
  --restart unless-stopped \
  docker.io/webmin/webmin:latest
```

## Persistent Volume

Webmin uses a named Podman volume `webmin-data` mounted at `/etc/webmin` inside the container.

| What persists | Path |
|---------------|------|
| Webmin config | `/etc/webmin/miniserv.conf` |
| Installed modules | `/etc/webmin/*/` |
| Module configs | `/etc/webmin/*/config` |
| Custom commands | `/etc/webmin/custom/commands/` |
| User settings | `/etc/webmin/webmin.acl` |

The volume persists across container restarts and updates.

## Security Model

| Permission | Rationale |
|------------|-----------|
| `--read-only` | Immutable root filesystem |
| `--cap-drop ALL` | Remove all capabilities |
| `--cap-add SYS_CHROOT` | Webmin needs chroot for system management |
| `--cap-add NET_BIND_SERVICE` | Bind to port 10000 |
| `--cap-add SETUID/SETGID` | Switch users for admin tasks |
| `-v webmin-data:/etc/webmin` | Persistent config/modules |
| `-v /etc:/host/etc:ro` | Read-only access to system config |
| `-v /home:/host/home:rw` | Read-write for user home management |

## Access

- **URL:** `https://localhost:10000`
- **User:** `root`
- **Password:** root password from installation

### Firefox PWA

A `.desktop` file is created at `~/.local/share/applications/webmin.desktop`. Launch Webmin as a standalone app from the niri launcher.

## Modules

### Built-in (bundled in Docker image)

| Module | Description | Configuration |
|--------|-------------|---------------|
| FirewallD | Manage firewalld rules, zones, services | `/etc/webmin/firewalld/config` |
| Software Packages | Install/remove/update apk packages | `/etc/webmin/software/config` |
| Disk/Filesystems | Mount/unmount, fstab management | `/etc/webmin/mount/config` |
| Process Monitoring | View/kill processes | `/etc/webmin/proc/config` |
| Cron Jobs | Manage scheduled tasks | `/etc/webmin/cron/config` |
| Log Viewer | View system logs | `/etc/webmin/logviewer/config` |

### Installed by installer

| Module | Description | Installation |
|--------|-------------|-------------|
| Custom Commands | Run btrbk, nix, custom scripts | Downloaded from `download.webmin.com` |

### Manual installation

Additional modules can be installed inside the container:

```bash
# Download module
curl -sSL https://download.webmin.com/download/modules/<module>.wbm.gz -o /tmp/<module>.wbm.gz

# Extract and copy to container
cd /tmp && gunzip <module>.wbm.gz
podman cp /tmp/<module>.wbm webmin:/tmp/<module>.wbm

# Install inside container
podman exec webmin /usr/share/webmin/install-module.pl /tmp/<module>.wbm
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

The custom commands directory is mounted into the container via `/etc/webmin/custom/commands/`.

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

# Remove Webmin (keeps volume)
podman rm -f webmin

# Remove Webmin + volume (full cleanup)
podman rm -f webmin && podman volume rm webmin-data

# Update Webmin
podman pull docker.io/webmin/webmin:latest
podman rm -f webmin
# Re-run with same parameters (volume persists)
```

## Security

- **Container isolation** — Webmin runs in a Podman container, not directly on host
- **Minimal capabilities** — Only necessary Linux capabilities granted
- **Read-only root** — Container filesystem is immutable
- **Persistent volume** — Config survives container restarts
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

# Inspect persistent volume
podman volume inspect webmin-data
```
