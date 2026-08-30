# Webmin — Web Administration Panel

> Web-based system administration for FeralOS. Installed as a standalone binary after first boot.

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

Webmin runs in a Podman container with minimal capabilities.

## Installation

Webmin is installed automatically when `enableWebmin: true` in `variables.yaml`.

### What happens during install

1. Installer compiles `/opt/feralos_webmin` binary
2. After first boot, run `/opt/feralos_webmin` to:
   - Configure Podman (storage driver btrfs, cgroups, subuid/subgid)
   - Create `/opt/webmin/` directory structure
   - Generate `docker-compose.yml`
   - Generate module configs
   - Start Webmin container via `podman compose up -d`
   - Create oneshot service `/etc/init.d/webmin-ensure`
   - Create `.desktop` file for Firefox PWA

### Manual Installation

```bash
# Run the installer binary
/opt/feralos_webmin

# Or configure manually
mkdir -p /opt/webmin/{config,commands}

# Start with podman compose
cd /opt/webmin && podman compose up -d
```

## Directory Structure

```
/opt/webmin/
├── docker-compose.yml      # Container definition
├── config/                 # Persistent module configs
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

## Access

- **URL:** `https://localhost:10000`
- **User:** `root`
- **Password:** root password from installation

### Firefox PWA

`.desktop` file at `~/.local/share/applications/webmin.desktop`.

## Modules

### Built-in (bundled in Docker image)

| Module | Description |
|--------|-------------|
| FirewallD | Firewall management |
| Software Packages | apk management |
| Disk/Filesystems | btrfs/mount management |
| Process Monitoring | Process view/kill |
| Cron Jobs | Scheduled tasks |
| Log Viewer | System logs |
| Custom Commands | btrbk/nix scripts |

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

# Update
cd /opt/webmin && podman compose pull && podman compose up -d

# Full cleanup (keeps config)
cd /opt/webmin && podman compose down
```

## Security

- **Container isolation** — Podman, not host
- **Minimal capabilities** — Only necessary Linux capabilities
- **Read-only root** — Immutable container filesystem
- **Persistent config** — `/opt/webmin/config/` survives restarts
- **HTTPS only** — SSL by default
- **Storage driver** — btrfs (native support)

## Troubleshooting

```bash
# Check binary exists
ls -la /opt/feralos_webmin

# Check container status
podman ps | grep webmin

# View logs
podman logs webmin

# Restart
cd /opt/webmin && podman compose restart
```
