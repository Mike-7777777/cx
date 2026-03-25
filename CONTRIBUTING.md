# Contributing to cx

Thanks for your interest in contributing!

## Development Setup

```bash
git clone https://github.com/Mike-7777777/cx.git
cd cx
make build    # build binary
make test     # run tests with race detector
make lint     # run golangci-lint
```

Requires Go 1.24+ and [golangci-lint](https://golangci-lint.run/welcome/install/).

## Making Changes

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Run `make lint test` to ensure everything passes
4. Commit with [Conventional Commits](https://www.conventionalcommits.org/) format:
   - `feat: add new command`
   - `fix: resolve path issue on Windows`
   - `docs: update README`
5. Open a pull request

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `internal/` for non-exported packages
- Keep dependencies minimal — prefer the standard library
- Table-driven tests with `t.Run()` subtests

## Reporting Issues

Open a [GitHub Issue](https://github.com/Mike-7777777/cx/issues) with:
- cx version (`cx version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
