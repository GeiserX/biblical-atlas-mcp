# Getting started

biblical-atlas-mcp runs on your machine and reads the atlas's public data file from <https://biblical-atlas.geiser.cloud/data.json>. It needs a network connection the first time it starts; after that it can run from its disk cache or from a local copy of the file.

## Pick an install path

| Path | Needs | Transport |
| --- | --- | --- |
| npm (`npx biblical-atlas-mcp`) | Node.js 18 or newer | stdio |
| Release binary | Nothing | stdio or HTTP |
| Container image | Docker or another OCI runtime | HTTP |
| From source | Go 1.27 | stdio or HTTP |

### npm

The npm package is a small wrapper. It downloads the release binary for your platform from the GitHub release of the same version, checks it against the `checksums.txt` that ships inside the npm package (so the sums come with npm's provenance, not from the release), and starts it in stdio mode. The download runs on install, or on first start when the package manager skips install scripts, as npm 12, pnpm and Bun do by default.

```bash
claude mcp add biblical-atlas -- npx -y biblical-atlas-mcp
```

For Claude Desktop, Cursor and other clients that start stdio servers, add this to the client's `mcpServers` config (VS Code calls the key `servers`; the README has a block for it and for Codex):

```json
{
  "mcpServers": {
    "biblical-atlas": { "command": "npx", "args": ["-y", "biblical-atlas-mcp"] }
  }
}
```

### Release binary

Download the archive for your system from the [releases page](https://github.com/GeiserX/biblical-atlas-mcp/releases), check it against `checksums.txt`, and unpack `biblical-atlas-mcp` somewhere on your `PATH`. Builds exist for Linux, macOS and Windows on amd64 and arm64.

```bash
biblical-atlas-mcp --version
TRANSPORT=stdio biblical-atlas-mcp        # for a client that starts the server itself
biblical-atlas-mcp                        # streamable HTTP on http://127.0.0.1:8080/mcp
```

A stdio client config that uses the binary directly:

```json
{
  "mcpServers": {
    "biblical-atlas": { "command": "/usr/local/bin/biblical-atlas-mcp", "env": { "TRANSPORT": "stdio" } }
  }
}
```

### Container image

The image starts in HTTP mode on port 8080, listening on every interface inside the container, so it needs a token: without `MCP_AUTH_TOKEN` it exits at start with a message saying so. Tags are release versions only; there is no `latest` tag.

```bash
docker run --rm -p 127.0.0.1:8080:8080 -e MCP_AUTH_TOKEN=change-me ghcr.io/geiserx/biblical-atlas-mcp:v0.2.0
curl http://127.0.0.1:8080/healthz        # ok
```

Point an HTTP client at `http://127.0.0.1:8080/mcp` with the header `Authorization: Bearer change-me`; [Configuration](configuration.md) has a client config. Put a reverse proxy with TLS in front if you publish the port beyond your own machine.

### From source

```bash
git clone https://github.com/GeiserX/biblical-atlas-mcp.git
cd biblical-atlas-mcp
go build -o biblical-atlas-mcp ./cmd/server
```

## A first call

Ask your agent a question the atlas can answer, for example "who was alive in 607 a.e.c.?" or "what does the atlas have on Hechos 16?". The agent calls `people_in_year` with `year: -607`, or `lookup_passage` with `reference: "Hch 16"`. The answer is JSON with Spanish names, a card link on the atlas site for each record and the sources behind it.

To check that the data loaded, ask for `dataset_info`. It reports the build date of the data file, the record counts and which Bible books the atlas has read verse by verse.
