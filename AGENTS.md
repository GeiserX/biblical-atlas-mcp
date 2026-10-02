# AGENTS.md

biblical-atlas-mcp is a Go MCP server for the biblical-atlas Bible atlas. It downloads the atlas's public data file (`https://biblical-atlas.geiser.cloud/data.json`), indexes it in memory and answers nine read-only tools. The design is in [docs/design/DESIGN.md](docs/design/DESIGN.md); read it before changing behaviour. User docs are in [docs/](docs/).

## Layout

- `cmd/server/main.go`: `run(ctx, args, stdin, stdout, stderr, getenv)` holds the whole program; `main` only calls it.
- `internal/config`: environment variables. A bad value exits 2 naming the variable.
- `internal/atlas`: `model.go` (tolerant decoding, format check), `store.go` (fetch, ETag, refresh, keep-the-old-copy), `cache.go` (disk cache), `index.go` (every index, search, suggestions), `years.go`, `text.go` (normalisation, scoring), `reference.go` (book table, reference parser). Nothing else reads the file or the network.
- `internal/tools`: one file per tool, `register.go` (the tool list, `Instructions`, annotations), `output.go` (ordered JSON objects, refs, dates, sources, items, paging), `args.go` (argument checks).
- `internal/atlas/testdata/`: the hand-written fixture and three variants (`jworg.json` is the fixture with its chapter sources on jw.org).
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
- Card links use the constant site base, never `BIBLICAL_ATLAS_DATA_URL`. Every URL in an answer is the site, jw.org (www or wol) or the coordinate source; `TestLinks` checks it.
- The fixture holds real ids and structure but only invented short texts. Copy no text from jw.org or from the atlas into this repository.
- Keep `package.json` and `server.json` on the same version, and the archive names in `postinstall.js` in step with `.goreleaser.yaml`; `manifest_test.go` checks both.
- Docker images are pinned by semver; there is no `latest` tag.
- Before trusting a new test, break the line it guards and watch it fail.
- Heavy work (race runs, repeated test runs, image builds) belongs in CI, not on a laptop.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->

## Where the tracker syncs

This repo is public, so its tracker syncs only to the private Dolt remote named by `sync.remote` in `.beads/config.yaml` (`giteaer/biblical-atlas-mcp-beads` on Gitea). The block above says sync uses "your git remote". Here that never means this GitHub repo. Don't add it as a Dolt remote and don't push `refs/dolt/*` to it.
