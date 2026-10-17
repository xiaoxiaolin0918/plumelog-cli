---
name: plumelog-shared
version: 0.1.2
description: "Use for plumelog CLI setup/auth: auth login/status/logout, whoami, config, doctor, JSON ok/result envelope (success stdout; failure stderr with object error), --dry-run/--jq/--yes/--admin-password, high-risk gate (exit 10 + confirmation_required), cookie jar, PLUMELOG_* env, path-prefix, no-secret rules."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog --help"
---

# plumelog 共享规则

所有 `plumelog-*` skill 共享的底座：Session Cookie 鉴权、配置优先级、JSON 信封、风险门禁与脱敏。

开始任何域操作前先读本文件。细项见 [`references/output-and-risk.md`](references/output-and-risk.md)。

## 通用准则

1. **调用前先确认用法**：读对应域 skill，或跑 `plumelog <cmd> --help`；帮助里标有 Risk：`read` | `write` | `high-risk-write`。
2. **默认 JSON**：判断成功用 **`ok == true`**（或进程退出码 0）。**不要**用上游 PlumeLog 的 `code == 0`。成功负载在 **`result`**（不是 yunxiao 的 `data`）。
3. **失败信封**：写到 **stderr** 的 `{ "ok": false, "error": { "type", "message", ... }, ... }`；`error` **始终是对象**（可含 `subtype` / `hint` / `risk` / `action`）。缺 `--yes` 时 `error.subtype=confirmation_required` 且 **exit 10**（不是 exit 100，也不是 stdout 上的 string error）。
4. **禁止**把登录密码、`adminPassWord`、Session Cookie 值写入终端、回复、报告或 skill 示例。只用 `has_cookie` / `cookie_path` / 打码后的 dry-run。

## 认证（Session Cookie，非 Bearer）

PlumeLog 与网页相同：`POST /login` 拿 `JSESSIONID` 等 Cookie，持久化到 Netscape `cookies.txt`。

```bash
export PLUMELOG_BASE_URL="http://plumelog.iipcloud.cn"   # 生产已验证；本地默认 http://localhost:8891
export PLUMELOG_LOGIN_USERNAME="..."                     # 勿提交真实值
export PLUMELOG_LOGIN_PASSWORD="..."                     # 勿打印

plumelog doctor
plumelog auth login --username "$PLUMELOG_LOGIN_USERNAME" --password "$PLUMELOG_LOGIN_PASSWORD"
plumelog auth status
plumelog auth status --probe    # 只读 POST /getRunModel 探会话
plumelog whoami                 # 根级别名，字段与 auth status 相同
plumelog whoami --probe
plumelog auth logout
```

### 生产 URL 与凭据来源

| 项 | 约定 |
|----|------|
| 生产 base URL（E2E 已通） | `http://plumelog.iipcloud.cn` |
| CLI 正式环境变量 | `PLUMELOG_BASE_URL` / `PLUMELOG_LOGIN_USERNAME` / `PLUMELOG_LOGIN_PASSWORD` / `PLUMELOG_ADMIN_PASSWORD` / `PLUMELOG_PATH_PREFIX` |
| 可选映射（zhiyi-ops-cli `.env`） | `PLUMELOG_URL` → `PLUMELOG_BASE_URL`；`PLUMELOG_USER` → `PLUMELOG_LOGIN_USERNAME`；`PLUMELOG_PASSWORD` → `PLUMELOG_LOGIN_PASSWORD`。**映射时不要把密码写入文件或报告。** |

Cookie jar 默认：

| OS | 路径 |
|----|------|
| Windows | `%APPDATA%\plumelog\cookies.txt` |
| Unix | `$XDG_CONFIG_HOME/plumelog/cookies.txt` 或 `~/.config/plumelog/cookies.txt` |

覆盖：`--cookie-jar`。`auth status` / `whoami` / `doctor` / `config show` **只**报元数据，不打印 Cookie 值。

## 配置优先级

`flags > environment > config.json > defaults`

```bash
plumelog config show
plumelog config path
plumelog config set base_url http://127.0.0.1:8891
plumelog config set path_prefix root    # 或 /plumelog；auto 为默认
```

`config set` **拒绝**写入密码类字段。密码只用 flag 或环境变量。

### `--path-prefix`

| 值 | 行为 |
|----|------|
| `auto`（默认） | 先试 `""`，失败再试 `/plumelog`（若配置偏好 `/plumelog` 则顺序相反） |
| `root` / `""` | 无前缀 |
| `/plumelog` | 固定前缀 |

## 输出契约

成功（**stdout**，exit 0）：

```json
{ "ok": true, "result": { } }
```

失败（**stderr**，exit ≠ 0）：

```json
{ "ok": false, "error": { "type": "cli", "message": "human message" }, "risk": "high-risk-write" }
```

非 0 退出时请读 **stderr** 上的信封；不要假定失败 JSON 在 stdout，也不要把 `error` 当 string。

`--jq '<expr>'` 过滤整份信封（例：`--jq .result`）。`--format json|pretty|table`。

dry-run 成功例（仍在 stdout）：

```json
{ "ok": true, "dry_run": true, "risk": "high-risk-write", "method": "POST", "url": "...", "headers": { "Cookie": "<redacted>" }, "would_require": ["--yes"] }
```

## 风险与确认

| Risk | 行为 |
|------|------|
| `read` | 可直接执行 |
| `write` | 可直接执行；写前建议 `--dry-run` |
| `high-risk-write` | **必须**用户显式同意后再加 `--yes`；禁止静默加 `--yes` |

双门禁（还要 `--admin-password` / `PLUMELOG_ADMIN_PASSWORD`，映射 query `adminPassWord`，永不打印）：

- `admin queue clear`
- `admin index delete`
- `api` 命中 `deleteQueue` / `deleteIndex`

仅需 `--yes`（不发 admin 密码）：`admin index reset`、`admin delete-by-query`、`alarm delete`、`app extend del`，以及 `api` 上其它已知破坏路径。

### 门禁信封（Agent 必须按此分支）

缺 `--yes`（及缺 admin 时，真实执行、非 dry-run）：

- 进程 **exit 10**（**不是** exit 100）
- **stderr** JSON：`ok: false`，`error` 为对象，且 `error.subtype == "confirmation_required"`（详见 [`references/output-and-risk.md`](references/output-and-risk.md)）

Agent 流程：识别 `ok==false` 且 `error.subtype==confirmation_required`（或 `risk==high-risk-write`）→ **向用户展示 action/风险并等待同意** → 同意后把 `--yes`（及需要的 `--admin-password`）追加到原 argv 重试。**禁止**静默加 `--yes`。

`--dry-run`：只预览；即使带 `--yes` 也不发网；预览脱敏 Cookie / adminPassWord。

## doctor 排障序

```bash
plumelog doctor
plumelog auth status --probe
plumelog auth login --username "$PLUMELOG_LOGIN_USERNAME" --password "$PLUMELOG_LOGIN_PASSWORD"
plumelog config show
```

## whoami

根级 `plumelog whoami`（Risk: read）是 **`auth status` 的薄别名**：同一套输出路径与字段（`base_url` / `authenticated` / `has_cookie` / `cookie_path` / `path_prefix`）。永不打印 Cookie 值或密码；`--probe` 用 `/getRunModel` 精炼 `authenticated`。

## skills / 安装

```bash
plumelog skills list
plumelog skills read plumelog-shared
plumelog skills path
plumelog skills path plumelog-query
plumelog skills install
plumelog skills install --force
plumelog skills install --symlink
plumelog skills install --skill plumelog-shared --skill plumelog-query
plumelog skills install --dir "D:\custom\skills" --dry-run
```

- 源目录：二进制旁 `skills/`、源码树 `skills/`、或 cwd `skills/`（含 `plumelog-*` + `SKILL.md`）
- 默认安装根：`~/.agents/skills`（Unix）/ `%USERPROFILE%\.agents\skills`（Windows）
- **无** `plumelog update` / self-update（P2）；构建：`go build -o plumelog.exe .`

## Windows / MSYS

若在 git-bash 里调 `plumelog api` 且 path 以 `/` 开头，MSYS 可能改写路径。CLI **会自动 unmangle**（stderr `note:`）；仍可用 `MSYS_NO_PATHCONV=1`、`MSYS2_ARG_CONV_EXCL='*'`、双斜杠 `//clientQuery`，或 `PLUMELOG_API_NO_UNMANGLE=1` 关闭。

## Reference

- [`references/output-and-risk.md`](references/output-and-risk.md)
- 差距与 backlog：`docs/gap-vs-yunxiao.md`
- 发版手跑：`docs/smoke-checklist.md`

## 401 自动重登（Issue #2，opt-in）

Session Cookie 可能数小时失效。遇上上游 `code:401` / HTTP 401 时，可选择自动重登**一次**并重放原请求：

```bash
export PLUMELOG_LOGIN_USERNAME="..."
export PLUMELOG_LOGIN_PASSWORD="..."   # 勿打印 / 勿写入报告
export PLUMELOG_AUTO_LOGIN=1             # 或命令加 --auto-login
plumelog query search --app demo -q timeout --auto-login
```

约束：

- 密码只从 `PLUMELOG_LOGIN_PASSWORD`（用户名还可来自 config `login_username`）读取；
- **只重试一次**；再 401 原样报错；
- 走共享 HTTP client + cookie jar；dry-run / `auth login` 自身不触发；
- 密码永不落入日志、dry-run、错误信封（与现有脱敏规则一致）。
