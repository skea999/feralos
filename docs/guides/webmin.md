# Webmin — Web Administration Panel (Podman Compose)

> Web-based system administration for FeralOS, running in a Podman container managed via compose, with persistent configuration in `/opt/webmin/`.

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

Webmin runs in a Podman container with minimal capabilities, managed via `docker-compose.yml` in `/opt/webmin/`.

## Installation

Webmin is installed automatically when `enableWebmin: true` in `variables.yaml`. This also enables `enablePodman: true`.

### What gets created

```
/opt/webmin/
├── docker-compose.yml      # Container definition
├── config/                 # Persistent module configs (mounted into container)
│   ├── firewalld/config
│   ├── software/config
│   ├── custom/config
│   ├── smart-status/config
│   ├── proc/config
│   ├── cron/config
│   └── logviewer/config
├── modules/                # Installed Webmin modules
└── commands/               # Custom scripts (btrbk, nix)
```

### Manual Installation

```bash
# Ensure podman is installed
rc-service podman start

# Create directory structure
mkdir -p /opt/webmin/{config,modules,commands}

# Start with podman compose
cd /opt/webmin && podman compose up -d

# Or with podman-compose
cd /opt/webmin && podman-compose up -d
```

## Persistent Configuration

All configuration lives in `/opt/webmin/config/` and is mounted into the container at `/etc/webmin/`.

| Config file | Module | Purpose |
|-------------|--------|---------|
| `/opt/webmin/config/firewalld/config` | FirewallD | firewall-cmd path, firewall_type |
| `/opt/webmin/config/software/config` | Software Packages | apk commands |
| `/opt/webmin/config/custom/config` | Custom Commands | scripts directory |
| `/opt/webmin/config/smart-status/config` | SMART Status | smartctl path |
| `/opt/webmin/config/proc/config` | Process Monitoring | ps/kill/renice paths |
| `/opt/webmin/config/cron/config` | Cron Jobs | crontab path |
| `/opt/webmin/config/logviewer/config` | Log Viewer | log files |

Config persists across container restarts and updates.

## Security Model

| Permission | Rationale |
|------------|-----------|
| `--read-only` | Immutable root filesystem |
| `--cap-drop ALL` | Remove all capabilities |
| `--cap-add SYS_CHROOT` | System management |
| `--cap-add NET_BIND_SERVICE` | Port 10000 |
| `--cap-add SETUID/SETGID` | User switching |
| `-v /opt/webmin/config:/etc/webmin` | Persistent config |
| `-v /opt/webmin/commands:/etc/webmin/custom/commands` | Custom scripts |
| `-v /etc:/host/etc:ro` | Read-only system config |

## Access

- **URL:** `https://localhost:10000`
- **User:** `root`
- **Password:** root password from installation

### Firefox PWA

A `.desktop` file is created at `~/.local/share/applications/webmin.desktop`.

## Modules

### Built-in (bundled in Docker image)

| Module | Description | Config |
|--------|-------------|--------|
| FirewallD | Firewall management | `/opt/webmin/config/firewalld/config` |
| Software Packages | apk management | `/opt/webmin/config/software/config` |
| Disk/Filesystems | btrfs/mount management | Built-in |
| Process Monitoring | Process view/kill | `/opt/webmin/config/proc/config` |
| Cron Jobs | Scheduled tasks | `/opt/webmin/config/cron/config` |
| Log Viewer | System logs | `/opt/webmin/config/logviewer/config` |

### Installed by installer

| Module | Description | Source |
|--------|-------------|--------|
| Custom Commands | btrbk/nix scripts | `download.webmin.com` |

## Custom Commands

Place scripts in `/opt/webmin/commands/` — they're mounted into the container:

```bash
# Create btrbk commands
cat > /opt/webmin/commands/btrbk-snapshot << 'EOF'
#!/bin/sh
btrbk run --config /etc/btrbk/btrbk.conf
EOF

cat > /opt/webmin/commands/btrbk-list << 'EOF'
#!/bin/sh
btrbk list snapshots --config /etc/btrbk/btrbk.conf
EOF

chmod +x /opt/webmin/commands/btrbk-*
```

## Container Management

```bash
# Status
cd /opt/webmin && podman compose ps

# Logs
cd /opt/webmin && podman compose logs webmin

# Restart
cd /opt/webmin && podman compose restart

# Stop
cd /opt/webmin && podman compose stop

# Start
cd /opt/webmin && podman compose start

# Update image
cd /opt/webmin && podman compose pull && podman compose up -d

# Full cleanup (keeps config)
cd /opt/webmin && podman compose down
```

## Security

- **Container isolation** — Webmin runs in Podman, not directly on host
- **Minimal capabilities** — Only necessary Linux capabilities
- **Read-only root** — Container filesystem immutable
- **Persistent config** — `/opt/webmin/config/` survives restarts
- **HTTPS only** — SSL by default
- **Firewall** — If `webminListenAll: true`, allow port 10000

## Troubleshooting

```bash
# Container status
cd /opt/webmin && podman compose ps

# Logs
cd /opt/webmin && podman compose logs webmin

# Restart
cd /opt/webmin && podman compose restart

# Inspect config
cat /opt/webmin/config/firewalld/config
```
