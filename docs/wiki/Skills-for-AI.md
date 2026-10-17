# Skills for AI agents

Start with the bundled skill **plumelog-shared**, then load a domain skill (`plumelog-query`, `plumelog-trace`, `plumelog-app`, `plumelog-alarm`, `plumelog-admin`, `plumelog-api`).

## Install / refresh

```bash
plumelog skills list
plumelog skills list --check    # equal | outdated | ahead | invalid (+ missing/unknown)
plumelog skills read plumelog-shared
plumelog skills install
plumelog skills update          # force-refresh to match this binary
```

Skills are **embedded** in the binary when no on-disk `skills/` tree is found.

## Output contract (do not invent)

- Success: **stdout** `{ "ok": true, "result": ... }`, exit 0
- Failure: **stderr** `{ "ok": false, "error": { "type", "message", ... } }` — `error` is an **object**
- High-risk without `--yes`: **exit 10**, `error.subtype=confirmation_required`
- Never put passwords, `adminPassWord`, or cookie values in replies or logs
- Prefer `--dry-run` before writes; never silently add `--yes`

## Risk

| Risk | Gate |
|------|------|
| read | run |
| write | run (dry-run recommended) |
| high-risk-write | `--yes` required; some admin paths also need `--admin-password` |

In-repo details: `skills/plumelog-shared/SKILL.md` and `docs/architecture.md`.
