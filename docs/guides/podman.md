# Podman — Rootless Container Runtime

> Docker-compatible container management without daemon, running rootless by default.

## Overview

Podman is a daemonless container engine for developing, managing, and running OCI containers. It runs containers rootless by default, improving security.

Includes:
- `podman` — container runtime
- `podman-compose` — Docker Compose compatibility
- `podman-docker` — Docker CLI compatibility
- `docker` → `podman` symlink (dummy package)

## Installation

Podman is installed automatically when `enablePodman: true` in `variables.yaml`.

### Manual Installation

```bash
# Install podman and dependencies (exact set from chrootPhase/features/podman.go:PodmanPackages)
apk add podman podman-openrc iptables podman-compose podman-docker fuse-overlayfs
# docker is a dummy package (internal_rules.yaml: podman replace docker) → docker → podman symlink

# Enable services (both required — cgroups for podman)
rc-update add podman default
rc-update add cgroups default

# Storage driver: btrfs (root is btrfs, Alpine ships full storage.conf — sed only driver line)
mkdir -p /etc/containers
sed -i 's/^#*driver[[:space:]]*=.*/driver = "btrfs"/' /etc/containers/storage.conf

# Add user to podman group
adduser <username> podman

# Create docker → podman symlink (handled by dummy package)
ln -sf /usr/bin/podman /usr/local/bin/docker
```

### Rootless Configuration

```bash
# Configure subuid/subgid for rootless containers (SetupPodman writes exactly this)
echo "<username>:100000:65536" > /etc/subuid
echo "<username>:100000:65536" > /etc/subgid
```

> **fuse-overlayfs kept**: required as fallback for rootless overlay even with btrfs driver (kernel overlay still needs fuse).

## Usage

### Basic Commands

```bash
# Pull an image
podman pull docker.io/library/alpine:latest

# Run a container
podman run -d --name myapp -p 8080:80 docker.io/library/nginx:alpine

# List running containers
podman ps

# List all containers (including stopped)
podman ps -a

# Stop a container
podman stop myapp

# Remove a container
podman pod rm myapp

# View logs
podman logs myapp

# Execute command in running container
podman exec -it myapp /bin/sh
```

### Docker Compose Compatibility

```bash
# Use docker-compose with podman
alias docker=podman
docker-compose up -d
```

### Image Management

```bash
# List images
podman images

# Remove image
podman rmi nginx:alpine

# Build from Dockerfile
podman build -t myapp .

# Save image to file
podman save myapp -o myapp.tar

# Load image from file
podman load -i myapp.tar
```

### Volume Management

```bash
# Create volume
podman volume create mydata

# List volumes
podman volume ls

# Use volume in container
podman run -v mydata:/data docker.io/library/alpine:latest
```

## Rootless vs Rootful

| Feature | Rootless | Rootful |
|---------|----------|---------|
| Security | ✅ Better (no root) | ⚠️ Root access |
| Performance | ⚠️ Slightly slower | ✅ Full speed |
| Network | User namespace | Full network access |
| Storage | `~/.local/share/containers/` | `/var/lib/containers/` |
| Default | ✅ Yes | No |

## Integration with Webmin

Webmin's Custom Commands module can manage podman containers. Create scripts in `/etc/webmin/custom/commands/`:

```bash
# /etc/webmin/custom/commands/podman-ps
#!/bin/sh
podman ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

# /etc/webmin/custom/commands/podman-images
#!/bin/sh
podman images --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
```

## Troubleshooting

```bash
# Check podman info
podman info

# Reset podman (removes all containers/images)
podman system reset

# Check rootless config
podman unshare cat /proc/self/uid_map
```
