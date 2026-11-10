# Deploy smuf on Railway

smuf is a self-hosted HTTP and raw TCP tunnel server written in Go. A local `smuf` client connects to `smuf-server`, and yamux carries streams for each tunnel over that connection.

The repository includes a Docker build and Railway configuration with one replica. The template is a starting point, not proof that Railway's public networking satisfies smuf's routing requirements.

## Required network topology

Before deploying, confirm all of the following in your Railway project:

- A public raw TCP route reaches `SMUF_CONTROL_PORT`, default `7000`. An HTTP-only route cannot carry the smuf client handshake.
- The base public hostname reaches `SMUF_HTTP_PORT`, default `8080`, and every generated subdomain of that hostname also reaches the same service instance.
- Wildcard DNS and routing work for names such as `a3f1c9d4e2b8.<base-domain>`. A route for only the base hostname is insufficient for HTTP tunnels.
- If Railway maps a different external control port, `SMUF_SERVER` on the client uses that external host and port.
- If you enable raw TCP tunnels, each allocated public port has a separate working route. Setting `SMUF_TCP_PORT_RANGE` alone does not create provider networking.

The repository's `railway.json` selects `Dockerfile` and sets `numReplicas: 1`. It does not declare the routes above. The Dockerfile exposes container ports, which is not the same as creating public Railway routes or wildcard DNS.

## Configuration

The Railway-oriented `.env.example` defines:

```env
SMUF_DOMAIN=${RAILWAY_PUBLIC_DOMAIN}
SMUF_AUTH_TOKEN=${{secret(64, "abcdef0123456789")}}
```

These are Railway template expressions, not syntax supported by smuf's local `.env` parser. Confirm that Railway resolves them to the intended public base domain and token, and use the resolved values when configuring the client. Do not copy the expressions into a local `.env` file.

Set the same resolved token in the client environment:

```bash
export SMUF_SERVER=<external-control-host>:<external-control-port>
export SMUF_AUTH_TOKEN=<same-resolved-token>
./smuf 3000
```

Use the public URL and scheme that Railway actually provides. smuf prints `https` only when its own `SMUF_HTTPS` mode is enabled. If Railway terminates TLS in front of an HTTP listener, the URL printed by the client may not describe the external scheme correctly.

## HTTPS and application behavior

smuf's built-in automatic HTTPS expects direct public access to ports `80` and `443`, wildcard DNS, and a writable ACME cache. A platform-managed TLS route is a different topology. Do not enable `SMUF_HTTPS` unless its listener and ACME requirements are actually satisfied.

The current ACME policy accepts the base domain and every subdomain, including names without registered tunnels. Arbitrary TLS SNI values can trigger certificate issuance attempts, cache growth, and certificate authority quota consumption. The policy needs hardening before a public service. Prefer a trusted reverse proxy with host validation and a wildcard certificate obtained through DNS-01. smuf's own HTTPS redirect also trusts the request `Host` when constructing the target.

When TLS terminates at Railway, smuf receives HTTP and derives forwarding headers from that connection. Test login redirects, absolute URL generation, WebSocket upgrades, and streaming responses in the application behind the tunnel.

The base domain serves an unauthenticated dashboard and `/_smuf/tunnels`. These expose tunnel URLs, local ports, and client IPs. Restrict them at the network or proxy layer if they should not be public.

## Raw TCP tunnels

Raw TCP mode is disabled until `SMUF_TCP_PORT_RANGE` is set. The supplied Docker Compose example configures a range without publishing it, and the Railway configuration does not add public routes for that range. Do not advertise SSH, database, or other TCP access until the selected ports are reachable and the exposed service has its own authentication and encryption.

Public HTTPS does not encrypt the client-to-server control channel. The token handshake and yamux traffic use raw TCP without application-level TLS, so use a network path whose exposure you accept.

## Scaling and recovery

Keep one server replica. Active tunnels and their routing state live in memory inside `smuf-server`; Railway replicas do not share that state. A restart drops registrations, and clients create new registrations when they reconnect. Random HTTP IDs and allocated raw TCP ports can change.

There are no server-wide caps on client or visitor connections, yamux streams, memory, CPU, or bandwidth. Railway and other providers can impose their own connection, bandwidth, runtime, and pricing limits, but smuf does not provide global resource controls.

Use the current version for demos or protected personal use, not as a public multi-tenant service.
