# Installation

smuf has two binaries: `smuf-server` runs on the public host and `smuf` runs beside the local application. You can use published build artifacts or compile both from source.

## Download release artifacts

The published `v0.4.0` release is neither a draft nor a prerelease. Download its archives and `checksums.txt` from [Releases](https://github.com/cosmind-rusu/smuf/releases). It contains artifacts for:

| Platform | Architectures |
|----------|---------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64 |

Archives use the project name, version, OS, and architecture in their filenames. Windows artifacts are ZIP files; other platforms use tar archives. Each archive includes the client, server, README, and license. A separate `checksums.txt` is generated for release artifacts.

Extract the archive, compare its checksum with the release checksums, and place the binaries somewhere suitable for your user or service account. On Unix-like systems, putting them in a directory on `PATH` avoids the `./` prefix used in the examples.

Release automation creates drafts for subsequent releases. For any version other than the verified `v0.4.0` artifacts listed above, use the files actually visible on the Releases page rather than assuming that a tag has been published.

## Build from source

Requires [Go](https://go.dev/dl/) 1.26.3 or newer.

```bash
git clone https://github.com/cosmind-rusu/smuf.git
cd smuf
go build -o smuf-server ./cmd/smuf-server
go build -o smuf ./cmd/smuf
```

Verify the binaries:

```bash
./smuf -v
./smuf-server -v
```

A direct source build reports `dev`. Version, commit, and date are injected by GoReleaser for release builds.

### Cross-compilation

The binaries build with CGO disabled in release automation. For example, to build a Linux arm64 server from another development machine:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o smuf-server-linux-arm64 ./cmd/smuf-server
```

Cross-compiling proves that Go can produce the artifact, not that smuf has been tested on that target.

## Docker

The repository's `docker-compose.yml` builds only the server. Before starting it:

1. Replace the placeholder `SMUF_DOMAIN=tudominio.com` with your base domain.
2. Set a token in the shell running Compose.
3. Confirm that the published ports and DNS records match your deployment.

```bash
export SMUF_AUTH_TOKEN="$(openssl rand -hex 32)"
docker compose up -d
```

The Compose example publishes control port `7000`, HTTP port `8080`, and ports `80` and `443` for HTTPS. It also enables a raw TCP allocation range without publishing those ports. Add the necessary Compose port mappings before using `./smuf --tcp`.

See [Server Setup](Server-Setup) for HTTPS requirements and [Configuration](Configuration) for every variable.
