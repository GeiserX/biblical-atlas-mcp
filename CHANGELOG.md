# Changelog

All notable changes to this project are listed here, newest first. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- `get_record` for a place pages its journey stops 10 at a time (`journey_stops_offset`, with `journey_stops_total` and `next_journey_stops_offset` in the answer). Jerusalem's 108 stops made its answer 92 KB, which Claude Code refuses with its default 25,000-token limit; it is 57 KB now.
- `lookup_passage` lists at most 25 people, places and citing records instead of 30, so a whole book with `limit` 50 stays under the 70 KB answer cap (1 Chronicles was 70.5 KB).

## [0.2.0] - 2026-10-27

### Changed

- **Breaking for HTTP deployments:** the server refuses to start in HTTP mode on a non-loopback address (`0.0.0.0`, a LAN address, the container's default) without `MCP_AUTH_TOKEN`. Add `-e MCP_AUTH_TOKEN=...` to `docker run`. On `127.0.0.1` the token stays optional.
- Speaks MCP 2026-07-28 (mcp-go v1.4.0) alongside every earlier revision down to 2024-11-05, on stdio and HTTP: `server/discover`, the `MCP-Protocol-Version`, `Mcp-Method` and `Mcp-Name` headers, and cache hints on `tools/list` (one hour, public). Older clients still use `initialize`.
- Every tool declares an output schema and returns `structuredContent` next to the same JSON as text. Inputs are checked against a strict schema before the tool runs: an unknown parameter or an out-of-range value is a tool error naming the field. A `null` value for an optional parameter is refused the same way; leave the parameter out instead.
- `dataset_info` names the data source without credentials, query or fragment, and a local file by its name only. Load and refresh errors are scrubbed the same way.

### Added

- `get_records`: up to 20 records of any type in one call, with per-record errors and the records left out once the answer reaches about 56 KB.
- English Bible book names and abbreviations (`Acts`, `2 Kings`, `Rev`) in `lookup_passage`, `list_events` and `search`. Answers still name the book in Spanish.
- `search` puts a named Bible book first (`matched: book`), adds `passage` when the query reads as a reference, and returns `did_you_mean` when nothing matches.
- `dataset_info` lists `filters`: the place kinds, offices, period kinds, event kinds and roles the loaded atlas uses.
- Server title, description, website and icons in `initialize`, `server/discover` and `server.json`.
- `MCP_ALLOWED_ORIGINS` lets named browser origins call `/mcp`, with the CORS answers a browser needs; other cross-origin browser requests get 403.

### Security

- HTTP: at most 32 requests at once (the next gets 503 with `Retry-After`), the token compared in constant time, the `Bearer` scheme matched in any case, cross-origin browser requests refused.
- The npm wrapper checks the download against the `checksums.txt` shipped inside the npm package instead of one fetched from the release, follows HTTPS redirects only and stops a download over 64 MB. It installs on first start when the package manager skipped install scripts (npm 12, pnpm, Bun).
- Every GitHub Action is pinned to a commit, the container base images are pinned by digest, and CI adds govulncheck and CodeQL.

## [0.1.4]

### Security

- Built with go1.27.3, which rejects certificates issued through the eight compromised eMudhra (emSign) intermediates. Earlier Linux binaries and images trusted them, so a forged certificate for the data host would have been accepted.

## [0.1.3]

### Added

- Places carry their parts: the gates, towers, pools and other named sites inside a place or beside it, which the atlas now lists apart from the place's other names. `search` still finds Jerusalén by «Puerta de las Ovejas», now with `matched: part` and the part named («Puerta de las Ovejas, en Jerusalén»); a part with its own place record finds that place. `get_record` shows a place's `parts` apart from `other_names`, and `part_of` on a place another one lists as a part.
- Each release publishes `server.json` to the official MCP registry from CI, once npm serves the new version.

## [0.1.2]

### Changed

- The macOS binaries are signed with a Developer ID certificate and notarized by Apple, so Gatekeeper runs a downloaded binary without a warning.

## [0.1.1]

### Fixed

- Bible chapters are read from source links on jw.org as well as wol.jw.org. The atlas is moving its sources to jw.org, and with only the wol.jw.org shape known, `cited_by` and every chapter found through sources came back empty without an error.
- Passage links (`read_url`, event `passages`) open the jw.org study Bible at the cited verse or span, the way the site links them, instead of the whole chapter on wol.jw.org.

### Added

- `dataset_info` reports `chapter_sources` and a `warnings` entry when the file has sources but none of them reads as a Bible chapter. The load log carries the same warning.

## [0.1.0]

### Added

- Nine read-only tools over the biblical-atlas atlas: `search`, `get_record`, `list_records`, `list_events`, `people_in_year`, `lookup_passage`, `find_connection`, `places_near` and `dataset_info`.
- Download of the atlas's public data file with ETag revalidation, an hourly refresh, a disk cache and a local-file mode for offline use.
- stdio and streamable HTTP transports, with an optional bearer token and `/healthz`.
- Release binaries for Linux, macOS and Windows, an npm wrapper, and a multi-arch container image on GHCR.
