# plumelog

[![CI](https://github.com/xiaoxiaolin0918/plumelog-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/xiaoxiaolin0918/plumelog-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/xiaoxiaolin0918/plumelog-cli?include_prereleases)](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.3)

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
go install github.com/xiaoxiaolin0918/plumelog-cli@v0.1.3
```

Release notes: [v0.1.3](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.3).

## What's new in 0.1.3

### Breaking

- **`query apps` result shape**: in v0.1.2 and earlier, `result` was often a **bare JSON array** of app names. From 0.1.3 it is always an **object**: `{ "apps": [...], "count": N, ... }` (optional `app_like` / `sample` / enrich fields). Migrate scripts and `--jq` from `.result[0]` / treating `.result` as an array to **`.result.apps`**.

### Fixes & features (Issues #18–#23)

- Multi-word `-q` / `--content` uses Lucene `default_operator: AND` (#23); explicit `OR` / quotes still work
- `--range day|week|month` also expands body `dtTime` to **24h / 7d / 30d** when `--from-time`/`--to-time` are omitted (#22). **`month` means 30 days, not a calendar month**
- Empty upstream body → stderr `ok:false`, `error.subtype=empty_body`, `result.hits.total=0`, non-zero exit (#19)
- `trace get` with 0 hits stays `ok:true` and adds a hint to try `query search --trace-id` (#21)
- `--from-time` / `--to-time` accept `now`, relative (`-3d` / `-2h` / `+15m`), RFC3339, local date/time layouts, and epoch ms|s (#18)
- `query apps --app-like` / `--sample` / `--enrich` (#20)

See also [CHANGELOG.md](CHANGELOG.md).

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

`auth login --web` shows a Chinese form with **服务地址** (base URL, prefilled from `--base-url` / env / config) and optional **记住并自动登录**, which stores an encrypted password beside config as `login.cred` (never plaintext in `config.json`). Details: [Auth wiki](docs/wiki/Auth.md).

`auth status` prints only `has_cookie`, `cookie_path`, and `base_url` — never the cookie value.

## Query

Shortcut filters follow Home.vue: `match_phrase` on `appName`, `env`, `logLevel`, `traceId`, `className`, `serverName`; `query_string` on `content` with **`default_operator: AND`** (#23); repeatable `--extend field=value`; `range` on `dtTime`. `--exclude-app` moves the app clause into `must_not`.

Default sort is `dtTime desc`. A `--trace-id` switches to `dtTime asc` plus `seq asc`. Highlight is on unless `--no-highlight`.

`--from-time` / `--to-time` accept epoch ms|s, RFC3339, local `YYYY-MM-DD[ HH:MM[:SS]]`, `now`, and relative offsets like `-3d` / `-2h` / `+15m` (#18). Date-only values start at local midnight; a future `--to-time` is clamped to now.

```bash
./plumelog query search --app demo --level ERROR -q "timeout AND retry" \
  --from-time 2024-01-02T00:00:00Z --to-time 2024-01-02T01:00:00Z

./plumelog query search --from-time -3d --to-time now --level ERROR
./plumelog query search --dsl-file query.json --from-time 1700000000000 --to-time 1700003600000
./plumelog query dsl --file query.json
./plumelog query apps
./plumelog query apps --source es
./plumelog query apps --app-like demo --enrich
```

`clientStartDate` / `clientEndDate` choose daily indices (`plume_log_run_YYYYMMDD` or, with `--trace`, `plume_log_trace_YYYYMMDD`). They default to the last `default_time_window` (1h).

`--index` or `--range day|week|month` switches the call to `POST /query`. `--range` is relative to **now** and, when times are omitted, also expands body `dtTime` to **24h / 7d / 30d** (#22). **`month` = 30 days (not a calendar month).** Prefer explicit times for historical indexes.

With `--index` alone (no `--from-time` / `--to-time` / `--range`), the CLI auto-aligns the `dtTime` window to the index `YYYYMMDD` so historical indexes do not silently return 0 hits. The same auto-align applies to `query dsl --index`.

An empty upstream body is treated as failure: stderr `ok:false`, `error.subtype=empty_body`, `result.hits.total=0`, non-zero exit (#19). True zero-hit ES responses remain `ok:true`.

**`query apps` result (breaking vs ≤0.1.2):** `result` is `{ "apps": [...], "count": N, ... }`, not a bare array. Use `--jq '.result.apps'`.

`trace get` with zero hits stays `ok:true` and may set `hint` pointing at `query search --trace-id` (#21).

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

## Documentation

Human- and agent-oriented pages live **in this repository** under [docs/wiki/](docs/wiki/README.md) (start at [Home](docs/wiki/Home.md)). **GitHub Wiki is deprecated and disabled**; docs/wiki/ is the only source of truth.

- [Getting Started](docs/wiki/Getting-Started.md)
- [Auth](docs/wiki/Auth.md) (session cookie, `auth login --web`)
- [Skills for AI](docs/wiki/Skills-for-AI.md)
- [Architecture (short)](docs/wiki/Architecture.md) → full contract: [docs/architecture.md](docs/architecture.md)

Chinese overview: [docs/wiki/Home.zh-CN.md](docs/wiki/Home.zh-CN.md). Smoke checklist: [docs/smoke-checklist.md](docs/smoke-checklist.md).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Not in v0

The live tail WebSocket (`ws://<host>/plumelog/websocket`) is not implemented. Use the web UI for the streaming console.
