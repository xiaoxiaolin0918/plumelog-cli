# AGENTS.md — plumelog-cli

本机落地约定：独立 **Go + cobra** CLI 仓，二进制名 **`plumelog`**。父目录 `D:\work\ops` 是 Python 运维工作区；**本目录是独立 git 仓**（GitHub：`xiaoxiaolin0918/plumelog-cli`），不要按 ops 的 Python/`tools/` 惯例往这里塞脚本。

## 角色与边界

| 角色 | 职责 |
|------|------|
| 本机实现 | 日常改码、测试、开 PR、发版准备；维护本文件与 README |
| 架构契约 | [`docs/architecture.md`](docs/architecture.md)；人类/Agent 短文在 [`docs/wiki/`](docs/wiki/README.md) |
| 云端 agent | 仅作辅助；**本仓 Cloud Agent 常不可达**，以本机 `gh` / git 为准 |

禁止：写入真实 PlumeLog 密码、Cookie、`adminPassWord`、token；把 secrets 写进 README/Issue/PR；臆造不存在的 `release.yml` / npm 工作流（当前 CI 只有 `go test`）。

## 目录形态

```
plumelog-cli/
  AGENTS.md / README.md / README.zh-CN.md / SECURITY.md / CONTRIBUTING.md
  go.mod / go.sum / main.go / Makefile / LICENSE
  cmd/                 # cobra 命令
  internal/            # 客户端、配置、鉴权、风险门、version 等
  docs/
    architecture.md    # 架构契约
    wiki/              # 仓内 wiki（非 GitHub Wiki）
    smoke-checklist.md
  skills/              # 随二进制打包的 agent skill markdown
  .github/workflows/   # ci.yml（go test）
```

与旁侧 ops 的边界：

- `D:\work\ops\tools/`、`D:\work\ops\AGENTS.md` → Python 运维，勿混入本仓
- 本仓只承载 Go CLI；安装产物可进用户 PATH，勿散落到 `%TEMP%`

## 本机构建

- Go：`D:\tools\go\bin` 加入 PATH；`GOPROXY=https://goproxy.cn,direct`
- 校验：

```powershell
$env:PATH = "D:\tools\go\bin;" + $env:PATH
$env:GOPROXY = "https://goproxy.cn,direct"
go test ./...
go build -o plumelog.exe .
.\plumelog.exe version
```

- 当前发布线：**v0.1.2**（`internal/version/version.go` 的 `Version` 必须与 tag 对齐；发版准备 PR 只 bump + docs，**不**在准备分支上打 tag / 建 GitHub Release）

## 架构与命令面（摘要）

命令组：`auth` / `query` / `trace` / `app` / `alarm` / `admin` / `api` / `skills` / `doctor` / `config` / `version` / `whoami`。

- 鉴权：Servlet Session Cookie（常见 `JSESSIONID`），非 Bearer。`auth login --web` 仅监听 `127.0.0.1`；可选记住密码 → 加密 `login.cred`（从不进 `config.json` 明文）
- 高风险写：须 `--yes`；部分还需 `--admin-password` / `PLUMELOG_ADMIN_PASSWORD`
- 细节以 `docs/architecture.md` 与 `docs/wiki/` 为准

## 工作风格

- 用户：肖晓霖；中文简报；少问确认；破坏性操作先确认
- **TDD**：能测的先写/改测试再实现；改完跑 `go test ./...`
- 非 trivial 大改可开 PR；发版：bump `Version` → README/SECURITY/wiki 链接 → 合 main 后再打 `vX.Y.Z` tag

## 安全提醒

- 不要提交 `*_cookies.txt`、`.env`、`login.cred`、`login.key`
- 文档与示例只用占位主机（如 `http://127.0.0.1:8891`）与 `$PLUMELOG_LOGIN_PASSWORD` 一类环境变量名
