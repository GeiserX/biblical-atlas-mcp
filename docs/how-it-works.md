# How it works

The server ships no atlas data. The atlas publishes its whole dataset as one JSON file, and the server downloads that file, checks it, indexes it in memory and answers every tool call from it.

## The data file

The default source is <https://biblical-atlas.geiser.cloud/data.json>: about 7 MB of JSON (about 1 MB gzipped), served with an `ETag`. The server reads format `biblical-atlas/v0`, also written `biblical-earth/v0` by older atlas builds, and ignores any field it does not know, so the atlas can add fields without breaking it. The server refuses a file in any other format with an error that asks for a newer server, and keeps serving the copy it already has.

The server accepts a file when it is a JSON object with the known format, non-empty `personas`, `lugares` and `fuentes` objects, and non-empty `eventos` and `libros` arrays. The cap is 64 MB.

## Start-up

Start-up never blocks the MCP handshake. In the background the server:

1. loads the disk cache, when one exists for the same URL, and serves it at once;
2. downloads the file when there is no cache, or refreshes it in the background, while the cached copy keeps answering, when that copy is older than the refresh interval.

A tool call that arrives before any copy is loaded waits for the download, bounded by the download timeout. If the download fails, the call returns an error naming the cause and the URL, and the next call tries again (no sooner than 5 seconds later).

## Refresh

A copy is served for `BIBLICAL_ATLAS_REFRESH` (1 hour by default). After that, the next call is answered from the old copy at once while one background request asks the site again with `If-None-Match`:

- `304 Not Modified`: the copy stays and its check time moves forward.
- `200`: the new file is checked and indexed, then swapped in with one pointer store. Calls already running finish on the copy they started with.
- Anything else, including a file that fails the checks or cannot be read at all: the old copy keeps serving, `dataset_info` reports `stale: true` with the `refresh_error`, and the server asks again after another interval.

Every list answer carries `dataset`, the build date of the copy that answered, so a caller can tell when paging crossed a refresh.

## Disk cache

After each download the server writes `data.json` and `meta.json` (URL, ETag and fetch time) to `BIBLICAL_ATLAS_CACHE_DIR`, through a temporary file and a rename. The cache is used only for the same URL. If the directory cannot be written, the server logs one line and runs from memory.

## Offline use

With `BIBLICAL_ATLAS_DATA_FILE` set, the server reads that file and never touches the network or the disk cache. After each refresh interval it checks the file's size and modification time and reloads it when they change.

## Indexes

Each load builds, once: records by type and id; search texts; relations in both directions (the atlas stores a relation on one person only); the person graph for `find_connection`; events by person and by place; letters, journeys, stops, finds and seats by place and by person; every event passage parsed into verse ranges; the records citing each chapter; and what the events say about each person's birth and death. For the current file this takes well under a second.

## Years

The data file stores astronomical years, where 0 is 1 a.e.c. The server converts at the edge, so a caller only ever sees signed historical years: `-607` is 607 a.e.c. and there is no year 0.
