[![progress-banner](https://backend.codecrafters.io/progress/shell/528df814-d08e-4604-bcab-23a000d2bdeb)](https://app.codecrafters.io/users/johnny880319?r=2qF)

# codecrafters-shell-go

Modern Go baseline for the CodeCrafters "Build Your Own Shell" challenge.

This repository is intentionally set up with strict quality gates, reproducible tooling, and CI checks so you can focus on implementing shell features safely.

Contribution workflow and PR checklist: see `CONTRIBUTING.md`.

## 1) Project structure

```text
.
├── cmd/my_shell/main.go        # Application entrypoint only
├── internal/shell/repl.go      # Core shell logic (private to this module)
├── internal/shell/repl_test.go # Unit tests for shell logic
├── .golangci.yaml              # Strict lint config
├── .pre-commit-config.yaml     # Fast local quality gates
├── .github/workflows/go.yml    # CI checks on push/PR
├── Dockerfile                  # Multi-stage container build
└── Makefile                    # Common developer commands
```

Design note:
- `cmd/` is for executables.
- `internal/` is for app logic that should not be imported by other modules.
- `pkg/` is intentionally omitted until you truly need a public library API.

## 2) Strict linting

Linting is handled by `golangci-lint` with an explicit strict set in `.golangci.yaml`:
- correctness: `govet`, `staticcheck`, `errcheck`, `errorlint`, `ineffassign`, `unused`
- security: `gosec`
- maintainability/style: `revive`, `gocritic`, `gocognit`, `lll`, `misspell`, `nolintlint`

Why explicit list instead of `enable-all`:
- `enable-all` often breaks when new/deprecated linters are added in future releases.
- Explicit lists are still strict, but stable and maintainable in CI.

## 3) Auto-format on save

VS Code settings are provided in `.vscode/settings.json`:
- format on save is enabled for Go files
- imports are organized on save
- `gofumpt` mode is enabled in `gopls`

Command line format checks:
- `make fmt` to auto-format
- `make fmt-check` to fail if formatting is not clean

## 4) Environment management (Go equivalent)

Go projects usually do **not** use virtualenv-style tooling (like Python `uv`).
The standard, idiomatic approach is:
- `go.mod` + `go.sum` for dependency management
- `go` directive in `go.mod` to define expected language/toolchain baseline

This repo uses:
- `go 1.25.0`

## 5) CI/CD quality gates

GitHub Actions workflow (`.github/workflows/go.yml`) runs on push/PR to `main`/`master`:
1. `gofmt` check
2. `go vet`
3. `golangci-lint`
4. `go test -race -covermode=atomic`
5. `go build`
6. `docker build`

If any step fails, the workflow fails and merge should be blocked by branch protection.

## 6) pre-commit strategy

`pre-commit` runs fast checks before commit:
- whitespace/yaml/conflict checks
- `gofmt`
- `go vet`
- `golangci-lint`

About tests in pre-commit:
- Your understanding is correct in most teams.
- Keep `pre-commit` fast; run full tests in CI (or `pre-push` if needed).

## 7) Unit test baseline

A minimal testable shell unit is already in place:
- `internal/shell/repl.go`
- `internal/shell/repl_test.go`

This gives you a clean pattern to add tests stage-by-stage as shell behavior grows.

## 8) Containerization: when and why

Current Docker setup is multi-stage:
- stage 1: build binary
- stage 2: minimal runtime image

Benefits:
- same runtime behavior across machines/CI
- cleaner onboarding (no local Go setup needed to run built image)
- production-like packaging if you later deploy this shell service/tool

When useful for this challenge:
- validating build reproducibility in CI
- sharing a runnable artifact with others

## 9) Makefile automation

Useful targets:
- `make ci` (format + vet + lint + tests + build)
- `make lint`
- `make test-race`
- `make docker-build`
- `make docker-run`

## 10) Extra maturity items already included / recommended

Included:
- `.editorconfig`
- `.gitignore`
- `.dockerignore`

Recommended next:
1. Add branch protection on `main` so CI must pass before merge.
2. Add Dependabot for Go modules and GitHub Actions updates.
3. Add coverage upload/reporting once test volume increases.

## Quick start

```bash
make ci
./your_program.sh
```

## CodeCrafters runtime scripts

- Local run script: `your_program.sh`
- Remote compile script: `.codecrafters/compile.sh`
- Remote run script: `.codecrafters/run.sh`

Both local/remote compile scripts build `./cmd/my_shell`, so package structure changes are automatically included.
