# Podman — Rootless Container Runtime

> Docker-compatible container management without daemon, running rootless by default.

## Overview

Podman is a daemonless container engine for developing, managing, and running OCI containers. It runs containers rootless by default, improving security.

## Installation

Podman is installed automatically when `enablePodman: true` in `variables.yaml`.

### Manual Installation

```bash
# Install podman and dependencies
apk add podman podman-openrc catatonit slirp4netns fuse-overlayfs uidmap shadow-uidmap

# Enable service
rc-update add podman default

# Add user to podman group
adduser <username> podman
```

### Rootless Configuration

```bash
# Configure subuid/subgid for rootless containers
echo "<username>:100000:65536" >> /etc/subuid
echo "<username>:100000:65536" >> /etc/subgid
```

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
