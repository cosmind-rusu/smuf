# Configuration

smuf is configured exclusively through **environment variables** (or a `.env` file).

## Server (`smuf-server`)

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SMUF_DOMAIN` | `localhost` | ✅ | Your public domain name |
| `SMUF_AUTH_TOKEN` | — | ⚠️ Recommended | Secret token for client auth |
| `SMUF_CONTROL_PORT` | `7000` | ❌ | Port where clients connect |
| `SMUF_HTTP_PORT` | `8080` | ❌ | Public HTTP port |
| `SMUF_HTTPS` | `false` | ❌ | Enable automatic HTTPS |
| `SMUF_HTTPS_PORT` | `443` | ❌ | HTTPS port (when HTTPS enabled) |
| `SMUF_ACME_EMAIL` | — | ❌ | Email for Let's Encrypt notices |
| `SMUF_MAX_CONNS_PER_IP` | `5` | ❌ | Max tunnels per client IP |
| `SMUF_HANDSHAKE_TIMEOUT` | `10s` | ❌ | Connection handshake timeout |
| `SMUF_TCP_PORT_RANGE` | — | ❌ | TCP tunnel port range (e.g., `20000-30000`) |

### Server `.env` example

```env
SMUF_DOMAIN=tunnel.yourdomain.com
SMUF_AUTH_TOKEN=$(openssl rand -hex 32)
SMUF_HTTPS=true
SMUF_HTTPS_PORT=443
SMUF_ACME_EMAIL=admin@yourdomain.com
SMUF_MAX_CONNS_PER_IP=10
SMUF_TCP_PORT_RANGE=20000-30000
```

## Client (`smuf`)

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `SMUF_SERVER` | `localhost:7000` | ✅ | Server address |
| `SMUF_AUTH_TOKEN` | — | ⚠️ If server requires | Auth token |
| `SMUF_SUBDOMAIN` | — | ❌ | Fixed subdomain |

### Client `.env` example

```env
SMUF_SERVER=tunnel.yourdomain.com:7000
SMUF_AUTH_TOKEN=a-long-secret-token
```

## Generating a secure token

```bash
openssl rand -hex 32
# → d1e8a3f7c9b24e5a8f1c6d3b9e7a2f4c8d0e5b6a1f3c7d9e2b4a6c8d0e1f3a5
```

Use the same token on both server and client.

## Loading `.env` files

smuf automatically loads a `.env` file if it exists **next to the binary** (same directory). No need for `source .env` or `export`.

Example directory structure:
```
/usr/local/bin/
├── smuf-server
├── smuf-server.env    ← loaded automatically (server)
└── smuf
~/smuf/
├── smuf
└── .env              ← loaded automatically (client)
```
