# Configuration

smuf uses process environment variables, a JSON file written by its setup wizard, or a simple local `.env` file. Command-line flags select client modes and can override the requested subdomain.

## Server (`smuf-server`)

| Variable | Default | Description |
|----------|---------|-------------|
| `SMUF_DOMAIN` | `localhost` | Base domain for `Host` routing, certificate policy, and public URLs |
| `SMUF_AUTH_TOKEN` | unset | Shared secret for client registration; empty disables authentication |
| `SMUF_CONTROL_PORT` | `7000` | Raw TCP listener for client control connections |
| `SMUF_HTTP_PORT` | `8080` | Public HTTP listener; with HTTPS enabled, handles ACME HTTP-01 and redirects |
| `SMUF_HTTPS` | `false` | Enables automatic public HTTPS |
| `SMUF_HTTPS_PORT` | `443` | Public HTTPS listener |
| `SMUF_ACME_EMAIL` | unset | Optional Let's Encrypt contact |
| `SMUF_ACME_CACHE` | `certs` | Certificate cache directory, relative to the server working directory unless absolute |
| `SMUF_PUBLIC_HTTP_PORT` | `SMUF_HTTP_PORT` | External port printed in HTTP URLs |
| `SMUF_PUBLIC_HTTPS_PORT` | `SMUF_HTTPS_PORT` | External port printed in HTTPS URLs |
| `SMUF_MAX_CONNS_PER_IP` | `5` | Concurrent control connections accepted per source IP |
| `SMUF_CONN_RATE_PER_MIN` | `30` | New control connection attempts accepted per source IP in a one-minute window |
| `SMUF_HANDSHAKE_TIMEOUT` | `10s` | Handshake read deadline in Go duration format |
| `SMUF_TCP_PORT_RANGE` | unset | Inclusive raw TCP public port range, for example `20000-20100` |

Boolean values accept `1`, `true`, `yes`, or `on`, without case sensitivity. Any other non-empty value is false. Invalid integer or duration values fall back to their defaults. An unset or invalid TCP range disables raw TCP tunnels.

Each tunnel consumes one concurrent control connection. These limits do not apply to public HTTP or TCP visitors. If several clients connect through the same NAT or TCP proxy, the server can count them under one observed IP.

The public-port overrides affect only URL generation. Configure the actual external listener, forwarding rule, and firewall separately.

### Direct HTTPS example

```env
SMUF_DOMAIN=tunnels.example.com
SMUF_AUTH_TOKEN=paste-a-random-token-here
SMUF_CONTROL_PORT=7000
SMUF_HTTP_PORT=80
SMUF_HTTPS=true
SMUF_HTTPS_PORT=443
SMUF_ACME_EMAIL=admin@example.com
SMUF_ACME_CACHE=/var/lib/smuf/certs
SMUF_MAX_CONNS_PER_IP=5
SMUF_CONN_RATE_PER_MIN=30
```

HTTP-01 requires public port `80` to reach `SMUF_HTTP_PORT`. Persist the ACME cache and restrict access to it.

## Client (`smuf`)

| Variable | Default | Description |
|----------|---------|-------------|
| `SMUF_SERVER` | `localhost:7000` | Server control address in `host:port` form |
| `SMUF_AUTH_TOKEN` | unset | Token matching the server configuration |
| `SMUF_SUBDOMAIN` | unset | Fixed HTTP subdomain; `--sub` takes precedence |

Example:

```env
SMUF_SERVER=tunnels.example.com:7000
SMUF_AUTH_TOKEN=paste-the-same-token-here
SMUF_SUBDOMAIN=myapp
```

When opening several HTTP ports in one command, the fixed subdomain applies only to the first port. Raw TCP tunnels do not use a custom HTTP subdomain.

## Loading order

Each process first loads one file-based configuration source, then reads its environment:

1. On Windows, `%APPDATA%\smuf\config.json`, or `%USERPROFILE%\smuf\config.json` if `APPDATA` is empty.
2. On other systems, `$HOME/smuf/config.json`.
3. `.env` beside the executable. Only when that file is absent, `.env` in the current working directory.

If the profile JSON exists and parses successfully, the loader stops there and never reads `.env`. Values from the selected file are set only when the corresponding process environment variable is empty. Therefore, a non-empty process environment variable wins; an empty one does not override a saved value.

The server and client wizards write the same path for the current OS user. Each writes a new JSON object rather than merging both configurations. If you run both programs under one account, prefer explicit process environments, separate OS users, or carefully managed separate configuration contexts.

## `.env` syntax

The parser handles blank lines, comments whose first non-space character is `#`, and `KEY=VALUE` entries. It splits on the first `=` and trims whitespace around keys and values.

It does not support shell execution, variable expansion, `export` syntax, or quote removal. For example:

```env
# Wrong: this entire expression becomes the token
SMUF_AUTH_TOKEN=$(openssl rand -hex 32)

# Wrong for most tokens: quotes become part of the value
SMUF_AUTH_TOKEN="abc123"

# Right: paste the generated value
SMUF_AUTH_TOKEN=abc123
```

Generate a token in your shell with `openssl rand -hex 32`, or let the server wizard generate one. Store configuration files outside source control and restrict their read permissions.

## Wizard behavior

`smuf-server --setup` asks for a domain, generates 32 random bytes as a 64-character hexadecimal token, and offers HTTPS when the domain is not `localhost`. Choosing HTTPS sets the HTTP port to `80` in the saved configuration.

`smuf --setup` asks for the server address, token, and optional subdomain. In an interactive terminal, the server also starts its wizard without `--setup` when `SMUF_DOMAIN` is absent. The client does the same when `SMUF_SERVER` is absent and no positional arguments were supplied, then prompts for a local port if needed.
