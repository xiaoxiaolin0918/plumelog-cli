# Output contract & high-risk approval

## Success vs failure

- Success: **stdout** JSON with `ok: true`, payload under **`result`** (not `data`), exit 0.
- Ordinary failure: **stderr** JSON with `ok: false` and **`error` as an object** (`type` / `message` / optional `subtype`/`hint`/`risk`/`action`); exit 1.
- Confirmation gate: **stderr** JSON with `ok: false` and **`error` as an object**; exit **10** (not 100).
- Agents must branch on `ok` / exit code / `error.subtype` — never on upstream PlumeLog `code == 0`. Prefer reading stderr on non-zero exit.

## Dry-run

```bash
plumelog admin queue clear --dry-run --admin-password "$PLUMELOG_ADMIN_PASSWORD"
```

Returns `ok: true`, `dry_run: true`, redacted URL/headers, optional `would_require`. Does **not** send, even with `--yes`.

## Exit 10 (`confirmation_required`)

Missing `--yes` and/or required `--admin-password` (real run, not dry-run):

```json
{
  "ok": false,
  "risk": "high-risk-write",
  "hint": "add --yes to confirm",
  "error": {
    "type": "confirmation",
    "subtype": "confirmation_required",
    "message": "refusing high-risk-write without --yes",
    "hint": "add --yes to confirm",
    "risk": "high-risk-write",
    "action": "admin index reset"
  }
}
```

Aligned with yunxiao: stop, ask the user, then retry with `--yes` (and `--admin-password` when the command needs it) only after explicit consent. This is **not** a network/auth error.

Dual admin gate: `admin queue clear` / `admin index delete` (and matching `api` paths) also require `--admin-password`; the hint names both flags when both are missing.
