# Contributing

Thanks for contributing to **plumelog-cli**.

## Setup

- Go **1.22+** (see `go.mod`)
- Clone, then:

```bash
go test ./...
go build -o plumelog .
```

## Pull requests

1. Branch from `main` (for example `fix/...` or `chore/...`).
2. Keep changes focused; include tests for behavior changes.
3. Run `go test ./...` locally before opening a PR.
4. Do **not** commit secrets: passwords, cookies, `.env`, or real server URLs with credentials.
5. Prefer Chinese or English commit messages that explain *why*.

## Code style

- Follow existing `cobra` command layout under `cmd/` and packages under `internal/`.
- High-risk writes must keep `--yes` / dry-run / admin-password redaction behavior.
- Update `README.md` / `README.zh-CN.md` and `skills/` when user-facing behavior changes.

## Security

See [SECURITY.md](SECURITY.md). Never paste credentials into issues, PRs, or logs.
