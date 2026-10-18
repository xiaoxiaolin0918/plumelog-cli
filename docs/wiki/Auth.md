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
- Passwords are not printed or logged. Optional **记住并自动登录** encrypts the password into `login.cred` beside config (Windows: DPAPI; other OS: AES-GCM + `login.key` mode 0600). Never plaintext in `config.json`
- If a session already exists in the jar, pass `--force`
- Form includes **服务地址** (base URL), prefilled from `--base-url` / `PLUMELOG_BASE_URL` / config / default; the submitted value is used for login and persisted via the normal config write path when it differs

### Status output

`auth status` / `whoami` print **metadata only** (`has_cookie`, `cookie_path`, `base_url`, …) — never the cookie value.

### Opt-in auto re-login

`PLUMELOG_AUTO_LOGIN=1` or `--auto-login`, **or** a remembered credential with `auto_login` from the web form: on upstream 401, re-login once and replay the request.

Credential priority for that silent login:

1. `PLUMELOG_LOGIN_USERNAME` / `PLUMELOG_LOGIN_PASSWORD` (env)
2. Encrypted `login.cred` (from web「记住并自动登录」)
3. `login_username` in config (username only; never a password field)
