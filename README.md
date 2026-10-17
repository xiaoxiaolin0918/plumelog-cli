# plumelog

[![CI](https://github.com/xiaoxiaolin0918/plumelog-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/xiaoxiaolin0918/plumelog-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/xiaoxiaolin0918/plumelog-cli?include_prereleases)](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.1)

Command-line client for a [PlumeLog](https://github.com/fayechenlong/plumelog) server (default port **8891**).

Binary name: **`plumelog`**. License: [MIT](LICENSE).

Chinese README: [README.zh-CN.md](README.zh-CN.md).

PlumeLog authenticates with a servlet session cookie (usually `JSESSIONID`), the same way the web UI does. This CLI does **not** use bearer tokens. Cookie values and passwords are never printed.

## Install

Requires Go **1.22+**.

**From source (recommended for contributors):**

```bash
git clone https://github.com/xiaoxiaolin0918/plumelog-cli.git
cd plumelog-cli
go test ./...
go build -o plumelog .
./plumelog --help
./plumelog version
```

**Windows (PowerShell):**

```powershell
go build -o plumelog.exe .
go test ./...
.\plumelog.exe --help
```

**Go install (module path = repo):**

```bash
go install github.com/xiaoxiaolin0918/plumelog-cli@v0.1.1
```

Release notes: [v0.1.0](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.1).

## Security (read this first)

- **Never commit** cookies, passwords, `.env`, or production credentials.
- Prefer env vars (`PLUMELOG_LOGIN_PASSWORD`, `PLUMELOG_ADMIN_PASSWORD`) over putting secrets on the command line in shared shells/history.
- Use placeholder hostnames in docs and issues (for example `http://127.0.0.1:8891`).
- Cookie jars live under the OS config directory with mode `0600` and are gitignored (`*_cookies.txt`, `.env`).
- See [SECURITY.md](SECURITY.md) to report vulnerabilities privately.

## 30-second start

```bash
export PLUMELOG_BASE_URL=http://127.0.0.1:8891
export PLUMELOG_LOGIN_PASSWORD='YOUR_PASSWORD'
export PLUMELOG_ADMIN_PASSWORD='YOUR_ADMIN_PASSWORD'

./plumelog doctor
./plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
./plumelog query search --app demo --level ERROR -q timeout
./plumelog trace get --trace-id TRACE_ID
./plumelog api POST /getRunModel
```

## Command tree

```
plumelog
  auth login | logout | status
  doctor
  config get | set | path | show
  query search | dsl | apps
  trace get
  app list | extend list | extend add | extend del
  alarm list | save | delete | messages
  admin queue counts | queue clear
  admin index info | index delete | index reset
  admin run-model | delete-by-query
  api <METHOD> <path>
  skills list | path | read | install | update
  version
  whoami
```

Every command `--help` is marked **Risk: read**, **write**, or **high-risk-write**. `api` is **varies** based on the path.

## Auth

```bash
./plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
./plumelog auth login --web          # browser form on 127.0.0.1 (alias: --browser); --force replaces jar
./plumelog auth status
./plumelog auth status --probe
./plumelog whoami
./plumelog auth logout
```

`POST /login` with `{"username","password"}`. `402` means invalid credentials. When the server has no `login.username`, an empty `auth login` still posts and records `auth: open`.

`auth status` prints only `has_cookie`, `cookie_path`, and `base_url` — never the cookie value.

## Query

Shortcut filters follow Home.vue: `match_phrase` on `appName`, `env`, `logLevel`, `traceId`, `className`, `serverName`; `query_string` on `content`; repeatable `--extend field=value`; `range` on `dtTime`. `--exclude-app` moves the app clause into `must_not`.

Default sort is `dtTime desc`. A `--trace-id` switches to `dtTime asc` plus `seq asc`. Highlight is on unless `--no-highlight`.

```bash
./plumelog query search --app demo --level ERROR -q "timeout AND retry" \
  --from-time 2024-01-02T00:00:00Z --to-time 2024-01-02T01:00:00Z

./plumelog query search --dsl-file query.json --from-time 1700000000000 --to-time 1700003600000
./plumelog query dsl --file query.json
./plumelog query apps
./plumelog query apps --source es
```

`clientStartDate` / `clientEndDate` choose daily indices (`plume_log_run_YYYYMMDD` or, with `--trace`, `plume_log_trace_YYYYMMDD`). They default to the last `default_time_window` (1h). A future `--to-time` is clamped to now.

`--index` or `--range day|week|month` switches the call to `POST /query`.

With `--index` alone (no `--from-time` / `--to-time` / `--range`), the CLI auto-aligns the `dtTime` window to the index `YYYYMMDD` so historical indexes do not silently return 0 hits. The same auto-align applies to `query dsl --index`. `--range` is relative to now; prefer explicit times for historical indexes.

## Skills (agent companions)

Bundled agent skill markdown can be listed, read, installed, or force-refreshed to match the running binary:

```bash
./plumelog skills list
./plumelog skills list --check
./plumelog skills install
./plumelog skills update
```

## API escape hatch

```bash
./plumelog api POST /clientQuery \
  --query clientStartDate=1700000000000 \
  --query clientEndDate=1700003600000 \
  --query size=20 --query from=0 \
  --body-file query.json

./plumelog api POST /deleteQueue --dry-run
```

Paths are `base_url + path_prefix + path`. `--path-prefix` is `auto` (try `""` then `/plumelog`, preferring a prefix stored in config), `root` (only `""`), or `/plumelog`. A path that already starts with `/plumelog/` is not prefixed again.

Ingest routes (`/sendLog`, `/getlog`) are not first-class commands. Reach them through `api`.

## Risk flags

| Risk | Behavior |
|------|----------|
| read | Runs immediately |
| write | Runs immediately. `--dry-run` prints the request and does not send it |
| high-risk-write | Needs `--yes` / `-y` to execute |

`admin queue clear` and `admin index delete` also need `--admin-password` (or `PLUMELOG_ADMIN_PASSWORD`). The CLI sends it as the query parameter `adminPassWord` and never prints the value.

Also high-risk (`--yes` only): `admin index reset`, `admin delete-by-query`, `alarm delete`, `app extend del`. Upstream `resetIndex` and `deleteByQuery` do not check the admin password, so the CLI does not put it on the wire.

`--dry-run` never sends, even when `--yes` is present. The preview includes method, URL, redacted headers, and body. Missing confirmations are listed in `would_require`.

Success is a JSON envelope on **stdout**: `{"ok":true,"result":...}`. Failures write JSON to **stderr** with an **object** `error` (`{"ok":false,"error":{"type":"cli","message":"..."},...}`). Missing `--yes` on a high-risk-write exits **10** with `error.subtype=confirmation_required` (also on stderr). `--format pretty` indents it. `--format table` prints an array of objects as TSV when it can. `--jq` filters the envelope (for example `--jq .result`).

## Config and cookies

| | Unix | Windows |
|--|------|---------|
| Directory | `$XDG_CONFIG_HOME/plumelog` or `~/.config/plumelog` | `%APPDATA%\plumelog` |
| Config | `config.json` (mode `0600`) | same file name |
| Cookies | `cookies.txt` (Netscape cookie file, mode `0600`) | same file name |

```bash
./plumelog config path
./plumelog config show
./plumelog config set base_url http://127.0.0.1:8891
./plumelog config set path_prefix /plumelog
./plumelog config set path_prefix root
./plumelog config set default_time_window 15m
```

Passwords are refused by `config set`. Use flags or:

- `PLUMELOG_BASE_URL`
- `PLUMELOG_PATH_PREFIX` (`auto`, `root`, or `/plumelog`)
- `PLUMELOG_LOGIN_USERNAME`
- `PLUMELOG_LOGIN_PASSWORD`
- `PLUMELOG_ADMIN_PASSWORD`

Precedence: flags, then environment, then `config.json`, then defaults. Default base URL is `http://localhost:8891` with no trailing slash. Default timeout is 30s.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Smoke checklist: [docs/smoke-checklist.md](docs/smoke-checklist.md).

## Not in v0

The live tail WebSocket (`ws://<host>/plumelog/websocket`) is not implemented. Use the web UI for the streaming console.
