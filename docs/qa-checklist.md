# plumelog-cli 验收清单（质检）

对照：

- 架构契约（**门禁与 Cookie 以本文为准**）：`D:\work\ops\plumelog-cli\docs\architecture.md`（2026-10-16 定稿）
- 模式对照仓：`D:\work\grokbot\yunxiao-cli`（cobra、auth/doctor/api、`--dry-run`/`--jq`/`--yes`、Risk 分级、JSON 信封）

说明：

- 当前仓仅有 `.placeholder`（等 scaffold），**无 go.mod / 源码**。本文件先定验收标准；源码齐后再实跑，状态改为 pass/fail/skipped。
- 命令名以 architecture 为准：删除队列/索引为 **`admin queue clear`** / **`admin index delete`**（对应上游 `/deleteQueue`、`/deleteIndex`），不是扁平的 `delete-queue`/`delete-index`。
- 质检只列缺陷与结论；**不擅自合仓、发版、改业务代码、在日志/stdout/清单示例中打印真实口令、adminPassWord、Session Cookie 值**。
- 分级：Critical（阻塞发版）/ Important（应对齐）/ Minor（体验/文档）。

---

## 0. Cookie jar 路径（architecture 硬性约定）

验收时路径与行为必须与 `architecture.md`「Cookie jar 路径约定」一致：

| 项 | 约定 |
|----|------|
| 默认目录 | `$XDG_CONFIG_HOME/plumelog/`；Windows：`%APPDATA%\plumelog\`；若无则 `~/.config/plumelog/` |
| Cookie jar | `<config_dir>/cookies.txt`（Netscape 或 Go cookiejar 持久化，实现固定一种并文档化） |
| 配置文件 | `<config_dir>/config.json` |
| 权限 | Unix 建议 `0600`；Windows 尽量收紧 ACL |
| 覆盖 | 全局 `--cookie-jar` 可覆盖默认 jar 路径 |
| `auth status` | **只**输出 `has_cookie` / `cookie_path` / `base_url` 等元数据，**不**打印 Cookie 值 |

---

## 1. 构建与测试

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| B1 | `go build` 成功 | Critical | yunxiao `Makefile` `build`；architecture P0 | 仓根 `go build -o plumelog .`（或 Makefile 等价） | 退出码 0，产出可执行文件 | 待实跑 |
| B2 | `go test ./...` 全绿 | Critical | yunxiao `Makefile` `test`；`gate_dryrun_test.go` 等 | `go test ./...`（可选 `-race` / `vet`） | 全部 PASS；Critical 门禁有单测 | 待实跑 |
| B3 | dry-run / `--yes` / `--admin-password` 有单测 | Important | yunxiao `internal/risk`、`gate_dryrun_test.go`；architecture §7 | 确认缺 `--yes`、缺 admin 密码、dry-run 不执行均有失败用例可回归 | 至少各 1 个测试拦住 | 待实跑 |

---

## 2. 认证：login → cookie jar → query（可 mock）

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| A1 | `auth login` 写入 Cookie jar | Critical | architecture §3：`POST /login` → 持久化 Set-Cookie 到 jar | mock：`auth login --username U`（密码来自 flag/环境/stdin，**勿打进报告**） | 退出码 0；jar 落在约定路径（或 `--cookie-jar`）；stdout 可含 `{ok, base_url, cookie_path}`，**无 Cookie 原文/密码** | 待实跑 |
| A2 | 默认 jar 路径符合 architecture | Critical | architecture「Cookie jar 路径约定」 | 不传 `--cookie-jar` 时检查实际写入路径 | Windows 下为 `%APPDATA%\plumelog\cookies.txt`（或文档等价解析结果）；Unix 为 `$XDG_CONFIG_HOME/plumelog/cookies.txt` 或 `~/.config/plumelog/cookies.txt` | 待实跑 |
| A3 | 鉴权 POST 自动带 jar | Critical | architecture：需鉴权 POST 自动附带 Cookie | login 后 mock `query search`（或 `POST /clientQuery`） | 请求携带会话 Cookie；未 login 时对需鉴权 POST 得明确错误（如 401）与 hint | 待实跑 |
| A4 | `auth status` 脱敏 | Important | architecture：只输出 has_cookie / cookie_path / base_url | 已登录跑 `auth status`（可选 `--probe`） | 无 Cookie 值、无密码 | 待实跑 |
| A5 | login→query 可纯 mock | Important | yunxiao auth dry-run gate 测试风格 | `go test` 不连真实 PlumeLog | CI 离线可跑通 | 待实跑 |
| A6 | `auth logout` 清 jar | Important | architecture：`POST /logout` + 清空 jar | logout 后 status 显示无 cookie | jar 清空/删除；后续鉴权失败符合预期 | 待实跑 |

---

## 3. `--dry-run` 不发请求

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| D1 | 全局 `--dry-run` | Critical | architecture §2；yunxiao `root.go` `globalDryRun` | `plumelog --help` | 存在；说明为只打印将发请求、不执行 | 待实跑 |
| D2 | 写/高风险 + dry-run 零网络 | Critical | architecture §7：dry-run 时即使带 `--yes` 也不得真实执行；yunxiao `runMutating` | mock HTTP：对 `admin queue clear` / `admin index delete` / `api` 写方法加 `--dry-run` | transport 调用为 0；有 Method/URL/Headers(脱敏)/Body 预览 | 待实跑 |
| D3 | dry-run 预览脱敏 | Important | architecture 禁止打印 Cookie/口令；yunxiao redact 测试 | 预览含 Cookie/Authorization/admin 相关头时 | 密钥被 redact | 待实跑 |
| D4 | 与文档一致 | Important | architecture §2、§7；未来 README.zh-CN | 对照帮助与 README | 声称「不执行」处实测不发请求 | 待实跑 |

---

## 4. 高风险门禁（architecture §7）

**规则（以 architecture 为准）：**

- 凡 `high-risk-write`：**必须** `--yes`（缺则挡；建议退出码非 0，提示 Risk: high-risk-write，对齐 yunxiao exit 10 / `confirmation_required`）。
- **`admin queue clear`**、**`admin index delete`**（及若封装的 `alarm messages clear`）：在 `--yes` 之外 **另须** `--admin-password`（或 `PLUMELOG_ADMIN_PASSWORD`）映射上游 query `adminPassWord`；缺则挡；**永不回显密码**。
- `admin index reset`：须 `--yes`；`--admin-password` 建议可选（上游未强校验）。
- `alarm delete`、`app extend del`：须 `--yes`，不要求 admin 密码。

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| G1 | 无 `--yes` 挡所有 high-risk | Critical | architecture §7；yunxiao `CheckHighRisk` | 不带 `--yes` 跑 `admin queue clear`、`admin index delete`、`alarm delete`、`app extend del` 等 | **不发请求**；明确 high-risk / 需 `--yes` | 待实跑 |
| G2 | `admin queue clear` 无 admin 密码必挡 | Critical | architecture：`/deleteQueue` + adminPassWord | 有 `--yes`、无 `--admin-password` / 环境变量 | **不发请求**；提示需 `--admin-password`；输出无密码原文 | 待实跑 |
| G3 | `admin index delete` 无 admin 密码必挡 | Critical | architecture：`/deleteIndex` + adminPassWord | 同上 | 同 G2 | 待实跑 |
| G4 | 双门禁都满足才执行删除 | Critical | architecture §7 | `--yes` + 有效 `--admin-password`（仅 mock）于 queue clear / index delete | 才进入真实/mock 调用；缺任一仍挡 | 待实跑 |
| G5 | dry-run 优先于门禁执行 | Important | architecture：dry-run 即使有 `--yes` 也不执行 | `admin queue clear --dry-run`（可无 `--yes` 或无密码，以实现为准：至少不执行） | 只预览、不删 | 待实跑 |
| G6 | `--help` 风险措辞 | Important | architecture：命令 `--help` 标注 Risk；yunxiao Long `Risk:` | 抽查高风险命令 `--help` | 标明 `high-risk-write`、需 `--yes`；delete 类注明还需 `--admin-password` | 待实跑 |
| G7 | 仅 `--yes` 的高风险（无 admin） | Important | `alarm delete`、`app extend del` | 无 `--yes` 挡；有 `--yes`（mock）可通过门禁 | 与 §7 表一致 | 待实跑 |

---

## 5. `api` 逃生舱

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| E1 | 存在 `api <METHOD> <path>` | Critical | architecture §8；yunxiao `cmd/api.go` | `plumelog api --help` | 命令存在；支持双前缀 path | 待实跑 |
| E2 | api 支持 `--dry-run` / `--jq` / `--format` | Important | architecture §8 | `api … --dry-run` | 无真实请求；有预览 | 待实跑 |
| E3 | api 写方法需 `--yes`（对齐） | Important | yunxiao api 非只读 → CheckHighRisk；architecture 高风险原则 | 无 `--yes` 的 DELETE/非只读 POST | 被挡或按实现文档明确约定 | 待实跑 |

---

## 6. `doctor` / `whoami` / README.zh-CN 30 秒上手

> architecture 树以 `auth status` + `doctor` 为主；若实现提供根级 `whoami`，按下列验收；否则 **`auth status` 可作为身份探针等价项**（须在实跑记录注明）。

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| H1 | `doctor` | Critical | architecture §9；yunxiao `cmd/doctor.go` | `plumelog doctor`（可 mock） | 检查 base_url、Cookie 存在性、getRunModel、（可选）getQueueCounts；打印 path_prefix/配置路径/**cookie 路径**；**不打印 Cookie 值** | 待实跑 |
| H2 | `whoami` 或 `auth status` 身份可读 | Critical | 父代理要求 whoami；architecture `auth status` | `whoami` 若存在则跑；否则 `auth status` | 脱敏身份/会话元数据；无明文密钥 | 待实跑 |
| H3 | README.zh-CN 30 秒上手 | Critical | yunxiao `README.zh-CN.md`「30 秒快速开始」；architecture 命令树 | 打开 README.zh-CN，最短路径 | 含安装/build → `auth login` → `doctor`/`whoami` 或 `auth status`；写操作提示先 `--dry-run`；高风险说明 `--yes` / `--admin-password` | 待实跑 |
| H4 | 根 `--help` Agent 导向 | Important | yunxiao root Long `AGENT QUICKSTART`；architecture Risk 约定 | `plumelog --help` | 说明 dry-run、高风险确认后再 `--yes`、删除类另需 admin 密码、`api` 示例 | 待实跑 |

---

## 7. 与 yunxiao-cli 对齐的补充项

| ID | 验收项 | 分级 | 对照依据 | 验收步骤 | 通过标准 | 状态 |
|----|--------|------|----------|----------|----------|------|
| X1 | Agent 可靠 `--help` 渐进发现 | Important | yunxiao AGENT QUICKSTART | 根与子命令 `--help` | 能发现 auth/query/admin/api 与 Risk | 待实跑 |
| X2 | Risk 三级与帮助一致 | Important | architecture：read \| write \| high-risk-write | 抽查 query（read）、普通写、admin 删除（high-risk） | 帮助与真实门禁一致 | 待实跑 |
| X3 | 确认门禁出口约定 | Important | yunxiao exit 10 / confirmation_required；architecture「缺 --yes 退出码非 0」 | 触发 G1 | 稳定非 0；若有 JSON 信封则 subtype 可脚本依赖 | 待实跑 |
| X4 | 输出禁止泄露密钥 | Critical | architecture 文首禁止项；yunxiao MaskToken | 扫 login/status/doctor/dry-run/错误信息 | 无完整 cookie、登录密码、adminPassWord | 待实跑 |
| X5 | 全局 flags 齐套 | Important | architecture §2 | `--base-url`/`--format`/`--jq`/`--dry-run`/`--yes`/`--admin-password`/`--cookie-jar`/`--config`/`--timeout`/`--path-prefix` | 均存在或文档标明未实现阶段（P0/P1/P2） | 待实跑 |
| X6 | 可选 smoke 清单 | Minor | yunxiao `docs/smoke-checklist.md` | 源码齐后可另写 | 非发版阻塞 | 待实跑 |

---

## 实跑记录（源码齐后填写）

| 轮次 | 日期 | 执行人 | 环境 | 结果摘要 | 缺陷编号 |
|------|------|--------|------|----------|----------|
| — | — | — | — | 源码未齐，未实跑 | — |

### 缺陷列表

- （无）

---

## 当前结论

**清单已按 `architecture.md` 定稿落盘；门禁以「高风险须 `--yes`；`admin queue clear` / `admin index delete` 另须 `--admin-password`」为准；Cookie jar 默认 `%APPDATA%\plumelog\cookies.txt`（Windows）/ `$XDG_CONFIG_HOME/plumelog/cookies.txt` 或 `~/.config/plumelog/cookies.txt`。源码未齐无法实跑；缺陷：无。**

### 对照锚点摘要

| 主题 | architecture / yunxiao |
|------|------------------------|
| 命令树 / 门禁表 | architecture §1、§7 |
| Cookie jar | architecture §2「Cookie jar 路径约定」→ `cookies.txt` |
| 认证 | architecture §3；yunxiao `cmd/auth.go`（模式） |
| dry-run / --yes | architecture §2；yunxiao `cmd/root.go`、`helpers.go`、`internal/risk` |
| api | architecture §8；yunxiao `cmd/api.go` |
| doctor | architecture §9；yunxiao `cmd/doctor.go` |
| 30 秒上手 | yunxiao `README.zh-CN.md`（plumelog 应对齐另写） |

父代理 6 类验收均已覆盖（B/A/D/G/E/H）；X 为对齐补项。G 节命令名已与 architecture 对齐为 `admin queue clear` / `admin index delete`。
