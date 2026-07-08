# Quick Start

Get your first tunnel running in under 2 minutes.

## 1. Start the server (on your VPS)

```bash
# Set your domain
export SMUF_DOMAIN=yourdomain.com

# Generate a secure auth token
export SMUF_AUTH_TOKEN=$(openssl rand -hex 32)

# Start the server
./smuf-server
```

You should see:
```
smuf-server v0.3.0
  Domain: yourdomain.com
  Control port: 7000
  HTTP port: 8080
  Auth: enabled
```

## 2. Start the client (on your machine)

```bash
# Set server address
export SMUF_SERVER=yourdomain.com:7000

# Use the same token
export SMUF_AUTH_TOKEN=<same-token-from-above>

# Expose your local app (e.g., port 3000)
./smuf 3000
```

Output:
```
  Tunnel ready!

  Local   → http://localhost:3000
  Public  → https://a3f1c9.yourdomain.com

  Press Ctrl+C to stop
```

## 3. Test it

Open `https://a3f1c9.yourdomain.com` in your browser. You should see your local app!

## What's next?

- [Client Usage](Client-Usage) — advanced client features
- [Configuration](Configuration) — all settings explained
- [Server Setup](Server-Setup) — production server configuration
