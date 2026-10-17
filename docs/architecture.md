# plumelog-cli architecture (v0.1.1)

> Contract for the `plumelog` CLI against a [PlumeLog](https://github.com/fayechenlong/plumelog) server (`MainController`, `LoginCheckInterceptor`, `Home.vue`). CLI patterns (cobra, auth/doctor/api, `--dry-run` / `--jq` / `--yes`) follow `yunxiao-cli`.
>
> Aligned with **release [v0.1.1](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.1)** / public repo [xiaoxiaolin0918/plumelog-cli](https://github.com/xiaoxiaolin0918/plumelog-cli) (GitHub only; no Origin mirror).
>
> **Never** print real passwords, `adminPassWord`, or Session Cookie values in logs, stdout, docs, or `docs/wiki/` examples.

---

## 1. Command tree (current)

```
plumelog
├── auth
│   ├── login          # POST /login → Cookie jar; --web/--browser = 127.0.0.1 form
│   ├── logout         # POST /logout + clear jar
│   └── status         # metadata only; optional --probe
├── doctor             # connectivity + auth + getRunModel / getQueueCounts
├── config
│   └── get | set | path | show
├── query
│   ├── search         # shortcut filters → ES DSL → POST /clientQuery (or /query)
│   ├── dsl            # raw DSL body (--file / stdin); --index auto-aligns dtTime
│   └── apps           # POST /queryAppNames or /getAppNames
├── trace
│   └── get            # POST /clientQuery?trace=true
├── app
│   ├── list
│   └── extend list | add | del
├── alarm
│   └── list | save | delete | messages
├── admin
│   ├── queue counts | clear
│   ├── index info | delete | reset
│   ├── run-model
│   └── delete-by-query
├── api <METHOD> <path>   # escape hatch; risk varies by path
├── skills
│   └── list | path | read | install | update
├── version
└── whoami             # thin alias of auth status
```

Every command `--help` marks **Risk:** `read` | `write` | `high-risk-write` (`api` is **varies**).

---

## 2. Hosting & identity

| Item | Value |
|------|--------|
| Public GitHub | https://github.com/xiaoxiaolin0918/plumelog-cli |
| Module path | `github.com/xiaoxiaolin0918/plumelog-cli` |
| Binary name | `plumelog` |
| License | MIT |
| Origin | **none** (GitHub only) |
| Default base URL | `http://localhost:8891` (no trailing slash) |
| Release documented here | v0.1.1 |

---

## 3. Auth: Session Cookie (not OAuth / not Bearer)

PlumeLog uses the same servlet session as the web UI.

| Step | Behavior |
|------|----------|
| Login | `POST {/login,/plumelog/login}` JSON `{"username","password"}` |
| Success | Server sets session token; CLI persists `Set-Cookie` (usually `JSESSIONID`) in Netscape `cookies.txt` |
| Failure | Upstream `402` → invalid credentials |
| Logout | `POST /logout` + clear local jar |
| Gate | Interceptor requires session on POST when `loginUsername` is configured; open auth when unset |
| Secrets | Passwords and cookie **values** never printed; `auth status` / `whoami` / `doctor` / `config show` expose metadata only (`has_cookie`, `cookie_path`, `base_url`, …) |

### CLI login modes

| Mode | How |
|------|-----|
| Flags / env / TTY | `--username` + `--password` / `PLUMELOG_LOGIN_PASSWORD` / prompt |
| Open auth | Empty username still POSTs `/login` when server has no `login.username` → `auth: open` |
| **`--web` / `--browser`** | Bind **127.0.0.1** only (never `0.0.0.0`), one-shot tokenized URL, system browser form, ~**120s** timeout, then POST `/login` and write jar. Passwords not logged or written to disk. Existing jar session requires **`--force`**. Dry-run does not leak plaintext. |
| Auto re-login (opt-in) | `PLUMELOG_AUTO_LOGIN=1` or `--auto-login`: on upstream 401, re-login **once** from env and replay (Issue #2) |

Cookie jar path:

| OS | Default |
|----|---------|
| Unix | `$XDG_CONFIG_HOME/plumelog/cookies.txt` or `~/.config/plumelog/cookies.txt` |
| Windows | `%APPDATA%\plumelog\cookies.txt` |

Override: `--cookie-jar`. File mode `0600` (Unix).

---

## 4. JSON envelope & exit codes

**Success** → **stdout**, exit `0`:

```json
{ "ok": true, "result": { } }
```

**Failure** → **stderr** (not stdout), non-zero exit; `error` is always an **object**:

```json
{ "ok": false, "error": { "type": "cli", "message": "..." }, "risk": "high-risk-write" }
```

| Situation | Exit | Notes |
|-----------|------|--------|
| High-risk missing `--yes` (and admin when required) | **10** | `error.subtype=confirmation_required` (also may include `action` / `hint` / `risk`) |
| Other CLI/HTTP failures | non-zero (typically 1) | Read stderr envelope |

`--jq` filters the whole envelope (e.g. `--jq .result`). `--format json|pretty|table`.

Dry-run success still on stdout with `"dry_run": true`; Cookie / `adminPassWord` redacted; `would_require` lists missing gates. Dry-run **never** sends HTTP even with `--yes`.

> Agents: do **not** treat upstream PlumeLog `code == 0` as CLI success. Use `ok == true` or process exit 0. Do **not** expect failure JSON on stdout or `error` as a string.

---

## 5. Risk policy

| Risk | Behavior |
|------|----------|
| `read` | Runs immediately |
| `write` | Runs immediately; prefer `--dry-run` before mutating |
| `high-risk-write` | Needs explicit `--yes` / `-y` to execute |

**Also need `--admin-password`** (or `PLUMELOG_ADMIN_PASSWORD` → query `adminPassWord`, never printed):

- `admin queue clear` (`/deleteQueue`)
- `admin index delete` (`/deleteIndex`)
- `api` hitting those paths

**`--yes` only** (upstream does not check admin password; CLI does not send it):

- `admin index reset`, `admin delete-by-query`, `alarm delete`, `app extend del`

Implementation: `internal/risk` (`ExitConfirmationRequired = 10`, `GateResult`).

---

## 6. Global flags & config

| Flag | Notes |
|------|--------|
| `--base-url` | Or `PLUMELOG_BASE_URL` / config |
| `--path-prefix` | `auto` (default: try `""` then `/plumelog`), `root` / `""`, or `/plumelog` |
| `--format` | `json` (default) \| `pretty` \| `table` |
| `--jq` | jq expression on envelope |
| `--dry-run` | Print request only |
| `--yes` / `-y` | Confirm high-risk |
| `--admin-password` | Or `PLUMELOG_ADMIN_PASSWORD` |
| `--cookie-jar` / `--config` / `--timeout` | Defaults: config dir + 30s |

Precedence: **flags > environment > `config.json` > defaults**.

`config set` **refuses** password fields. Config lives next to the jar (`config.json`, mode `0600`).

Env vars: `PLUMELOG_BASE_URL`, `PLUMELOG_PATH_PREFIX`, `PLUMELOG_LOGIN_USERNAME`, `PLUMELOG_LOGIN_PASSWORD`, `PLUMELOG_ADMIN_PASSWORD`, `PLUMELOG_AUTO_LOGIN`.

---

## 7. Dual path prefix (`/` and `/plumelog`)

Server routes are often registered as `{ "/foo", "/plumelog/foo" }`. CLI URL = `base_url` + `path_prefix` + `path`. Paths already starting with `/plumelog/` are not prefixed again.

Primary mappings (unchanged in spirit from earlier drafts): `auth` → `/login` `/logout`; `query search` → `/clientQuery` or `/query` with `--index`/`--range`; `trace get` → `/clientQuery?trace=true`; alarm/admin/app extend as in README.

Index naming: run logs `plume_log_run_YYYYMMDD*`; trace `plume_log_trace_YYYYMMDD*`; monitor messages `plumelog_monitor_message_key`.

### `--index` without explicit time (Issue #4)

When `--index` is set and the user did **not** pass `--from-time` / `--to-time` / `--range`, the CLI auto-aligns the `dtTime` window to the index `YYYYMMDD` (full day) so historical indexes do not silently return 0 hits. Applies to **`query search` and `query dsl`** (DSL body `dtTime` is rewritten). Explicit time flags are not overridden. `--range` is relative to *now*; prefer explicit times for historical indexes. A stderr `note:` may accompany the alignment.

---

## 8. Skills (embedded companions)

Skills ship under `skills/plumelog-*` and are **embedded in the binary** (`skills/embed.go`). Commands:

| Command | Role |
|---------|------|
| `skills list` | List bundled skills |
| `skills list --check` | Compare installed vs bundled frontmatter `version` |
| `skills read <name>` | Print `SKILL.md` |
| `skills path [name]` | Print skills root or one skill path |
| `skills install` | Copy/symlink into agent skills dir (`~/.agents/skills` or `%USERPROFILE%\.agents\skills`) |
| `skills update` | Force-refresh installed copy to match **this** binary (Issue #5) |

`list --check` statuses (installed **relative to** bundled): `equal` | `outdated` | `ahead` | `invalid`, plus `missing` / `unknown` when a side lacks a comparable version (`cmd.CompareSemver`).

Source resolution: directory beside binary → source-tree `skills/` → cwd `skills/` → **embedded** tree (materialized to user cache).

Load order for agents: read **plumelog-shared** first, then domain skills (`plumelog-query`, `trace`, `app`, `alarm`, `admin`, `api`).

---

## 9. Query shortcuts (Home.vue alignment)

`query search` builds `bool.must[]` (optional `must_not` for `--exclude-app`): `match_phrase` on `appName` / `env` / `logLevel` / `traceId` / `className` / `serverName`; `query_string` on `content`; repeatable `--extend field=value`; `range` on `dtTime`. Default sort `dtTime desc`; with `--trace-id` → `dtTime asc` + `seq asc`. Highlight on unless `--no-highlight`.

Default time window: config `default_time_window` (1h). Future `--to-time` clamped to now.

---

## 10. doctor / whoami / api / MSYS

- **doctor**: reachability, cookie presence, `getRunModel`, optional queue counts, prints path_prefix / cookie path (no secrets).
- **whoami**: root alias of `auth status` (same fields); `--probe` allowed.
- **api**: raw METHOD + path; supports `--dry-run` / gates for destructive paths; ingest (`/sendLog`, `/getlog`) via `api` only.
- **Windows/MSYS**: `api` paths starting with `/` may be mangled; CLI auto-unmangles (stderr `note:`); override with `MSYS_NO_PATHCONV` / `PLUMELOG_API_NO_UNMANGLE=1`.

---

## 11. Out of scope (v0)

- Live tail WebSocket (`ws://<host>/plumelog/websocket`) — use the web UI.
- No full `plumelog update` / self-update binary yet (skills update only).

---

## 12. Related docs

| Doc | Purpose |
|-----|---------|
| [README.md](../README.md) / [README.zh-CN.md](../README.zh-CN.md) | Install, security, quick start |
| [skills/README.md](../skills/README.md) / `skills/plumelog-shared` | Agent contracts |
| [docs/smoke-checklist.md](smoke-checklist.md) | Release smoke (exit **10** + `confirmation_required`) |
| [SECURITY.md](../SECURITY.md) / [CONTRIBUTING.md](../CONTRIBUTING.md) | Vulnerability reports / contrib |
| [docs/wiki/](wiki/README.md) | In-repo pages: Home, Getting-Started, Auth, Skills-for-AI, Architecture (not GitHub Wiki) |

Upstream references (PlumeLog server/UI): `MainController`, `LoginCheckInterceptor`, `Home.vue` (`getShouldFilter` / `clientQuery`), `http.js` (`withCredentials`).
