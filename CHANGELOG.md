# Changelog

All notable changes to this project are listed here, newest first. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
