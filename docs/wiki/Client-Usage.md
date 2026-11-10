# Client Usage

The `smuf` client connects local ports on its own machine to `smuf-server`. It always dials `localhost:<port>`; there is no flag for forwarding a different local hostname or a remote third machine.

## Setup and basic commands

```bash
./smuf --setup             # Run the interactive configuration wizard
./smuf 3000                # Expose localhost:3000 over HTTP
./smuf 3000 4000 5000      # Open three independent HTTP tunnels
./smuf --sub myapp 3000    # Request a fixed HTTP subdomain
./smuf --tcp 22            # Expose localhost:22 through a public raw TCP port
./smuf -h                  # Show command help
./smuf -v                  # Show the compiled version string
```

Before connecting to the server, the client checks every requested local port with a two-second TCP dial. Start the local applications first.

In an interactive terminal, running without arguments starts the wizard when `SMUF_SERVER` is unset, then asks for a port when needed. Non-interactive use requires ports as arguments; the server address can come from the default, saved files, or the environment.

## HTTP tunnels

HTTP is the default mode. The server proxies requests to the local HTTP service and supports streaming responses, Server-Sent Events, long polling, and upgrades such as WebSocket.

```bash
./smuf 3000
# Example result with default HTTP settings:
# http://a3f1c9d4e2b8.tunnels.example.com:8080
```

The generated ID is 12 hexadecimal characters. It is assigned again when a new tunnel is registered, so do not treat a random URL as permanent.

The public `Host` value is preserved for the local application. Applications that generate absolute links must also handle the public scheme and forwarding headers correctly, especially when an external proxy terminates TLS.

HTTP mode expects the local service to speak HTTP. To carry another protocol, including a service with its own TLS listener, use raw TCP mode.

## Multiple ports

```bash
./smuf 3000 4000 5000
```

Each port gets a separate control connection and yamux session. The default server limit of five concurrent control connections per source IP therefore applies to tunnels, not to the number of client processes.

`--sub` and `SMUF_SUBDOMAIN` apply only to the first port and only in HTTP mode. Other ports receive random IDs.

## Fixed subdomains

```bash
./smuf --sub myapp 3000
```

The server normalizes the requested name to lowercase. The result must have 1 to 63 letters, digits, or hyphens, and cannot start or end with a hyphen. Registration fails if another active tunnel already owns the name. The name becomes available after that tunnel closes, but another client can claim it first.

A fixed name provides stable routing while you can retain it. It does not reserve the name across server restarts or while your client is disconnected.

## Raw TCP tunnels

Set an allocation range on the server first:

```bash
export SMUF_TCP_PORT_RANGE=20000-20100
./smuf-server
```

Then expose the local TCP service:

```bash
./smuf --tcp 5432
# Example result: tcp://tunnels.example.com:20000
```

Use the printed host and port in the client for that service. Every public port must be reachable through the host firewall and any container or provider networking. The allocated port can change when the tunnel is re-created.

Raw TCP adds no authentication or encryption to the exposed protocol. Use the service's own protections and restrict public access before exposing SSH, a database, or another sensitive service. UDP is not supported.

## Reconnection and shutdown

After a session drops, the client waits three seconds and then retries. Failed registration attempts retry every five seconds. A random HTTP ID or raw TCP port can change after reconnecting; the client prints an updated URL when that happens.

Ctrl+C stops the client. Active public connections tied to a closed yamux session end with that session.

## Environment variables

| Variable | Description |
|----------|-------------|
| `SMUF_SERVER` | Control address, for example `tunnels.example.com:7000` |
| `SMUF_AUTH_TOKEN` | Shared token required when enabled on the server |
| `SMUF_SUBDOMAIN` | Fixed HTTP subdomain, overridden by `--sub` |

See [Configuration](Configuration) for profile paths, `.env` syntax, and precedence. The control connection is raw TCP without application-level TLS, even when public HTTPS is enabled.
