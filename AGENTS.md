# AGENTS.md — plumelog-cli

本机落地约定：独立 Go + cobra CLI 子仓，二进制名 **`plumelog`**。父目录 `D:\work\ops` 是 Python 运维工作区；**本目录是独立 git 仓**，不要按 ops 的 Python/`tools/` 惯例往这里塞脚本。

## 角色分工

| 角色 | 职责 |
|------|------|
| 云端 coding agent | 非 trivial 脚手架与大段写码（当前：https://cursor.com/agents/bc-a3980ac1-1b45-5f18-9b8d-ab697ad21fa5 ，Origin `new_repo`，未 publish 前无稳定 clone URL） |
| 架构小弟 | 契约 `docs/architecture.md`（命令组、全局 flag、路径映射、风险门） |
| 本机实现 | 同步云端产物、构建验证、小修；维护本文件与 README/构建说明 |

禁止：手改 `.placeholder` 等脚手架占位；在未定稿契约上臆造命令面；写入 token / AccessKey / 密码。

## 目录约定（落地后目标形态）

对齐参考仓 `D:\work\grokbot\yunxiao-cli`：

```
plumelog-cli/
  AGENTS.md          # 本文件
  README.md / README.zh-CN.md
  go.mod / go.sum
  main.go
  Makefile
  cmd/               # cobra 命令
  internal/          # 客户端、配置、风险门等
  docs/
    architecture.md  # 架构契约（定稿后落盘）
  scripts/           # 构建/检查脚本（如有）
```

与旁侧 ops 的边界：

- `D:\work\ops\tools/`、`D:\work\ops\AGENTS.md` → Python 运维工具，勿混入本仓
- 本仓只承载 Go CLI；安装产物可进用户 PATH（参考 yunxiao 的 `%USERPROFILE%\.local\bin`），勿散落到 `%TEMP%`

## 脚手架与同步（先别瞎克隆）

当前状态：仅有 `.placeholder`（文案要求等云端脚手架），**无 git**。

准备步骤（云端 publish / 给出可 clone URL 后再执行；URL 以父代理或云端产物为准）：

1. **确认远程**：拿到 Origin/GitHub 的 clone URL 或已 publish 的 repo 名；未知 URL 不克隆。
2. **备份占位**：若目录非空，先把 `.placeholder` / 本 `AGENTS.md` 拷到临时目录（避免被 `git clone` 拒绝或覆盖丢约定）。
3. **git 初始化方式（二选一，优先 A）**  
   - **A)** 空目录或清掉非约定文件后：`git clone <url> D:\work\ops\plumelog-cli`（若目录名冲突，先 clone 到旁路目录再 `robocopy`/`Move-Item` 合并）。  
   - **B)** 已有本文件时：`git init` → `git remote add origin <url>` → `git fetch` → `git checkout -B main origin/main`（以远程默认分支为准）。
4. **合并约定**：若远程尚无 `AGENTS.md`，把本机这份拷回仓根并提交（或开 PR）；保留架构侧 `docs/architecture.md`。
5. **构建验证**：本机需 Go（见下）；`make build` 或 `go build -o plumelog.exe .`，再 `.\plumelog.exe --help` / `--version`。
6. **通知**：同步+构建结果回父代理；命令面缺口对照 `docs/architecture.md` 回父代理。

## 构建前提（本机）

- 参考仓 Go：`yunxiao-cli` 的 `go.mod` 为 **go 1.24.x**；本仓以落地后的 `go.mod` 为准。
- 截至开局检查：**iipcloud-xxl 上 `go` 不在 PATH**，常见路径亦未找到。脚手架同步后需先安装/配置 Go 再做构建验证。
- 禁止把真实 PlumeLog `adminPassWord`、cookie、token 写进仓或本文件。

## 架构契约（待定稿）

架构侧将产出 `docs/architecture.md`，预期覆盖：

- 命令组：`auth` / `query` / `trace` / `app` / `alarm` / `admin` / `api`
- 全局 flag；双前缀路径映射；Home.vue DSL 快捷过滤
- 高风险操作：须 `--yes` + `adminPassWord`

**定稿前**：只做目录约定与同步准备，不手写命令实现。定稿后对照命令面是否可落地，缺口回父代理。

## 工作风格

- 用户肖晓霖；中文简报；少问确认；破坏性操作先确认。
- 非 trivial 写码 → 云端 agent；本机 → 同步、构建验证、小修。

## ??????????

- Go 1.24.4 @ `D:\tools\go\bin\go.exe`??? PATH ????
- ?? `GOPROXY=https://goproxy.cn,direct`
