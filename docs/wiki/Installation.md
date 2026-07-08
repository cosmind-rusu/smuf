# Installation

There are three ways to install smuf. Choose the one that fits your setup.

## Option A: Download a binary (recommended)

Go to the [Releases page](https://github.com/cosmind-rusu/smuf/releases) and download the archive for your OS.

| File | Platform |
|------|----------|
| `smuf_0.3.0_Linux_x86_64.tar.gz` | Linux (amd64) |
| `smuf_0.3.0_Linux_arm64.tar.gz` | Linux (ARM64) |
| `smuf_0.3.0_Darwin_x86_64.tar.gz` | macOS (Intel) |
| `smuf_0.3.0_Darwin_arm64.tar.gz` | macOS (Apple Silicon) |
| `smuf_0.3.0_Windows_x86_64.zip` | Windows (64-bit) |

```bash
# Extract
tar -xzf smuf_0.3.0_Linux_x86_64.tar.gz

# You'll get two binaries:
#   smuf-server  → deploy on your VPS
#   smuf         → run on your local machine
```

> **Tip:** Put the binaries in your `PATH` (e.g., `/usr/local/bin/`) for easy access.

## Option B: Docker

A `docker-compose.yml` is included in the repository. This runs only the server component.

```bash
git clone https://github.com/cosmind-rusu/smuf.git
cd smuf
docker compose up -d
```

This starts `smuf-server` with default settings. Configure via environment variables (see [Configuration](Configuration)).

## Option C: Build from source

Requires [Go](https://go.dev/dl/) 1.26.3+.

```bash
git clone https://github.com/cosmind-rusu/smuf.git
cd smuf

# Build server
go build -o smuf-server ./cmd/smuf-server

# Build client
go build -o smuf ./cmd/smuf

# Verify
./smuf -v       # → smuf v0.3.0
./smuf-server -v  # → smuf-server v0.3.0
```

### Cross-compilation

```bash
# Build for a different platform
GOOS=linux GOARCH=arm64 go build -o smuf-server-arm64 ./cmd/smuf-server
```
