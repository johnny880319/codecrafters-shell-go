[![progress-banner](https://backend.codecrafters.io/progress/shell/528df814-d08e-4604-bcab-23a000d2bdeb)](https://app.codecrafters.io/users/johnny880319?r=2qF)

# codecrafters-shell-go

Go implementation for the CodeCrafters "Build Your Own Shell" challenge.

This repository now contains an actively implemented shell, not just a starter skeleton.

Configuration note:
- This tooling stack is AI-suggested.
- The repository owner is still learning these tools and may refine settings over time.

## Implemented Features

- REPL loop with interactive input (`readline`)
- Builtins: `exit`, `echo`, `type`, `pwd`, `cd`, `history`
- External command execution via `PATH` lookup
- Pipelines (`|`)
- Redirection: `>`, `1>`, `2>`, `>>`, `1>>`, `2>>`
- Quote and escape handling:
  - single quotes
  - double quotes
  - backslash escaping
- Tab completion:
  - command completion (builtins + executables)
  - filesystem path completion
  - double-tab list behavior
- Shell history in memory and file-backed history through `HISTFILE`
  (`history -r`, `history -w`, `history -a`)

## Project Layout

```text
.
├── cmd/my_shell/main.go         # Entrypoint
├── internal/shell/repl.go       # REPL and command execution
├── internal/shell/builtins.go   # Builtin command implementations
├── internal/shell/helper.go     # Parsing, path lookup, redirection helpers
├── internal/shell/completer.go  # Tab completion logic
├── internal/shell/repl_test.go  # Unit tests
├── .github/workflows/go.yml     # CI checks
├── .golangci.yaml               # Lint config (v2 schema)
├── .golangci-version            # Pinned golangci-lint version
├── .pre-commit-config.yaml      # Pre-commit checks
├── Makefile                     # Local automation
├── Dockerfile                   # Multi-stage container build
└── .vscode/launch.json          # VS Code debug target (cmd/my_shell)
```

## Prerequisites

- Go `1.25.x`
- `golangci-lint` `v2.x` (pinned in `.golangci-version`)
- `pre-commit` (optional but recommended)
- Docker or Podman (optional)

## Run Locally

```bash
./your_program.sh
```

or

```bash
make run
```

## Debug in VS Code

Use `Debug my_shell` in `.vscode/launch.json`.
It points to `cmd/my_shell`, so breakpoints should work without the
`no Go files in <repo-root>` build error.

## Testing and Quality

Common commands:

- `make fmt`
- `make fmt-check`
- `make vet`
- `make lint`
- `make test`
- `make test-race`
- `make ci` (runs the full local CI-equivalent pipeline)

Pre-commit:

```bash
PRE_COMMIT_HOME=$PWD/.cache/pre-commit pre-commit install --install-hooks
make pre-commit
```

## CI

GitHub Actions (`.github/workflows/go.yml`) runs on `push`/`pull_request` to
`main`/`master`:

1. `gofmt` check
2. `go vet`
3. `golangci-lint`
4. `go test -race -covermode=atomic`
5. `go build`
6. `docker build`

## Container

Build and run:

```bash
make docker-build
make docker-run
```

The Dockerfile uses a multi-stage build:

- builder: `golang:1.25-alpine`
- runtime: `alpine:3.23`

## Notes

- `CONTRIBUTING.md` contains commit/PR workflow and checklist.
- This is still an in-progress shell implementation; missing POSIX features
  are expected as challenge stages advance.
