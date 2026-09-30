# Changelog

All notable changes to this project are listed here, newest first. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0]

### Added

- Nine read-only tools over the biblical-atlas atlas: `search`, `get_record`, `list_records`, `list_events`, `people_in_year`, `lookup_passage`, `find_connection`, `places_near` and `dataset_info`.
- Download of the atlas's public data file with ETag revalidation, an hourly refresh, a disk cache and a local-file mode for offline use.
- stdio and streamable HTTP transports, with an optional bearer token and `/healthz`.
- Release binaries for Linux, macOS and Windows, an npm wrapper, and a multi-arch container image on GHCR.
