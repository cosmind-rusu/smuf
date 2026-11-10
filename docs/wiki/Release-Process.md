# Release Process

Releases are driven by version tags and GoReleaser. Publishing is a separate maintainer action because automation creates a draft release.

## Before tagging

1. Use Go 1.26.3 or newer, matching `go.mod` and the release workflow.
2. Run `go fmt ./...`, `go vet ./...`, `go test ./...`, and `go build ./...`.
3. Confirm that the CI toolchain also satisfies the minimum version in `go.mod`.
4. Review the release notes and public documentation for version, protocol, configuration, and security changes.
5. Choose a semantic version tag such as `v0.5.0`. This is an example, not a statement that a release with that version exists.

## Automated release

Tag the intended commit and push the tag:

```bash
git tag v0.5.0
git push origin v0.5.0
```

A tag matching `v*` triggers `.github/workflows/release.yml`. The workflow:

1. checks out the full Git history;
2. sets up Go 1.26.3;
3. runs `go test ./...`;
4. runs GoReleaser v2 with `release --clean`.

The current `.goreleaser.yaml` builds:

- Linux amd64 and arm64;
- macOS amd64 and arm64;
- Windows amd64.

Windows arm64 is explicitly excluded. CGO is disabled, and version, commit, and date are injected through linker flags. GoReleaser creates archives and `checksums.txt`, generates a changelog from commits, and creates a draft GitHub release.

## Publish the draft

1. Open [Releases](https://github.com/cosmind-rusu/smuf/releases).
2. Inspect the draft version, commit, changelog, and every artifact.
3. Verify the archive contents and checksums.
4. Publish only when the intended binaries are present and tested.

A completed automation run does not make the draft publicly available by itself.

## Local artifacts without creating a release

With GoReleaser v2 installed, build the configured targets locally:

```bash
goreleaser build --clean
```

Inspect its output before distributing anything. Cross-compilation alone is not a runtime compatibility test.

For a single local binary with explicit version metadata:

```bash
go build -ldflags="-X main.version=v0.5.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o smuf ./cmd/smuf
go build -ldflags="-X main.version=v0.5.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o smuf-server ./cmd/smuf-server
```

Without those linker variables, `-v` reports `dev`.

## Pre-release tags and changelog

GoReleaser's `prerelease: auto` setting marks tags it recognizes as pre-releases. Use a conventional suffix such as `v0.5.0-rc.1`, and inspect the resulting draft rather than assuming its classification.

The changelog sorts commits in ascending order and excludes commit subjects beginning with `docs:`, `test:`, or `chore:`, plus matching merge-commit subjects. Using `feat:` and `fix:` for relevant changes makes the generated draft easier to review, but maintainers remain responsible for editing the final notes.
