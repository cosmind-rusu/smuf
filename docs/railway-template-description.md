# Deploy and Host smuf on Railway

**smuf** is a self-hosted HTTP tunnel tool — like ngrok, but yours. Deploy `smuf-server` on Railway for zero-quota, always-warm tunnels to your local services.

## About Hosting smuf

Hosting smuf on Railway is straightforward. The `smuf-server` binary runs in a Docker container, listening for control connections from your local client and proxying HTTP/HTTPS traffic through. Railway handles TLS, domain management, and scaling — you just set your domain and auth token. The server maintains persistent multiplexed connections via yamux, allowing multiple tunnels and concurrent requests over a single TCP link. Railway's built-in public networking maps your custom domain directly to the tunnel endpoint.

## Common Use Cases

- **Local web development** — Expose localhost apps to clients, test webhooks, or demo work-in-progress without deploying
- **IoT and API gateways** — Tunnel SSH, databases, or REST APIs from devices behind NAT or firewalls
- **Multi-service debugging** — Expose several local services simultaneously (frontend, backend, database) on distinct subdomains

## Dependencies for smuf Hosting

- **Railway public networking** — HTTP proxy with custom domain pointing to the smuf-server instance
- **Auth token** — A shared secret between client and server (auto-generated via Railway template variables)

### Deployment Dependencies

- [GitHub Repository](https://github.com/cosmind-rusu/smuf)
- [smuf Client binary](https://github.com/cosmind-rusu/smuf/releases) — runs on your local machine

### Implementation Details

```bash
# On your local machine, after deploying the server:
export SMUF_SERVER=your-railway-domain.railway.app:7000
export SMUF_AUTH_TOKEN=<same-token-from-template>
./smuf 3000
# → https://a3f1c9.your-railway-domain.railway.app → localhost:3000
```

## Why Deploy smuf on Railway?

Railway is a singular platform to deploy your infrastructure stack. Railway will host your infrastructure so you don't have to deal with configuration, while allowing you to vertically and horizontally scale it.

By deploying smuf on Railway, you are one step closer to supporting a complete full-stack application with minimal burden. Host your servers, databases, AI agents, and more on Railway.
