<p align="center">
  <img src="https://raw.githubusercontent.com/GeiserX/biblical-atlas-mcp/main/docs/images/banner.svg" alt="biblical-atlas-mcp" width="100%">
</p>

<h1 align="center">biblical-atlas-mcp</h1>

<p align="center">
  <a href="https://github.com/GeiserX/biblical-atlas-mcp/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/GeiserX/biblical-atlas-mcp/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/GeiserX/biblical-atlas-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/biblical-atlas-mcp?style=flat-square" alt="License"></a>
</p>

biblical-atlas-mcp is an MCP server that gives AI agents the [biblical-atlas](https://github.com/GeiserX/biblical-atlas) Bible atlas as ten read-only tools: people, places, events, reigns, letters, journeys and Bible passages. Answers carry the atlas id, a link to the record's card on [the atlas site](https://biblical-atlas.geiser.cloud/) where the record has one, and links to its sources: a list answer carries the first one and the total, and `get_record` returns them all. The atlas is in Spanish, and the server returns its names and summaries as they are.

## Features

- Search every record by its Spanish name; accents and case are ignored ("jerusalen" finds Jerusalén).
- Read one record in full: family and relations in both directions, offices, coordinates and candidate sites, what happened at a place.
- List events by year or range, person, place, role, kind or Bible book, in date or story order.
- Ask who was alive, ruling or active in a year, with the evidence for each person.
- Look up a Bible reference ("Hch 16:1", "Hch 16", "Rut") and get the events, people, places and records that cite it.
- Find the shortest chain of family ties, succession and companionship between two people.
- Find places within a radius of a place or a coordinate.
- Years are signed in both directions: -607 means 607 a.e.c.
- No database and no atlas data in the package: the server downloads the atlas's public data file, caches it and refreshes it hourly with a conditional request.

## Quick start

Needs Node.js 18 or newer. The npm package downloads the release binary for your platform on first run and checks it against the checksums published with the package.

| Client | Setup |
| --- | --- |
| Claude Code | `claude mcp add biblical-atlas -- npx -y biblical-atlas-mcp` |
| Claude Desktop | `claude_desktop_config.json`: the `mcpServers` block below |
| Cursor | `~/.cursor/mcp.json` or `.cursor/mcp.json`: the `mcpServers` block below |
| VS Code | `.vscode/mcp.json`: the `servers` block below |
| Codex | `~/.codex/config.toml`: the TOML block below |

```json
{
  "mcpServers": {
    "biblical-atlas": { "command": "npx", "args": ["-y", "biblical-atlas-mcp"] }
  }
}
```

```json
{
  "servers": {
    "biblical-atlas": { "type": "stdio", "command": "npx", "args": ["-y", "biblical-atlas-mcp"] }
  }
}
```

```toml
[mcp_servers.biblical-atlas]
command = "npx"
args = ["-y", "biblical-atlas-mcp"]
```

Then ask "who was alive in 607 a.e.c.?". The container image for HTTP clients, release binaries and every setting are in [Getting started](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/getting-started.md).

## Tools

All ten tools are read only: they never write, delete or call anything but the atlas's own data file, so there is no read-only mode to switch on. Each declares an output schema and returns its answer both as `structuredContent` and as JSON text.

| Tool | What it answers |
| --- | --- |
| `search` | Records by name, Spanish ("Pablo") or a Bible book in Spanish or English ("Acts"); `did_you_mean` on a miss |
| `get_record` | One record in full, with its sources |
| `get_records` | Up to 20 records in one call |
| `list_records` | Periods, letters, journeys, finds, tours, books, months; places of a kind; holders of an office |
| `list_events` | Events by year, person, place, role, kind or Bible book |
| `people_in_year` | Who was alive or active in a year, with the evidence |
| `lookup_passage` | What the atlas has on a reference ("Hch 16:1" or "Acts 16:1") |
| `find_connection` | The shortest chain of ties between two people |
| `places_near` | Places within a radius of a place or a point |
| `dataset_info` | Freshness, counts, and the values the `kind` and `role` filters accept |

## Protocol

The server speaks MCP 2026-07-28 and every earlier revision down to 2024-11-05, on stdio and on streamable HTTP. Clients that still start with `initialize` (Claude Code, Claude Desktop, Cursor and VS Code today) work as before; clients on 2026-07-28 get `server/discover`, stateless requests and cacheable tool lists from the same binary.

## Documentation

- [Getting started](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/getting-started.md): every install path and a first call
- [Configuration](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/configuration.md): every variable, flag and client config
- [Usage](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/usage.md): the ten tools, their parameters and what they return
- [How it works](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/how-it-works.md): download, cache, refresh and offline use
- [Development](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/development.md): build, test and release

## Related projects

The data belongs to [biblical-atlas](https://github.com/GeiserX/biblical-atlas), the atlas this server reads. It stays on the atlas site with its sources; this server only fetches and indexes it. Other MCP servers in the family: [telegram-archive-mcp](https://github.com/GeiserX/telegram-archive-mcp).

## License

[MIT](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/LICENSE)
