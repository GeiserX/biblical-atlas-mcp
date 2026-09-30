# Contributing

Thanks for helping. Bug reports, fixes and new tool ideas are welcome.

## Before you start

- For a bug, open an issue with the tool call you made, what came back and what you expected.
- For a new tool or a change in what a tool returns, open an issue first so we can agree on the shape.
- Security problems go through [SECURITY.md](SECURITY.md), never a public issue.

## Making a change

1. Fork the repository and create a branch from `main`.
2. Build and test:

   ```bash
   go build ./...
   go vet ./...
   test -z "$(gofmt -l .)"
   go test ./...
   ```

3. Add a test for what you changed. The tool tests live in [`internal/tools/tools_test.go`](internal/tools/tools_test.go) and run against the fixture in [`internal/atlas/testdata/`](internal/atlas/testdata/).
4. Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages and the pull request title (`feat:`, `fix:`, `docs:`...).
5. Open a pull request against `main`. CI must pass before it is merged.

## Data

This server never ships or edits atlas data. A wrong fact belongs in [biblical-atlas](https://github.com/GeiserX/biblical-atlas), the atlas itself. The test fixture uses real ids with invented texts; please keep it that way.

## License

By contributing you agree that your contribution is licensed under the [MIT License](LICENSE).
