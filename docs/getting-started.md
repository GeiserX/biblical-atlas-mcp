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

The npm package is a small wrapper. On install it downloads the release binary for your platform from the GitHub release of the same version, checks it against `checksums.txt` and starts it in stdio mode.

```bash
claude mcp add biblical-atlas -- npx -y biblical-atlas-mcp
```

For Claude Desktop, Cursor and other clients that start stdio servers, add this to the client's `mcpServers` config:

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

The image starts in HTTP mode on port 8080. Tags are release versions only; there is no `latest` tag.

```bash
docker run --rm -p 127.0.0.1:8080:8080 ghcr.io/geiserx/biblical-atlas-mcp:v0.1.1
curl http://127.0.0.1:8080/healthz        # ok
```

Point an HTTP client at `http://127.0.0.1:8080/mcp`. If you publish the port beyond your own machine, set `MCP_AUTH_TOKEN` and put a reverse proxy with TLS in front; see [Configuration](configuration.md).

### From source

```bash
git clone https://github.com/GeiserX/biblical-atlas-mcp.git
cd biblical-atlas-mcp
go build -o biblical-atlas-mcp ./cmd/server
```

## A first call

Ask your agent a question the atlas can answer, for example "who was alive in 607 a.e.c.?" or "what does the atlas have on Hechos 16?". The agent calls `people_in_year` with `year: -607`, or `lookup_passage` with `reference: "Hch 16"`. The answer is JSON with Spanish names, a card link on the atlas site for each record and the sources behind it.

To check that the data loaded, ask for `dataset_info`. It reports the build date of the data file, the record counts and which Bible books the atlas has read verse by verse.
