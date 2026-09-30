# Development

## Build and test

```bash
go build ./...
go vet ./...
test -z "$(gofmt -l .)"
go test ./...
node --test postinstall.test.js
```

The default tests run offline against a hand-written fixture in [`internal/atlas/testdata/`](https://github.com/GeiserX/biblical-atlas-mcp/tree/main/internal/atlas/testdata). They cover the store (ETag, 304, failures, disk cache, local-file mode) with an injected clock, the year conversion, search scoring, the Bible reference parser, every tool through a real `tools/call` message, the annotations, every URL the tools return, `run()` in both transports with the HTTP limits, and the agreement between `package.json`, `server.json`, `postinstall.js` and `.goreleaser.yaml`.

The fixture keeps the real structure and real ids of a handful of records; every text in it is a short invented sentence. When the atlas adds a field that the server maps, add it to the fixture with a test.

## Live test

```bash
BIBLICAL_ATLAS_LIVE=1 go test ./internal/... -run TestLive
```

It downloads the real data file, checks that its format is known, that no record points at a missing id, that every event passage and every book name parses and every source URL is HTTPS, then calls every tool with ids taken from the file. `TestLiveAnswerSizes` calls the tools across the whole file and fails when an answer passes 60 KB with default arguments or 70 KB at any page size, so answers stay under the 25,000-token tool-output limit of Claude Code. It never runs in CI.

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/server` | `run()`: flags, config, store, server, transports |
| `internal/config` | Environment variables and their validation |
| `internal/atlas` | The data file model, loading, refresh, disk cache, indexes, search and the Bible reference parser. No other package reads the file or the network. |
| `internal/tools` | The nine tools, their output builders and argument helpers |
| `version` | Build metadata set by `-ldflags` |

## Release

1. Update [`CHANGELOG.md`](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/CHANGELOG.md), and set the same version in [`package.json`](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/package.json) and both places in [`server.json`](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/server.json). A test fails when they disagree.
2. Merge to `main` with CI green.
3. Tag `vX.Y.Z` and push the tag. [`release.yml`](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/.github/workflows/release.yml) then:
   - builds the binaries, archives and `checksums.txt` with GoReleaser and publishes the GitHub release;
   - builds the linux/amd64 and linux/arm64 image and pushes `ghcr.io/geiserx/biblical-atlas-mcp:vX.Y.Z`;
   - pushes the same image to Docker Hub when the repository variable `DOCKERHUB_ENABLED` is `true`;
   - publishes to npm with trusted publishing when `NPM_PUBLISH_ENABLED` is `true`.

Semver tags only; the images get no `latest` tag.

### One-time setup

| Step | Needs | Until then |
| --- | --- | --- |
| GitHub release and binaries | Nothing | Works on the first tag |
| GHCR image | Nothing. After the first push, set the package to public once in its settings. | Works on the first tag |
| Docker Hub image and description | Secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`, variables `DOCKERHUB_NAMESPACE` and `DOCKERHUB_ENABLED=true` | Skipped; the release stays green |
| npm | A trusted publisher on npmjs.com for this repository and `release.yml`, then variable `NPM_PUBLISH_ENABLED=true`. If npm wants the package to exist first, publish the first version by hand once. | Skipped; the release stays green |
| Official MCP registry | `mcp-publisher`, run by hand after the npm version exists | Manual, below |

### Official MCP registry

After the npm package of the new version is live:

```bash
mcp-publisher login github
mcp-publisher publish
```

`server.json` lists the npm package only. The container starts in HTTP mode, so an OCI entry that claims stdio would be false.
