# plumelog

[![CI](https://github.com/xiaoxiaolin0918/plumelog-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/xiaoxiaolin0918/plumelog-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/xiaoxiaolin0918/plumelog-cli?include_prereleases)](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.2)

[PlumeLog](https://github.com/fayechenlong/plumelog) 服务的命令行客户端。默认端口 **8891**。

二进制名 **`plumelog`**。许可证：[MIT](LICENSE)。英文说明：[README.md](README.md)。

PlumeLog 使用和网页一样的 Session Cookie（常见名 `JSESSIONID`），不是 Bearer Token。CLI 不会在输出、日志或示例里打印密码或 Cookie 值。

## 安装

需要 Go **1.22** 或更高版本。

**从源码（推荐贡献者）：**

```bash
git clone https://github.com/xiaoxiaolin0918/plumelog-cli.git
cd plumelog-cli
go test ./...
go build -o plumelog .
./plumelog --help
./plumelog version
```

**Windows（PowerShell）：**

```powershell
go build -o plumelog.exe .
go test ./...
.\plumelog.exe --help
```

**Go install：**

```bash
go install github.com/xiaoxiaolin0918/plumelog-cli@v0.1.2
```

发布说明：[v0.1.2](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.2)。


## 0.1.2 更新摘要

- 中文 `auth login --web`：表单含 **服务地址**（base URL）与可选 **记住并自动登录**（加密写入 `login.cred`；Windows 用 DPAPI，其他平台 AES-GCM）
- 静默重登凭据成对取值；静默登录遇 `402` 会清除 `login.cred`
- 相对 v0.1.1 的文档/wiki 整理（仓内 wiki 链接、去掉 BOM）
- 版本号对齐为 **0.1.2**（此前 main 上仍显示 0.1.0）

## 安全（先读）

- **不要提交** Cookie、密码、`.env` 或生产环境凭据。
- 优先用环境变量（`PLUMELOG_LOGIN_PASSWORD`、`PLUMELOG_ADMIN_PASSWORD`），避免在共享 shell/历史里留下明文。
- 文档与 Issue 里用占位主机名（例如 `http://127.0.0.1:8891`）。
- Cookie 落在系统配置目录、权限 `0600`，且已被 gitignore（`*_cookies.txt`、`.env`）。
- 漏洞请按 [SECURITY.md](SECURITY.md) 私下报告。

## 30 秒上手

```bash
export PLUMELOG_BASE_URL=http://127.0.0.1:8891
export PLUMELOG_LOGIN_PASSWORD='你的登录密码'
export PLUMELOG_ADMIN_PASSWORD='你的管理密码'

./plumelog doctor
./plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
./plumelog auth status
./plumelog whoami
./plumelog query search --app demo --level ERROR -q timeout
./plumelog trace get --trace-id TRACE_ID
./plumelog api POST /getRunModel
```

Windows PowerShell 可以把环境变量写成 `$env:PLUMELOG_LOGIN_PASSWORD = '你的登录密码'`。

## 命令树

```
plumelog
  auth login | logout | status          # 登录 / 登出 / 是否已有 Cookie
  doctor                                  # 连通性、会话、getRunModel、队列
  config get | set | path | show          # 配置；show 会脱敏
  query search | dsl | apps               # 快捷查询、原始 DSL、应用名
  trace get                               # trace=true 下按 traceId 查链路
  app list | extend list | add | del      # 应用名与扩展字段
  alarm list | save | delete | messages   # 告警规则与监控消息
  admin queue counts | clear              # 队列深度 / 清空
  admin index info | delete | reset       # 索引
  admin run-model | delete-by-query
  api <METHOD> <path>                     # 原始 HTTP
  skills list | path | read | install | update
  version
  whoami
```

`--help` 会标注 Risk：`read`、`write`、`high-risk-write`。`api` 的风险由路径决定。

## 鉴权

```bash
./plumelog auth login --username admin --password "$PLUMELOG_LOGIN_PASSWORD"
./plumelog auth login --web          # 本机 127.0.0.1 浏览器表单（别名 --browser）；已有会话加 --force
./plumelog auth status
./plumelog auth status --probe
./plumelog whoami
./plumelog auth logout
```

`POST /login`，body 为 `{"username","password"}`。`402` 表示凭据无效。服务端未配置 `login.username` 时，空的 `auth login` 仍会 POST 并记录 `auth: open`。

`auth login --web` 中文表单含 **服务地址**（可由 `--base-url` / 环境变量 / 配置预填）与可选 **记住并自动登录**（密码加密存为配置旁的 `login.cred`，不会明文写入 `config.json`）。详见 [鉴权 Auth](docs/wiki/Auth.md)。

`auth status` 只输出 `has_cookie`、`cookie_path`、`base_url`，**从不**打印 Cookie 值。

## 查询示例

```bash
./plumelog query search --app demo --level ERROR -q timeout \
  --from-time 2024-01-02T00:00:00Z --to-time 2024-01-02T01:00:00Z

./plumelog query search --dsl-file query.json \
  --from-time 1700000000000 --to-time 1700003600000

./plumelog query apps --source es
./plumelog trace get --trace-id TRACE_ID --size 100
```

快捷条件与 Home.vue 一致：`appName` / `env` / `logLevel` / `traceId` / `className` / `serverName` 用 `match_phrase`，正文用 `query_string`，`--extend field=value` 可重复，时间打在 `dtTime` 上。`--exclude-app` 把应用放到 `must_not`。

默认排序 `dtTime desc`。带了 `--trace-id` 则改为 `dtTime asc` + `seq asc`。高亮默认打开，`--no-highlight` 关闭。

不写时间时，索引用最近 `default_time_window`（默认 1 小时）。`--to-time` 若在未来会被夹到当前时间。

`--index` 或 `--range day|week|month` 会改走 `POST /query`。运行日志索引形如 `plume_log_run_YYYYMMDD`，链路索引形如 `plume_log_trace_YYYYMMDD`。

仅 `--index`、无 `--from-time` / `--to-time` / `--range` 时，CLI 按索引名中的 `YYYYMMDD` 自动对齐全天时间窗（避免历史索引静默 0 条）；`query dsl --index` 同样会改写 body 里的 `dtTime`。`--range` 相对「现在」；历史索引请用显式时间。

## Skills（Agent 配套）

```bash
./plumelog skills list
./plumelog skills list --check
./plumelog skills install
./plumelog skills update
```

## 逃生舱

```bash
./plumelog api POST /clientQuery \
  --query size=20 --query from=0 \
  --body-file query.json

./plumelog api POST /deleteQueue --dry-run
```

URL = `base_url + path_prefix + path`。`--path-prefix` 默认 `auto`：先试 `""`，404 再试 `/plumelog`（若配置里写了 `/plumelog` 则先试它）。也可以设成 `root` 或 `/plumelog`。

## 风险开关

- **read**：直接执行。
- **write**：直接执行。`--dry-run` 只打印请求。
- **high-risk-write**：真正执行必须带 `--yes`（或 `-y`）。

清空队列和删除索引还需 `--admin-password`（或环境变量 `PLUMELOG_ADMIN_PASSWORD`）。CLI 把它放进查询参数 `adminPassWord`，输出里永远打码。

同样需要 `--yes` 的命令：`admin index reset`、`admin delete-by-query`、`alarm delete`、`app extend del`。上游的 `resetIndex` 与 `deleteByQuery` 并不校验管理密码，所以 CLI 不会把密码发到这两个接口。

`--dry-run` 即使同时写了 `--yes` 也不会发请求。预览里有 Method、URL、打码后的 Header 与 Body。缺的确认项在 `would_require`。

成功时 **stdout** JSON 信封：`{"ok":true,"result":...}`。失败写 **stderr**，且 `error` 是**对象**（含 `type`/`message`，门禁时还有 `subtype=confirmation_required`）。缺 `--yes` 时 **exit 10**。`--jq .result` 可对整份信封过滤。`--format` 支持 `json`（默认）、`pretty`、`table`。

## 配置与 Cookie

| | Unix | Windows |
|--|------|---------|
| 目录 | `$XDG_CONFIG_HOME/plumelog` 或 `~/.config/plumelog` | `%APPDATA%\plumelog` |
| 配置 | `config.json`（权限 `0600`） | 同名文件 |
| Cookie | `cookies.txt`（Netscape 格式，权限 `0600`） | 同名文件 |

```bash
./plumelog config show
./plumelog config set base_url http://127.0.0.1:8891
./plumelog config set path_prefix root
```

`config set` 拒绝写入密码。请用参数或环境变量：

- `PLUMELOG_BASE_URL`（默认 `http://localhost:8891`，不要带末尾斜杠）
- `PLUMELOG_PATH_PREFIX`
- `PLUMELOG_LOGIN_USERNAME` / `PLUMELOG_LOGIN_PASSWORD`
- `PLUMELOG_ADMIN_PASSWORD`

优先级：命令行 > 环境变量 > `config.json` > 默认值。HTTP 超时默认 30 秒（`--timeout`）。

## 文档

面向人类与 AI 的说明写在**本仓库** [docs/wiki/](docs/wiki/README.md)（入口 [Home.zh-CN](docs/wiki/Home.zh-CN.md) / [Home](docs/wiki/Home.md)），**不是** GitHub Wiki。

- [快速开始](docs/wiki/Getting-Started.md)
- [鉴权 Auth](docs/wiki/Auth.md)（Session Cookie、`auth login --web`）
- [Skills for AI](docs/wiki/Skills-for-AI.md)
- [架构摘要](docs/wiki/Architecture.md) → 完整契约：[docs/architecture.md](docs/architecture.md)

冒烟清单：[docs/smoke-checklist.md](docs/smoke-checklist.md)。

## 贡献

见 [CONTRIBUTING.md](CONTRIBUTING.md)。

## v0 不做

实时控制台 WebSocket（`ws://<host>/plumelog/websocket`）留给以后。看滚动日志请用网页。
