# Client Usage

The `smuf` client runs on your local machine and creates tunnels to your `smuf-server`.

## Basic usage

```bash
# Expose a single port
./smuf 3000

# Expose multiple ports at once
./smuf 3000 4000 5000

# TCP tunnel (SSH, databases, etc.)
./smuf --tcp 22

# Custom subdomain (fixed URL)
./smuf --sub myapp 3000
# → https://myapp.yourdomain.com
```

## First run — Setup wizard

Running without arguments or with `--setup` starts the interactive wizard:

```bash
./smuf --setup
```

The wizard will ask for:
- Server address (e.g., `yourdomain.com:7000`)
- Auth token (if required)
- Subdomain preference

Configuration is saved automatically.

## HTTP tunnels

Default mode. Your local HTTP service becomes available at a public URL.

```bash
# Expose a web app on port 3000
./smuf 3000
# → https://a3f1c9.yourdomain.com

# Expose a Next.js dev server
./smuf 3000
```

HTTP tunnels support:
- Regular HTTP requests
- WebSockets (e.g., `ws://localhost:3000` → `wss://a3f1c9.yourdomain.com`)
- Server-Sent Events (SSE)
- Long polling

## TCP tunnels

Use `--tcp` for non-HTTP protocols:

```bash
# SSH access to your local machine
./smuf --tcp 22

# PostgreSQL database
./smuf --tcp 5432

# Minecraft server
./smuf --tcp 25565

# Any TCP service
./smuf --tcp 8080
```

TCP tunnels assign a port from the server's `SMUF_TCP_PORT_RANGE`.

## Custom subdomains

Request a fixed subdomain instead of a random one:

```bash
./smuf --sub myapp 3000
# → https://myapp.yourdomain.com
```

> **Note:** Subdomains are first-come, first-served. If someone else is using it, you'll get an error.

## Environment variables

| Variable | Description |
|----------|-------------|
| `SMUF_SERVER` | Server address (e.g., `yourdomain.com:7000`) |
| `SMUF_AUTH_TOKEN` | Auth token (must match server) |
| `SMUF_SUBDOMAIN` | Fixed subdomain (equivalent to `--sub`) |

These can be set in a `.env` file next to the binary:

```env
SMUF_SERVER=yourdomain.com:7000
SMUF_AUTH_TOKEN=a-long-secret-token
```
