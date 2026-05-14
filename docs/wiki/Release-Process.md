# Release Process

How to create a new release of smuf.

## Automated release

Releases are automated via GitHub Actions and GoReleaser.

### Trigger a release

```bash
# Tag the current commit
git tag v0.4.0

# Push the tag
git push origin v0.4.0
```

That's it! The CI pipeline will:
1. Run `go test ./...`
2. Build binaries for all platforms (Linux, macOS, Windows × amd64, arm64)
3. Package archives (`.tar.gz` / `.zip`)
4. Generate `checksums.txt`
5. Create a **draft release** on GitHub

### Publish the release

1. Go to [Releases](https://github.com/cosmind-rusu/smuf/releases)
2. Find the draft release
3. Review the changelog and assets
4. Click **Publish release**

## Manual release (without CI)

```bash
# Build locally with GoReleaser
goreleaser release --clean

# Or build manually
go build -ldflags="-X main.version=v0.4.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o smuf ./cmd/smuf
go build -ldflags="-X main.version=v0.4.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o smuf-server ./cmd/smuf-server
```

## Versioning

smuf follows [Semantic Versioning](https://semver.org/):

- **v0.3.0** — First public release
- **v0.4.0** — Minor features (backward compatible)
- **v1.0.0** — Stable API

## Pre-release versions

Tags like `v0.4.0-beta1` or `v0.4.0-rc.1` are automatically marked as pre-release by GoReleaser.

## Changelog

The changelog is auto-generated from commit messages. To keep it clean:

- Use conventional commits: `feat:`, `fix:`, `docs:`, `chore:`, `test:`
- Commits prefixed with `docs:`, `test:`, or `chore:` are excluded from the changelog
- Merge commits and branch merges are also excluded
