# Releasing

A release is a set of static binaries, a checksum file and a container image, all built from one tagged commit.

## Versioning

The version is the single line in `cmd/fleetwatch/VERSION`, embedded in the binary. The project follows [semantic versioning](https://semver.org/) and records changes in `CHANGELOG.md` in the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format. A version is cut when a release is made, not per commit. Until 1.0, a minor version may change the wire contract; a deprecated field is announced in the changelog one minor version before it goes (`claude_version` is deprecated in favour of `agent_version` and is removed in 0.2).

`/version` resolves the version in this order: `APP_VERSION` if set and not `dev`, then a link-time stamp (`-X github.com/wayju/fleetwatch/internal/core.BuildVersion`), then the embedded `VERSION`. The commit comes from `APP_COMMIT`, then the link-time stamp (`...core.BuildCommit`), else `unknown`.

## Cutting a release

1. Move the `CHANGELOG.md` entries from Unreleased to a dated section for the new version, and set `cmd/fleetwatch/VERSION` to match.
2. Merge that change, then tag the merge commit `v<version>`, for example `v0.1.0`. The release workflow refuses a tag that does not equal `v` plus the contents of `VERSION`.
3. The release workflow, triggered only by a `v*` tag, in three jobs:
   - `build` checks that the tag is `vX.Y.Z`, equals `v` plus the contents of `VERSION`, and sits on `main`; runs `go vet ./...` and `go test -race ./...`; cross-compiles `fleetwatch-linux-amd64` and `fleetwatch-linux-arm64` with `CGO_ENABLED=0`, `-trimpath` and the version and commit stamped in; and writes `SHA256SUMS`. It has no write permission.
   - `image` builds a multi-arch (amd64 and arm64) image from `Dockerfile.release`, pushes `ghcr.io/wayju/fleetwatch:<version>` and `:<major.minor>`, and attests its build provenance.
   - `release` attests the binaries and publishes a GitHub release with the binaries and `SHA256SUMS` attached.

   While the repository is private, both attestation steps are skipped and the release is marked as a prerelease. GitHub stores attestations only for public repositories on a personal account, so `gh attestation verify` works only for releases cut after the repository went public.

## Verifying a download

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify fleetwatch-linux-amd64 --repo wayju/FleetWatch
```

The checksum proves the binary matches the published sums, and the attestation proves the binary was built by this repository's release workflow from the tagged commit. Pin an image by digest rather than by tag if you need it to stay put.

## Which workflow does what

The public repository carries its release automation under `.github/workflows/`. The project's maintainers also build from a private source repository whose pipeline has its own deployment and approval steps. That pipeline is not part of this repository and a release does not depend on it: the public tree is exported from the private one with the private-only paths removed, and the exported tree must pass `go vet ./...` and `go test ./...`, contain no private host or account names, and pass a secret scan before it is published (ADR-011). The public history therefore holds one commit per release.

## Before you tag

- `go vet ./... && go test ./...` is green.
- `CHANGELOG.md` and `VERSION` agree.
- `examples/prices.json` is understood to be a dated sample; it is not maintained as part of a release.
- A secret scan of the tree finds nothing.
