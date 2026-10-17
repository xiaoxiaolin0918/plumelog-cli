# Architecture

Canonical in-repo document (this page is a short index):

- https://github.com/xiaoxiaolin0918/plumelog-cli/blob/main/docs/architecture.md
- Local path: [docs/architecture.md](../architecture.md)

## Snapshot (v0.1.1)

- **Transport auth:** Session Cookie via `POST /login` (optional `auth login --web` on 127.0.0.1)
- **Envelope:** success on stdout `{ok,result}`; failures on stderr with object `error`
- **Gates:** Risk labels; `--dry-run` / `--yes` / `--admin-password`; missing confirm → **exit 10** + `confirmation_required`
- **Skills:** embed + `list|read|path|install|update` with semver `--check`
- **Hosting:** PUBLIC GitHub only (no Origin); docs live in-repo under [`docs/wiki/`](README.md) (not GitHub Wiki)
- **Query:** `--index` without a time window auto-aligns `dtTime` to the index date (search + dsl)

See also [Auth](Auth.md), [Skills-for-AI](Skills-for-AI.md), [Getting-Started](Getting-Started.md).
