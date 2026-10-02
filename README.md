<p align="center">
  <img src="https://raw.githubusercontent.com/GeiserX/biblical-atlas-mcp/main/docs/images/banner.svg" alt="biblical-atlas-mcp" width="100%">
</p>

<h1 align="center">biblical-atlas-mcp</h1>

<p align="center">
  <a href="https://github.com/GeiserX/biblical-atlas-mcp/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/GeiserX/biblical-atlas-mcp/ci.yml?style=flat-square&label=CI" alt="CI"></a>
  <a href="https://github.com/GeiserX/biblical-atlas-mcp/blob/main/LICENSE"><img src="https://img.shields.io/github/license/GeiserX/biblical-atlas-mcp?style=flat-square" alt="License"></a>
</p>

biblical-atlas-mcp is an MCP server that gives AI agents the [biblical-atlas](https://github.com/GeiserX/biblical-atlas) Bible atlas as nine read-only tools: people, places, events, reigns, letters, journeys and Bible passages. Answers carry the atlas id, a link to the record's card on [the atlas site](https://biblical-atlas.geiser.cloud/) where the record has one, and links to its sources: a list answer carries the first one and the total, and `get_record` returns them all. The atlas is in Spanish, and the server returns its names and summaries as they are.

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

With Node.js 18 or newer, add the server to Claude Code with `claude mcp add biblical-atlas -- npx -y biblical-atlas-mcp`, or to any client that starts stdio servers:

```json
{
  "mcpServers": {
    "biblical-atlas": { "command": "npx", "args": ["-y", "biblical-atlas-mcp"] }
  }
}
```

Then ask "who was alive in 607 a.e.c.?"; the container image for HTTP clients, release binaries and every setting are in [Getting started](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/getting-started.md).

## Documentation

- [Getting started](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/getting-started.md): every install path and a first call
- [Configuration](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/configuration.md): every variable, flag and client config
- [Usage](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/usage.md): the nine tools, their parameters and what they return
- [How it works](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/how-it-works.md): download, cache, refresh and offline use
- [Development](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/docs/development.md): build, test and release

## Related projects

The data belongs to [biblical-atlas](https://github.com/GeiserX/biblical-atlas), the atlas this server reads. It stays on the atlas site with its sources; this server only fetches and indexes it. Other MCP servers in the family: [telegram-archive-mcp](https://github.com/GeiserX/telegram-archive-mcp).

## License

[MIT](https://github.com/GeiserX/biblical-atlas-mcp/blob/main/LICENSE)
