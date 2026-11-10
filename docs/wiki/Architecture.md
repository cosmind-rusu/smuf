# Architecture

## Components

smuf consists of two programs:

- `smuf` runs beside the local service, registers tunnels, accepts yamux streams, and copies bytes to `localhost`.
- `smuf-server` accepts control connections, keeps the active tunnel registry, routes public HTTP or raw TCP traffic, and serves the dashboard.

There is no external database or broker. The registry is an in-memory Go map protected for concurrent access.

## Registration flow

For each requested local port, the client opens a raw TCP connection to `SMUF_SERVER`, default `localhost:7000`.

```text
Client                                      Server
  │                                           │
  │  AUTH <token> PORT 3000 SUB myapp          │
  │───────────────────────────────────────────►│
  │                                           │ validate token, port, and name
  │                                           │ create ID and in-memory entry
  │  OK myapp http://myapp.example.com:8080    │
  │◄───────────────────────────────────────────│
  │                                           │
  │            yamux session on this TCP connection
```

Without authentication or a custom name, the command is `PORT 3000`. Raw TCP mode uses `TCP <port>`. The server reads at most 1024 bytes for the handshake and applies `SMUF_HANDSHAKE_TIMEOUT`, default `10s`.

Random HTTP and TCP tunnel IDs are six cryptographically random bytes encoded as 12 hexadecimal characters. A custom HTTP subdomain replaces the random ID after syntax and availability checks. The server checks custom-name uniqueness again during registry insertion to close the race between concurrent registrations.

Each tunnel has its own TCP connection and yamux session. Opening three ports in one client command creates three sessions, not one shared session.

## HTTP request path

The public HTTP listener compares the request's `Host` with `SMUF_DOMAIN`:

- The exact base domain serves the dashboard and `/_smuf/tunnels`.
- A subdomain selects the tunnel ID from the registry.
- Any other host receives a bad-request response; an inactive tunnel ID receives a not-found response.

For ordinary HTTP, Go's `httputil.ReverseProxy` forwards the request through a transport whose connections are yamux streams. HTTP keep-alive can reuse a stream for multiple requests, so a stream represents a proxied transport connection rather than necessarily one request. The transport pool is tied to a specific yamux session and cleaned up when that session closes.

The proxy preserves the public `Host` value and sets forwarding headers. WebSocket and other HTTP upgrades use the reverse proxy's upgrade handling. CONNECT has a separate raw-byte forwarding path. Streaming responses are not given a global response-header deadline, which allows slow responses, SSE, and long polling to remain open.

The client accepts each stream and dials `localhost:<registered-port>`. Two copy loops carry bytes in both directions until one side closes.

## Raw TCP path

A valid `SMUF_TCP_PORT_RANGE` creates a server-side allocator. For each raw TCP tunnel, the allocator finds a port that is not assigned to another active tunnel and can be bound temporarily. In the current implementation, the server registers the tunnel and sends `OK <id> tcp://<domain>:<port>` before creating the public listener.

This leaves a transient window in which the client can report the public address before the listener accepts connections. An immediate connection can therefore fail. If the subsequent listener bind fails, the server closes the session and removes the registration, but the success response has already been sent. This ordering is a current limitation, not a desirable startup guarantee.

Each accepted public TCP connection opens one yamux stream. The client copies that stream to the local TCP port. Closing the tunnel closes the listener and returns the allocation to the range.

The allocator does not publish Docker ports, change firewall rules, or configure a hosting provider. A successful local bind check is not proof that the internet can reach that port.

## Reconnection and lifecycle

The client serves a session until it closes, waits three seconds, and registers the tunnel again. Failed reconnection attempts repeat every five seconds. This is a new registration:

- a random ID can change;
- a fixed subdomain must still be available;
- a raw TCP port can change;
- active connections from the old session do not migrate.

On shutdown, the server closes the control listener and active yamux sessions, then waits up to five seconds for tunnel handlers to clean up registry entries and TCP listeners.

## State and scaling

Tunnel metadata and session references live only in the server process. There is no persistence, replication, or shared registry between replicas. Restarting the server drops registrations, and adding another replica does not make either one aware of the other's tunnels.

The per-IP control limits are not global resource controls. The server has no server-wide caps on client or visitor connections, yamux streams, memory, CPU, or bandwidth.

The dashboard reads a snapshot of the same in-memory registry. It exposes tunnel IDs, types, public URLs, local ports, client IPs, and creation timestamps without built-in authentication.

## Trust boundaries

The shared token authenticates tunnel registration only when configured. The control handshake and yamux session use raw TCP without application-level TLS. Automatic HTTPS applies to the separate public HTTP listener, between visitors and the server.

The ACME host policy accepts the base domain and every subdomain, including names without registered tunnels. TLS SNI can therefore trigger certificate work for arbitrary names under the domain. The HTTP-to-HTTPS redirect also uses the request `Host` without validating it. These are current exposure risks that require hardening or a trusted TLS proxy before a public multi-tenant deployment.

smuf does not authenticate public visitors or add encryption to a raw TCP service. Applications and network controls must provide those protections where required.

## Code structure

```text
smuf/
├── cmd/
│   ├── smuf/           # Client, registration, reconnection, local copying
│   ├── smuf-server/    # Control, proxy, TCP allocator, dashboard, tests
│   ├── test-tcp/       # Manual raw TCP check
│   └── test-websocket/ # Manual WebSocket check
└── internal/
    ├── tunnel/         # Registry and buffered connection wrapper
    ├── wizard/         # Interactive setup and configuration loader
    └── logger/         # Timestamped logging
```
