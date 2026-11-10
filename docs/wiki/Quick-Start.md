# Quick Start

This example exposes `localhost:3000` through `tunnels.example.com` with the default public HTTP port. Replace the domain and token with your own values.

## 1. Prepare DNS and firewall rules

Point both `tunnels.example.com` and `*.tunnels.example.com` to the server. Allow inbound TCP on `7000` for smuf clients and `8080` for public HTTP requests.

For a local-only test, use the default `SMUF_DOMAIN=localhost` and connect to `127.0.0.1`. Subdomain resolution for `*.localhost` varies by system, so set the `Host` header explicitly when testing with curl.

## 2. Start the server

```bash
export SMUF_DOMAIN=tunnels.example.com
export SMUF_AUTH_TOKEN="$(openssl rand -hex 32)"
printf 'Client token: %s\n' "$SMUF_AUTH_TOKEN"
./smuf-server
```

Keep the token available for the client. If you omit it, the control listener accepts unauthenticated tunnel requests.

You can instead run `./smuf-server --setup`. The interactive wizard generates a token and saves configuration in your user profile. Its HTTPS option requires public ports `80` and `443` plus working DNS before certificates can be issued.

## 3. Start your local application

Make sure it responds on the client machine:

```bash
curl http://localhost:3000
```

The client refuses to register a port when its initial connection to that local port fails.

## 4. Start the client

```bash
export SMUF_SERVER=tunnels.example.com:7000
export SMUF_AUTH_TOKEN='paste-the-server-token-here'
./smuf 3000
```

With HTTPS disabled and the default ports, output looks like:

```text
  Tunnel ready!

  Local   → http://localhost:3000
  Public  → http://a3f1c9d4e2b8.tunnels.example.com:8080

  Press Ctrl+C to stop
```

Use the URL printed by your client. The random ID in this example is not a value you should expect to receive.

For a fixed name, run `./smuf --sub myapp 3000` and use `myapp.tunnels.example.com` if that subdomain is available.

## 5. Test the tunnel

Open the printed URL or run:

```bash
curl http://a3f1c9d4e2b8.tunnels.example.com:8080/
```

Stop the client with Ctrl+C. Closing its control session removes the tunnel from the server registry and releases a raw TCP port if one was allocated.

## Next steps

- [Client Usage](Client-Usage) for multiple ports and raw TCP
- [Configuration](Configuration) for saved settings and every variable
- [Server Setup](Server-Setup) for HTTPS and service management
- [Troubleshooting](Troubleshooting) if the printed URL does not work
