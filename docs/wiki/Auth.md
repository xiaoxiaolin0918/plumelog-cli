# Auth

PlumeLog authenticates with a **servlet session cookie** (commonly `JSESSIONID`), identical to the browser UI. This CLI does **not** use OAuth or bearer tokens.

## Login

```bash
./plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
./plumelog auth login --web          # 127.0.0.1 one-shot form (alias: --browser)
./plumelog auth login --browser --force   # replace existing jar session
./plumelog auth status
./plumelog auth status --probe
./plumelog whoami
./plumelog auth logout
```

### `--web` / `--browser`

- Listens on **127.0.0.1** only (never `0.0.0.0`)
- Opens the system browser to a tokenized URL; submit username/password once
- Reuses normal `POST /login` and writes the cookie jar
- ~120s timeout; listener closes on success or timeout
- Passwords are not printed, logged, or written to disk
- If a session already exists in the jar, pass `--force`

### Status output

`auth status` / `whoami` print **metadata only** (`has_cookie`, `cookie_path`, `base_url`, …) — never the cookie value.

### Opt-in auto re-login

`PLUMELOG_AUTO_LOGIN=1` or `--auto-login`: on upstream 401, re-login once from env and replay the request.
