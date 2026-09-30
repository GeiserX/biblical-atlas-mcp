# AGENTS.md

biblical-atlas-mcp is a Go MCP server for the biblical-atlas Bible atlas. It downloads the atlas's public data file (`https://biblical-atlas.geiser.cloud/data.json`), indexes it in memory and answers nine read-only tools. The design is in [docs/design/DESIGN.md](docs/design/DESIGN.md); read it before changing behaviour. User docs are in [docs/](docs/).

## Layout

- `cmd/server/main.go`: `run(ctx, args, stdin, stdout, stderr, getenv)` holds the whole program; `main` only calls it.
- `internal/config`: environment variables. A bad value exits 2 naming the variable.
- `internal/atlas`: `model.go` (tolerant decoding, format check), `store.go` (fetch, ETag, refresh, keep-the-old-copy), `cache.go` (disk cache), `index.go` (every index, search, suggestions), `years.go`, `text.go` (normalisation, scoring), `reference.go` (book table, reference parser). Nothing else reads the file or the network.
- `internal/tools`: one file per tool, `register.go` (the tool list, `Instructions`, annotations), `output.go` (ordered JSON objects, refs, dates, sources, items, paging), `args.go` (argument checks).
- `internal/atlas/testdata/`: the hand-written fixture and two variants.
- `package.json`, `run.js`, `postinstall.js`: the npm wrapper. `server.json`, `glama.json`: registry files.

## Commands

```bash
go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./...
node --test postinstall.test.js                               # npm wrapper checks
BIBLICAL_ATLAS_LIVE=1 go test ./internal/... -run TestLive    # real data file, never in CI
```

## Rules

- Output keys are English and mapped by the server; values are the atlas's Spanish text, verbatim. Never translate or rewrite a value, and never pass an unmapped field through.
- Years are signed historical years in every input and output (`-607` = 607 a.e.c., no year 0). Convert only with `ToHistorical` and `ToAstronomical`.
- Every reference carries a `type`; ids collide across types.
- Handlers return `mcp.NewToolResultError(msg), nil` for bad input, naming the parameter, the value and what is accepted. Nothing found is a normal answer with `total: 0`.
- Every tool keeps the four read-only annotations; `TestAnnotations` checks them. A new tool needs rows in the case table of [`internal/tools/tools_test.go`](internal/tools/tools_test.go), or `TestEveryToolHasACase` fails.
- Card links use the constant site base, never `BIBLICAL_ATLAS_DATA_URL`. Every URL in an answer is the site, wol.jw.org or the coordinate source; `TestLinks` checks it.
- The fixture holds real ids and structure but only invented short texts. Copy no text from jw.org or from the atlas into this repository.
- Keep `package.json` and `server.json` on the same version, and the archive names in `postinstall.js` in step with `.goreleaser.yaml`; `manifest_test.go` checks both.
- Docker images are pinned by semver; there is no `latest` tag.
- Before trusting a new test, break the line it guards and watch it fail.
- Heavy work (race runs, repeated test runs, image builds) belongs in CI, not on a laptop.
