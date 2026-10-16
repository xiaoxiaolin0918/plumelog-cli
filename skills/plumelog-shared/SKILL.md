---
name: plumelog-shared
version: 0.1.0
description: "Use for plumelog CLI setup/auth: auth login/status/logout, config, doctor, JSON ok/result envelope, --dry-run/--jq/--yes/--admin-password, high-risk gate (exit 100 + confirmation_required, yunxiao-aligned; dual --admin-password), cookie jar, PLUMELOG_* env, path-prefix, no-secret rules."
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
3. **失败信封**：当前多为 stdout 上的 `{ "ok": false, "error": "<string>", "risk": "..." }`；`error` 是 **confirmation_required 对象（或普通失败时的 string）**，不是对象。
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
# 无独立 whoami：用 auth status（可 --probe）等价
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

覆盖：`--cookie-jar`。`auth status` / `doctor` / `config show` **只**报元数据，不打印 Cookie 值。

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

## 输出契约（现状）

成功（stdout，exit 0）：

```json
{ "ok": true, "result": { } }
```

失败（常 stdout，exit ≠ 0）：

```json
{ "ok": false, "error": "human message", "risk": "high-risk-write" }
```

`--jq '<expr>'` 过滤整份信封（例：`--jq .result`）。`--format json|pretty|table`。

dry-run 成功例：

```json
{ "ok": true, "dry_run": true, "risk": "high-risk-write", "method": "POST", "url": "...", "headers": { "Cookie": "<redacted>" }, "would_require": ["--yes"] }
```

## 风险与确认（现状 vs yunxiao）

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

### 门禁信封 — 当前实现（Agent 必须按此分支）

实测（2026-10-16）：

- 缺 `--yes`（及缺 admin 时）：**exit 10**，stdout confirmation_required 对象（或普通失败时的 string） error，例如  
  exit **10** with error.subtype=`confirmation_required` (see references/output-and-risk.md)
- ****已**对齐 yunxiao：`exit 100` + `error: { type, subtype: "confirmation_required", hint, action }`

Agent 流程：识别 `ok==false` 且 `risk==high-risk-write`（或 error 含 `refusing high-risk-write`）→ **向用户展示 action/风险并等待同意** → 同意后把 `--yes`（及需要的 `--admin-password`）追加到原 argv 重试。**禁止**静默加 `--yes`。

对齐 exit 100 + `confirmation_required` 列为 P0 产品债（见 `docs/gap-vs-yunxiao.md`）；在未改 CLI 前，skill/脚本勿按 yunxiao 解析。

`--dry-run`：只预览；即使带 `--yes` 也不发网；预览脱敏 Cookie / adminPassWord。

## doctor 排障序

```bash
plumelog doctor
plumelog auth status --probe
plumelog auth login --username "$PLUMELOG_LOGIN_USERNAME" --password "$PLUMELOG_LOGIN_PASSWORD"
plumelog config show
```

## whoami

**无**根级 `whoami`。等价：`plumelog auth status`（加 `--probe` 可验证会话）。P2 可补别名，见 gap 文档。

## skills / 安装

CLI 已提供与 yunxiao 对齐的 skills 子命令：

```bash
plumelog skills list
plumelog skills read plumelog-shared
plumelog skills path                 # 仓内/旁路 skills 根
plumelog skills path plumelog-query  # 单个 skill 目录
plumelog skills install              # 复制到 ~/.agents/skills（Windows: %USERPROFILE%\.agents\skills）
plumelog skills install --force      # 覆盖已存在
plumelog skills install --symlink    # 软链（需 OS 支持）
plumelog skills install --skill plumelog-shared --skill plumelog-query
plumelog skills install --dir "D:\custom\skills" --dry-run
```

- 源目录：二进制旁 `skills/`、源码树 `skills/`、或 cwd `skills/`（含 `plumelog-*` + `SKILL.md`）
- 默认安装根：`~/.agents/skills`（Unix）/ `%USERPROFILE%\.agents\skills`（Windows）
- **无** `plumelog update` / self-update（P2）；构建：`go build -o plumelog.exe .`
## Windows / MSYS

若在 git-bash 里调 `plumelog api` 且 path 以 `/` 开头，MSYS 可能改写路径。规避：`MSYS_NO_PATHCONV=1`、`MSYS2_ARG_CONV_EXCL='*'`，或用 PowerShell/cmd。CLI 侧尚未做 yunxiao 式 unmangle（P2）。

## Reference

- [`references/output-and-risk.md`](references/output-and-risk.md)
- 差距与 backlog：`docs/gap-vs-yunxiao.md`
- 发版手跑：`docs/smoke-checklist.md`