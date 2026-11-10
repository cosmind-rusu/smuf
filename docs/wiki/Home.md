# smuf wiki

smuf is a self-hosted HTTP and raw TCP tunnel client and server written in Go. It exposes a local port through your own reachable server. HTTP tunnels use subdomains of your base domain; raw TCP tunnels use ports allocated from a configured range.

There is no smuf account or hosted control plane. Your infrastructure, provider quotas, network, and smuf's per-IP control limits still apply.

## Pages

| Page | Description |
|------|-------------|
| [Installation](Installation) | Binaries, source builds, and release targets |
| [Quick Start](Quick-Start) | Configure a server and expose a local HTTP port |
| [Server Setup](Server-Setup) | DNS, HTTPS, Docker, and systemd |
| [Client Usage](Client-Usage) | HTTP, TCP, subdomains, and reconnection |
| [Configuration](Configuration) | Variables, defaults, and file loading order |
| [Architecture](Architecture) | Handshake, routing, yamux, and state |
| [Troubleshooting](Troubleshooting) | Connection, DNS, TLS, and proxy failures |
| [Release Process](Release-Process) | Tags, GoReleaser, artifacts, and publication |

Start with the main [README](https://github.com/cosmind-rusu/smuf#readme) for the security model and current limitations. In particular, public HTTPS does not encrypt the client-to-server control channel, and the dashboard has no built-in authentication.

## Project links

- [GitHub repository](https://github.com/cosmind-rusu/smuf)
- [Releases](https://github.com/cosmind-rusu/smuf/releases)
- [Issues](https://github.com/cosmind-rusu/smuf/issues)
- [Discussions](https://github.com/cosmind-rusu/smuf/discussions)
