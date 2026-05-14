# Architecture

How smuf works under the hood.

## Overview

```
┌─────────────────┐         ┌─────────────────┐         ┌──────────────┐
│   Your machine  │  TCP    │     Your VPS    │  HTTP   │  Internet    │
│                 │◄───────►│                 │◄───────►│              │
│  localhost:3000 │  :7000  │  smuf-server    │  :8080  │  myapp.com   │
│       │         │         │       │         │         │              │
│    ┌──┴──┐      │         │    ┌──┴──┐      │         │              │
│    │smuf │      │         │    │tun  │      │         │              │
│    └─────┘      │         │    │reg  │      │         │              │
│                 │         │    └─────┘      │         │              │
└─────────────────┘         └─────────────────┘         └──────────────┘
```

## Connection flow

1. **Client connects** — `smuf` opens a TCP connection to `smuf-server:7000`
2. **Handshake** — The client sends a request: `PORT 3000` or `AUTH <token> PORT 3000`
3. **Server assigns URL** — The server creates a tunnel entry and responds: `OK a3f1c9 https://a3f1c9.yourdomain.com`
4. **yamux session** — Both sides establish a yamux session over the TCP connection
5. **Multiplexing** — When HTTP requests arrive at the public URL, the server opens a yamux stream, sends it to the client, which proxies to `localhost:3000`

## Key technologies

### [yamux](https://github.com/hashicorp/yamux) (Yet Another Multiplexer)

Multiplexes multiple streams over a single TCP connection. This is the core of smuf:

- One TCP connection between client and server carries many HTTP requests
- Each HTTP request gets its own yamux stream
- Streams are lightweight and created on demand

### Subdomain-based routing

Each tunnel gets a unique subdomain (random hex or custom). The server's HTTP handler reads the `Host` header and routes to the correct tunnel.

```
Request: GET /api/users HTTP/1.1
         Host: a3f1c9.yourdomain.com

Server extracts "a3f1c9" from host → looks up tunnel → yamux stream → client
```

### Connection recovery

If the TCP connection drops, the client automatically reconnects:

```go
for {
    sess, err := connectTunnel(serverAddr, ...)
    if err != nil {
        time.Sleep(5 * time.Second)
        continue
    }
    serveStreams(sess, port)
    time.Sleep(3 * time.Second)
}
```

## Dashboard

The server has a built-in web dashboard at `http://yourdomain.com:8080/` showing:
- Active tunnels (type, URL, local port, client IP, uptime)
- Real-time updates every 5 seconds
- JSON endpoint at `/_smuf/tunnels`

## Code structure

```
smuf/
├── cmd/
│   ├── smuf/           # Client binary
│   └── smuf-server/    # Server binary
└── internal/
    ├── tunnel/         # Registry + buffered connection
    ├── wizard/         # Interactive setup wizard
    └── logger/         # Timestamped logging
```
