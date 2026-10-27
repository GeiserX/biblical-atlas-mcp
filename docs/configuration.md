# Configuration

All settings are environment variables and all of them are optional. A value that does not parse stops the server at start with exit code 2 and a message naming the variable.

## Variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `BIBLICAL_ATLAS_DATA_URL` | `https://biblical-atlas.geiser.cloud/data.json` | Where to download the data file. Must be `http` or `https`. |
| `BIBLICAL_ATLAS_DATA_FILE` | unset | Path to a local copy. When set, no network is used and the URL is ignored. |
| `BIBLICAL_ATLAS_CACHE_DIR` | `<user cache dir>/biblical-atlas-mcp` | Where the downloaded file is kept between runs. `off` disables the disk cache. |
| `BIBLICAL_ATLAS_REFRESH` | `1h` | How long a copy is served before the server asks again. A Go duration of at least `1m`, or `0` to never refresh. |
| `BIBLICAL_ATLAS_TIMEOUT` | `30s` | Limit for one download. A Go duration from `1s` to `5m`. |
| `TRANSPORT` | unset | `stdio` for stdio. Unset or `http` for streamable HTTP. |
| `LISTEN_ADDR` | `127.0.0.1:8080` | HTTP listen address. The container image sets `0.0.0.0:8080`. |
| `MCP_AUTH_TOKEN` | unset | HTTP requests to `/mcp` need `Authorization: Bearer <token>`. Required when `LISTEN_ADDR` is not a loopback address: the server refuses to start without it. No white space. |
| `MCP_ALLOWED_ORIGINS` | unset | Comma-separated browser origins (`https://app.example.org`) that may call `/mcp` from a web page. Unset, every cross-origin browser request gets 403. |

`<user cache dir>` is Go's `os.UserCacheDir()`: `~/Library/Caches` on macOS, `$XDG_CACHE_HOME` or `~/.cache` on Linux, `%LocalAppData%` on Windows. The container image sets `BIBLICAL_ATLAS_CACHE_DIR=/tmp/biblical-atlas-mcp`.

The npm wrapper always sets `TRANSPORT=stdio`. Every other variable passes through from the client's `env` block.

## Flags

| Flag | Effect |
| --- | --- |
| `--version`, `-v` | Prints the version, commit and build date, then exits 0. |
| `--help`, `-h` | Prints the variable table, then exits 0. |

Any other argument exits with code 2.

## HTTP endpoints

| Path | Auth | What it does |
| --- | --- | --- |
| `/mcp` | Bearer token when `MCP_AUTH_TOKEN` is set | Streamable HTTP, stateless: no session is kept between requests. `POST` only; a `GET` event stream is refused with 405, since the server sends no notifications. A body over 1 MB gets 413. A cross-origin browser request from an origin not in `MCP_ALLOWED_ORIGINS` gets 403. At most 32 requests are served at once; the next gets 503 with `Retry-After: 1`. |
| `/healthz` | None | Returns `ok`. It never touches the data. |

The data is public, but the tools still cost CPU, so the token is required as soon as the server listens beyond this machine (`0.0.0.0`, a LAN address, or the container's default). On `127.0.0.1` it stays optional; there the SDK also refuses requests whose `Host` header is not local, which stops DNS rebinding. The `Bearer` scheme is matched in any case, and the token is compared in constant time. Put a reverse proxy with TLS in front if the server is exposed beyond one machine.

The protocol versions are 2026-07-28 down to 2024-11-05 on both transports. A 2026-07-28 request over HTTP carries the `MCP-Protocol-Version`, `Mcp-Method` and, for `tools/call`, `Mcp-Name` headers; a missing or mismatched header gets 400.

## Client examples

A stdio client with a local copy of the data file and no network:

```json
{
  "mcpServers": {
    "biblical-atlas": {
      "command": "npx",
      "args": ["-y", "biblical-atlas-mcp"],
      "env": { "BIBLICAL_ATLAS_DATA_FILE": "/path/to/data.json" }
    }
  }
}
```

The container with a token:

```bash
docker run --rm -p 127.0.0.1:8080:8080 -e MCP_AUTH_TOKEN=change-me ghcr.io/geiserx/biblical-atlas-mcp:v0.1.4
```

An HTTP client config for it:

```json
{
  "mcpServers": {
    "biblical-atlas": {
      "type": "http",
      "url": "http://127.0.0.1:8080/mcp",
      "headers": { "Authorization": "Bearer change-me" }
    }
  }
}
```
