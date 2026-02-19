# Contributing Guide

Thanks for contributing to `codecrafters-shell-go`.

This repository is configured with strict quality checks to keep changes safe and easy to review.

## Prerequisites

- Go 1.25.x
- `golangci-lint` (same major version used in CI)
- `pre-commit` (recommended)
- Docker or Podman (optional, for container checks)

## First-Time Setup

```bash
PRE_COMMIT_HOME=$PWD/.cache/pre-commit pre-commit install --install-hooks
make help
```

## Local Workflow

1. Implement your change in small commits.
2. Run quick checks while iterating:
   - `make fmt`
   - `make test`
3. Run full checks before push:
   - `make ci`
   - `make pre-commit`

## Commit Message Style

Use concise, imperative messages. Conventional prefixes are recommended:

- `feat:` for new behavior
- `fix:` for bug fixes
- `chore:` for tooling/config/docs updates
- `test:` for test-only changes

Examples:

- `feat: implement echo builtin`
- `fix: handle EOF in repl scanner`
- `chore: tighten lint and ci checks`

## Pull Request Checklist

Before opening a PR, verify:

- [ ] `make ci` passes locally
- [ ] New behavior has unit tests when applicable
- [ ] No unrelated files are changed
- [ ] Docs are updated when behavior/tooling changes

## pre-commit Policy

`pre-commit` is intentionally fast (format + vet + lint).  
Long-running test suites stay in CI by default.

## Branch Protection

After CI is connected on GitHub, enable branch protection on `main`:

- Require pull request before merging
- Require status checks to pass (select `Go CI / quality`)
- Optional: require up-to-date branch before merge
