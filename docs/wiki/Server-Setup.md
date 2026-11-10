# Server Setup

An internet-facing server needs more than a running binary: the control listener, public listeners, DNS, and advertised URLs must describe the same topology.

## Prerequisites

- A host with a public route to the ports you will use.
- A base domain such as `tunnels.example.com`.
- DNS records for both the base domain and `*.tunnels.example.com`, pointing to the same smuf-server instance.
- Inbound TCP access to `SMUF_CONTROL_PORT`, default `7000`, and `SMUF_HTTP_PORT`, default `8080`.
- A shared random token stored with restricted read access.
- For raw TCP, a configured allocation range and inbound access to those public ports.

A single base-domain DNS record is not enough for random or custom HTTP subdomains. A platform that provides only one HTTP hostname and no raw TCP control route does not satisfy these requirements.

Install the server using [Installation](Installation), then configure it using [Configuration](Configuration).

## Minimal HTTP configuration

```env
SMUF_DOMAIN=tunnels.example.com
SMUF_AUTH_TOKEN=paste-a-long-random-token-here
SMUF_CONTROL_PORT=7000
SMUF_HTTP_PORT=8080
SMUF_HTTPS=false
SMUF_MAX_CONNS_PER_IP=5
SMUF_CONN_RATE_PER_MIN=30
```

Start the server from the environment where these values are set:

```bash
./smuf-server
```

Public tunnel URLs will use `http://<id>.tunnels.example.com:8080`. The base domain serves the dashboard. Do not expose this setup beyond a trusted network without reading the security notes below.

## Direct HTTPS with Let's Encrypt

Use this configuration when ports `80` and `443` reach smuf-server directly:

```env
SMUF_DOMAIN=tunnels.example.com
SMUF_AUTH_TOKEN=paste-a-long-random-token-here
SMUF_CONTROL_PORT=7000
SMUF_HTTP_PORT=80
SMUF_HTTPS=true
SMUF_HTTPS_PORT=443
SMUF_ACME_EMAIL=admin@example.com
SMUF_ACME_CACHE=/var/lib/smuf/certs
```

The ACME HTTP-01 listener must be reachable on public port `80`. The HTTPS listener normally uses `443`. Both the base domain and wildcard records must resolve correctly, and the service account must be able to write to the cache directory.

The current ACME policy accepts the configured base domain and every subdomain, not only names with registered tunnels. A TLS client can vary SNI to trigger on-demand certificate issuance attempts, cache growth, and certificate authority quota consumption. Keep the cache across restarts, but do not treat persistence as a defense against unwanted requests.

This policy needs hardening before built-in HTTPS is exposed as a public service. Until then, prefer a reverse proxy that validates allowed hosts and terminates TLS with a wildcard certificate provisioned through DNS-01. This avoids smuf's per-SNI HTTP-01 issuance path. When smuf's own HTTPS redirect is enabled, it also constructs the redirect target from the request `Host` without validating it, so host validation at a trusted proxy is important.

Public HTTPS does not encrypt the raw TCP control connection between `smuf` and `smuf-server`.

If an external router maps different public ports, `SMUF_PUBLIC_HTTP_PORT` and `SMUF_PUBLIC_HTTPS_PORT` adjust the ports printed in tunnel URLs. They do not configure the mapping itself.

## systemd service

The following is a Linux systemd example. It uses a dynamic service account and a persistent state directory. The capability setting permits binding to ports `80` and `443` without running the process as root.

Create `/etc/smuf/smuf-server.env` with the desired configuration, including an absolute `SMUF_ACME_CACHE=/var/lib/smuf/certs` when HTTPS is enabled. Then create `/etc/systemd/system/smuf-server.service`:

```ini
[Unit]
Description=smuf HTTP and TCP tunnel server
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/smuf/smuf-server.env
ExecStart=/usr/local/bin/smuf-server
Restart=on-failure
RestartSec=5
DynamicUser=yes
StateDirectory=smuf
WorkingDirectory=/var/lib/smuf
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
```

Enable and inspect the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now smuf-server
sudo systemctl status smuf-server
journalctl -u smuf-server
```

Check that the environment file is readable by systemd but not by unrelated users. Adapt paths and service management to your distribution and local policy.

## Docker Compose

The supplied Compose example builds the server, publishes `7000`, `8080`, `80`, and `443`, and mounts `./certs` at `/app/certs`. Replace its placeholder domain and provide `SMUF_AUTH_TOKEN` in the Compose environment.

For direct HTTPS, change the example to use `SMUF_HTTP_PORT=80` and `SMUF_HTTPS=true`; merely enabling HTTPS while leaving the ACME listener on an unreachable port will not satisfy HTTP-01.

The example sets `SMUF_TCP_PORT_RANGE=20000-30000` but does not publish those ports. Add explicit host mappings for a narrower range you actually need, then allow those host ports through external firewalls. Container port declarations alone do not create provider firewall rules.

## Reverse proxies and hosted platforms

The control port requires raw TCP forwarding. An HTTP-only reverse proxy cannot accept the smuf client handshake. Public HTTP routing must preserve the subdomain in `Host`, including for WebSocket upgrades and streaming responses.

When a proxy terminates TLS, smuf sees an HTTP connection and derives forwarding headers from that request. Test absolute redirects and URL generation in the local application. The client prints `https` only when smuf's own `SMUF_HTTPS` mode is enabled, so externally terminated TLS also requires care when interpreting the printed URL.

The server stores active tunnel state in memory. Keep control and public traffic on the same instance and do not assume that horizontal scaling will work. The supplied Railway configuration fixes one replica. See the [Railway deployment notes](https://github.com/cosmind-rusu/smuf/blob/main/docs/railway-template-description.md) for the remaining networking checks.

## Internet-facing checklist

- Use the current server for demos or protected personal use, not as a public multi-tenant service.
- Set `SMUF_AUTH_TOKEN`; an empty value disables client authentication.
- Treat the control channel as unencrypted TCP. Use a protected network path when token and tunneled traffic must not cross the network in clear text.
- Protect exposed applications independently. Tunnel registration authentication does not authenticate public visitors.
- Restrict the unauthenticated dashboard and `/_smuf/tunnels` if tunnel URLs, local ports, and client IPs should not be public.
- Open only required ports, and restrict source addresses for sensitive raw TCP services where your network allows it.
- Keep control limits enabled and remember that clients behind NAT or a TCP proxy can share the same observed IP bucket. There are no server-wide limits on connections, yamux streams, memory, CPU, or bandwidth.
- Persist and protect ACME state, monitor service logs, and test recovery after a server restart. Do not expose the current per-SNI ACME policy or unvalidated HTTPS redirect directly to an untrusted public network.
