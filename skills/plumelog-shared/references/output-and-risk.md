# Output contract & high-risk approval (plumelog)

## Success vs failure

- Success: stdout JSON with `ok: true`, payload under **`result`** (not `data`), exit 0.
- Failure: usually stdout JSON with `ok: false` and **`error` as a string**; exit ≠ 0 (currently **1** for gates, not 10).
- Agents must branch on `ok` or exit code — never on upstream PlumeLog `code == 0`.

## Dry-run

```bash
plumelog admin queue clear --dry-run
plumelog api POST /deleteQueue --dry-run
```

Returns `ok: true`, `dry_run: true`, redacted URL/headers, optional `would_require`. Does **not** send, even with `--yes`.

## High-risk gate (current)

Missing `--yes` / `--admin-password`:

```json
{"ok":false,"risk":"high-risk-write","error":"refusing high-risk-write without --yes and --admin-password"}
```

Exit **1**. No `error.subtype`, no `confirmation_required`, no structured `hint`/`action` yet.

**Do not** treat this like yunxiao exit 10 until CLI is changed. Stop, ask the user, then retry with `--yes` (and admin password when required) only after explicit consent.

## Dual gate

| Command | Needs `--yes` | Needs `--admin-password` (sent as `adminPassWord`) |
|---------|---------------|-----------------------------------------------------|
| `admin queue clear` | yes | yes |
| `admin index delete` | yes | yes |
| `admin index reset` | yes | no (not sent) |
| `admin delete-by-query` | yes | no (not sent) |
| `alarm delete` | yes | no |
| `app extend del` | yes | no |
| `api` → deleteQueue/deleteIndex | yes | yes |
| `api` → resetIndex/deleteByQuery/deleteWarningRule/delExtendfield | yes | no |

## Secrets

Never print login password, admin password, or cookie values. Prefer env vars; `config set` refuses password keys; dry-run redacts Cookie and `adminPassWord`.