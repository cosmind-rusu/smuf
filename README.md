<div align="center">
  <img src="banner.svg" width="100%" alt="smuf — self-hosted HTTP tunnel"/>
</div>

<br/>

<div align="center">
  <sub>SELF-HOSTED · OPEN SOURCE · WRITTEN IN GO</sub>
  <br/><br/>

  [![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
  [![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-9B59B6?style=flat-square)](LICENSE)
  [![Self-hosted](https://img.shields.io/badge/self--hosted-✓-E53E3E?style=flat-square)]()
  [![yamux](https://img.shields.io/badge/yamux-multiplexing-656a76?style=flat-square)](https://github.com/hashicorp/yamux)
</div>

---

smuf exposes a local port on a public URL — like ngrok, but **yours**: no quotas, no limits, on your own server.

```
localhost:3000  ◄─────►  https://a3f1c9.yourdomain.com
```

<br/>

<p align="center">
  <a href="assets/demo.svg">
    <img src="assets/demo.svg" alt="smuf demo" width="95%"/>
  </a>
</p>

---

## Installation

### Option A: Download a binary (recommended)

Go to [Releases](https://github.com/cdrusu/smuf/releases) and download the binary for your OS.

| Binary | Where it goes |
|--------|--------------|
| `smuf-server` | Your VPS / server |
| `smuf` | Your local machine |

### Option B: Docker

```bash
docker compose up -d
```

### Option C: Build from source

You'll need [Go](https://go.dev/dl/) 1.21+.

```bash
git clone https://github.com/cdrusu/smuf.git && cd smuf
go build -o smuf-server ./cmd/smuf-server
go build -o smuf        ./cmd/smuf
```

---

## Usage

**First time:** run without arguments and the wizard will set everything up:

```bash
./smuf-server   # on the server
./smuf --setup  # on your machine
```

**After that:**

```bash
./smuf 3000                    # exposes localhost:3000
./smuf 3000 4000 5000          # multiple ports at once
./smuf --sub myapp 3000        # fixed URL: myapp.yourdomain.com
./smuf --tcp 22                # pure TCP tunnel (SSH, DB, etc.)
```

Output:

```
  Tunnel ready!

  Local   → http://localhost:3000
  Public  → https://a3f1c9.yourdomain.com

  Press Ctrl+C to stop
```

---

## Dashboard

While the server is running, open in your browser:

```
http://yourdomain.com:8080/
```

It is designed in [HashiCorp](https://www.hashicorp.com/) style: dark `#0d0e12` background, system-ui font, cards with micro-shadows and blue accent (`#1060ff`).

Shows all active tunnels with:
- Tunnel type (HTTP / TCP)
- Public URL
- Local port and client IP
- Uptime

Updates every 5 s. The JSON endpoint is at `/_smuf/tunnels`.

---

## Configuration

Everything goes through environment variables (or a `.env` file next to the binary).

### Server (`smuf-server`)

| Variable | Default | Description |
|----------|---------|-------------|
| `SMUF_DOMAIN` | `localhost` | Your base domain |
| `SMUF_AUTH_TOKEN` | — | Secret token (**recommended in production**) |
| `SMUF_CONTROL_PORT` | `7000` | Port for clients to connect |
| `SMUF_HTTP_PORT` | `8080` | Public HTTP port |
| `SMUF_HTTPS` | `false` | Automatic HTTPS with Let's Encrypt |
| `SMUF_HTTPS_PORT` | `443` | HTTPS port |
| `SMUF_ACME_EMAIL` | — | Email for certificate notices |
| `SMUF_MAX_CONNS_PER_IP` | `5` | Max tunnels per IP |
| `SMUF_HANDSHAKE_TIMEOUT` | `10s` | Handshake timeout |
| `SMUF_TCP_PORT_RANGE` | — | Public TCP port range (e.g. `20000-30000`) |

### Client (`smuf`)

| Variable | Default | Description |
|----------|---------|-------------|
| `SMUF_SERVER` | `localhost:7000` | Server address |
| `SMUF_AUTH_TOKEN` | — | Token (must match the server) |
| `SMUF_SUBDOMAIN` | — | Fixed subdomain (equivalent to `--sub`) |

**Server `.env` example:**
```env
SMUF_DOMAIN=yourdomain.com
SMUF_AUTH_TOKEN=a-long-secret-token
# SMUF_HTTPS=true
# SMUF_ACME_EMAIL=you@email.com
```

**Client `.env` example:**
```env
SMUF_SERVER=yourdomain.com:7000
SMUF_AUTH_TOKEN=a-long-secret-token
# SMUF_SUBDOMAIN=myapp
```

> Generate a secure token with `openssl rand -hex 32`

---

## How it works

```
smuf 3000  ──TCP──►  smuf-server :7000
                           │
              "PORT 3000 SUB myapp"  →  "OK myapp https://myapp.yourdomain.com"
                           │
                   yamux (multiplexing)
                           │
          request → myapp.yourdomain.com → yamux stream → localhost:3000
```

Uses [`hashicorp/yamux`](https://github.com/hashicorp/yamux) to multiplex multiple HTTP requests over a single TCP connection.

---

## Roadmap

- [x] Automatic HTTPS with Let's Encrypt
- [x] Token authentication
- [x] Per-IP rate limiting
- [x] Real-time web dashboard (HashiCorp style)
- [x] Multiple tunnels per process
- [x] Custom subdomain
- [x] WebSockets
- [x] TCP tunnels (not only HTTP)
- [x] Official Docker image
- [x] Pre-compiled binaries
- [ ] Server-Sent Events (SSE)

---

## Project structure

```
smuf/
├── cmd/
│   ├── smuf/           # Client (your machine)
│   └── smuf-server/    # Server (your VPS)
└── internal/
    ├── tunnel/         # Registry + BufConn
    ├── wizard/         # Interactive setup
    └── logger/         # Timestamped logging
```

---

## Contributing

Found a bug or have an idea? Open an [issue](../../issues) or submit a pull request.

---

<div align="center">
  <img src="logo.svg" width="40" alt="smuf"/>
  <br/>
  <sub>Built with Go · <a href="LICENSE">Apache 2.0</a></sub>
</div>
