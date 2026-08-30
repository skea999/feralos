# Webmin — Web Administration Panel (Podman Compose)

> Web-based system administration for FeralOS. Complete guide: install, configure, enable all plugins, custom commands.

## Quick Start (copy & paste)

```bash
# 1. Create directory structure
mkdir -p /opt/webmin/{config,commands}

# 2. Create docker-compose.yml
cat > /opt/webmin/docker-compose.yml << 'EOF'
services:
  webmin:
    image: docker.io/webmin/webmin:latest
    container_name: webmin
    hostname: feralos-webmin
    restart: unless-stopped
    read_only: true
    cap_drop:
      - ALL
    cap_add:
      - SYS_CHROOT
      - NET_BIND_SERVICE
      - AUDIT_WRITE
      - SETUID
      - SETGID
      - FOWNER
      - DAC_OVERRIDE
      - SYS_ADMIN
    ports:
      - "127.0.0.1:10000:10000"
    volumes:
      - /opt/webmin/config:/etc/webmin
      - /opt/webmin/commands:/etc/webmin/custom/commands
      - /etc:/host/etc:ro
      - /var:/host/var:ro
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /run:/host/run:ro
      - /boot:/host/boot:ro
      - /home:/host/home:rw
      - /root:/host/root:rw
    environment:
      - WEBMIN_CONFIG=/etc/webmin
      - MINISERV_CONFIG=/etc/webmin/miniserv.conf
EOF

# 3. Create module configs
mkdir -p /opt/webmin/config/{firewalld,software,custom,smart-status,proc,cron,logviewer}

cat > /opt/webmin/config/firewalld/config << 'EOF'
firewall_cmd=/usr/sbin/firewall-cmd
firewall_type=firewalld
EOF

cat > /opt/webmin/config/software/config << 'EOF'
package_type=apk
update_command=apk update
install_command=apk add
remove_command=apk del
search_command=apk search
EOF

cat > /opt/webmin/config/custom/config << 'EOF'
commands_dir=/etc/webmin/custom/commands
EOF

cat > /opt/webmin/config/smart-status/config << 'EOF'
smartctl_command=/usr/sbin/smartctl
smartd_conf=/etc/smartd.conf
EOF

cat > /opt/webmin/config/proc/config << 'EOF'
ps_command=/bin/ps
kill_command=/bin/kill
renice_command=/usr/bin/renice
EOF

cat > /opt/webmin/config/cron/config << 'EOF'
cron_dir=/etc/crontabs
cron_command=/usr/bin/crontab
EOF

cat > /opt/webmin/config/logviewer/config << 'EOF'
log_files=/var/log/messages
log_files=/var/log/syslog
log_files=/var/log/auth.log
log_command=/usr/bin/tail
EOF

# 4. Install Custom Commands module
curl -sSL https://download.webmin.com/download/modules/custom.wbm.gz -o /tmp/custom.wbm.gz
cd /tmp && gunzip custom.wbm.gz

# 5. Start Webmin
cd /opt/webmin && podman compose up -d

# 6. Install module inside container
podman cp /tmp/custom.wbm webmin:/tmp/custom.wbm
podman exec webmin /usr/share/webmin/install-module.pl /tmp/custom.wbm
rm -f /tmp/custom.wbm*

# 7. Set root password
podman exec webmin sh -c 'echo "root:YOUR_PASSWORD" | chpasswd'

# 8. Access
echo "Webmin: https://localhost:10000"
echo "User: root"
echo "Password: YOUR_PASSWORD"
```

## Directory Structure

```
/opt/webmin/
├── docker-compose.yml      # Container definition
├── config/                 # Persistent module configs
│   ├── firewalld/config    # FirewallD settings
│   ├── software/config     # apk package manager
│   ├── custom/config       # Custom commands
│   ├── smart-status/config # Disk health
│   ├── proc/config         # Process monitoring
│   ├── cron/config         # Cron jobs
│   └── logviewer/config    # System logs
├── modules/                # Installed Webmin modules
└── commands/               # Custom scripts (btrbk, nix)
```

## Module Configs (copy & paste)

### FirewallD

```bash
mkdir -p /opt/webmin/config/firewalld
cat > /opt/webmin/config/firewalld/config << 'EOF'
firewall_cmd=/usr/sbin/firewall-cmd
firewall_type=firewalld
EOF
```

### Software Packages (apk)

```bash
mkdir -p /opt/webmin/config/software
cat > /opt/webmin/config/software/config << 'EOF'
package_type=apk
update_command=apk update
install_command=apk add
remove_command=apk del
search_command=apk search
EOF
```

### Custom Commands

```bash
mkdir -p /opt/webmin/config/custom
cat > /opt/webmin/config/custom/config << 'EOF'
commands_dir=/etc/webmin/custom/commands
EOF
```

### SMART Status

```bash
mkdir -p /opt/webmin/config/smart-status
cat > /opt/webmin/config/smart-status/config << 'EOF'
smartctl_command=/usr/sbin/smartctl
smartd_conf=/etc/smartd.conf
EOF
```

### Process Monitoring

```bash
mkdir -p /opt/webmin/config/proc
cat > /opt/webmin/config/proc/config << 'EOF'
ps_command=/bin/ps
kill_command=/bin/kill
renice_command=/usr/bin/renice
EOF
```

### Cron Jobs

```bash
mkdir -p /opt/webmin/config/cron
cat > /opt/webmin/config/cron/config << 'EOF'
cron_dir=/etc/crontabs
cron_command=/usr/bin/crontab
EOF
```

### Log Viewer

```bash
mkdir -p /opt/webmin/config/logviewer
cat > /opt/webmin/config/logviewer/config << 'EOF'
log_files=/var/log/messages
log_files=/var/log/syslog
log_files=/var/log/auth.log
log_command=/usr/bin/tail
EOF
```

## Custom Commands (copy & paste)

### btrbk commands

```bash
mkdir -p /opt/webmin/commands

cat > /opt/webmin/commands/btrbk-snapshot << 'EOF'
#!/bin/sh
btrbk run --config /etc/btrbk/btrbk.conf
EOF

cat > /opt/webmin/commands/btrbk-list << 'EOF'
#!/bin/sh
btrbk list snapshots --config /etc/btrbk/btrbk.conf
EOF

cat > /opt/webmin/commands/btrbk-delete << 'EOF'
#!/bin/sh
btrbk delete --config /etc/btrbk/btrbk.conf
EOF

cat > /opt/webmin/commands/btrbk-restore << 'EOF'
#!/bin/sh
btrbk restore --config /etc/btrbk/btrbk.conf
EOF

chmod +x /opt/webmin/commands/btrbk-*
```

### nix commands

```bash
cat > /opt/webmin/commands/nix-update << 'EOF'
#!/bin/sh
nix profile upgrade --all
EOF

cat > /opt/webmin/commands/nix-search << 'EOF'
#!/bin/sh
nix search nixpkgs "$1"
EOF

cat > /opt/webmin/commands/nix-install << 'EOF'
#!/bin/sh
nix profile add "nixpkgs#$1"
EOF

cat > /opt/webmin/commands/nix-remove << 'EOF'
#!/bin/sh
nix profile remove "$1"
EOF

chmod +x /opt/webmin/commands/nix-*
```

### Docker/Podman module

The Docker module for Webmin is automatically installed when Webmin is set up. It uses Podman via a `docker` → `podman` symlink (created by the docker dummy package).

**Features:**
- List containers (running, stopped)
- Start/stop/restart containers
- View container logs
- Container properties
- System info
- Monitor types for System Status

No additional setup needed — the module works out of the box with Podman.

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

# Update
cd /opt/webmin && podman compose pull && podman compose up -d

# Full cleanup (keeps config)
cd /opt/webmin && podman compose down
```

## Access

- **URL:** `https://localhost:10000`
- **User:** `root`
- **Password:** root password from installation

### Firefox PWA

`.desktop` file at `~/.local/share/applications/webmin.desktop`.

## Security

- **Container isolation** — Podman, not host
- **Minimal capabilities** — Only necessary Linux capabilities
- **Read-only root** — Immutable container filesystem
- **Persistent config** — `/opt/webmin/config/` survives restarts
- **HTTPS only** — SSL by default
- **Firewall** — If `webminListenAll: true`, allow port 10000

## Troubleshooting

```bash
# Status
cd /opt/webmin && podman compose ps

# Logs
cd /opt/webmin && podman compose logs webmin

# Restart
cd /opt/webmin && podman compose restart

# Inspect config
cat /opt/webmin/config/firewalld/config
```
