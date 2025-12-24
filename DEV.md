# Development Guide

This guide explains how to set up a Docker development environment with **live code reloading**.

## Prerequisites

- Docker & Docker Compose installed
- Go 1.24+ (for local development without Docker)

## Development with Docker (Hot Reload)

The development setup uses [Air](https://github.com/cosmtrek/air) for automatic hot-reloading. When you change Go source files outside the container, the changes are automatically detected and the application rebuilds.

### Files

| File | Purpose |
|------|---------|
| `Dockerfile.dev` | Development image with Go + Air hot-reload |
| `docker-compose.dev.yml` | Dev compose with volume mounts |
| `.air.toml` | Air hot-reload configuration |

### How to Run

**Build and start the development container:**

```bash
docker compose -f docker-compose.dev.yml up --build
```

**Run in detached mode (background):**

```bash
docker compose -f docker-compose.dev.yml up --build -d
```

**View logs (if running detached):**

```bash
docker compose -f docker-compose.dev.yml logs -f
```

**Stop the container:**

```bash
docker compose -f docker-compose.dev.yml down
```

### How It Works

1. Your entire project is mounted to `/app` inside the container
2. **Air** watches for changes in `.go` and `.yaml` files
3. When you edit code **outside** the container (in your IDE), Air detects changes and **automatically rebuilds** the binary
4. The app restarts with your new changes (~1-2 seconds)

You'll see output like this when changes are detected:

```
watching...
building...
running...
```

### Exposed Ports

| Port | Description |
|------|-------------|
| 8317 | Main API |
| 8085 | Secondary API |
| 1455 | Additional service |
| 54545 | Additional service |
| 51121 | Additional service |
| 11451 | Additional service |

## Production Build

For production, use the standard Docker Compose file:

```bash
docker compose up --build -d
```

This uses the multi-stage `Dockerfile` which creates an optimized, minimal Alpine image.
