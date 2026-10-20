# plumelog-cli 验收清单（现行）

> 更新：2026-10-20（Asia/Shanghai）· CLI **v0.1.4**  
> **禁止**在记录中粘贴密码、Cookie、adminPassWord、生产主机+凭据组合。

本文件是**现行**验收入口。早期「源码未齐 / scaffold」长表已过时，不再作为发版门禁。

## 对照文档

| 用途 | 路径 |
|------|------|
| 架构与契约 | `docs/architecture.md` |
| 发版短手跑 | `docs/smoke-checklist.md` |
| 相对 yunxiao 缺口 | `docs/gap-vs-yunxiao.md` |
| Agent 技能 | `skills/plumelog-shared/SKILL.md` 等 |
| 历史实跑笔记 | `docs/qa-run-*.md`（本地 gitignore；勿提交密钥） |

## 0. 每次发版必跑（Critical）

| ID | 项 | 步骤 | 通过标准 |
|----|----|------|----------|
| B1 | 单测 | `go test ./...` | 全绿 |
| B2 | 构建 | `go build -o plumelog.exe .`（或 CI） | 退出码 0 |
| B3 | 版本对齐 | `plumelog version` 与 `internal/version` / tag / SECURITY / AGENTS 一致 | 同号（如 0.1.4） |
| S1 | smoke | 按 `docs/smoke-checklist.md` 1–8 | 期望列全部满足 |
| R1 | 高风险门禁 | `admin queue clear` 无 `--yes` | exit **10**，`confirmation_required`，不发网 |
| R2 | dry-run | 任意写命令加 `--dry-run` | `dry_run: true`，transport 0 |
| X1 | 输出脱敏 | login / status / doctor / dry-run / 错误 | 无完整 Cookie、登录密码、admin 密码 |

## 1. 功能回归（Important，改相关域时）

| 域 | 最小覆盖 |
|----|----------|
| auth | login / status / logout；`--auto-login` 401 重放（单测已有） |
| query | `search` / `dsl` / `apps`；多词 `-q` AND；`--range`；空 body → `ok:false` |
| trace | `trace get` 空 hits 有 hint；与通用 invoke 共用 `doWithAutoLogin` |
| alarm | `list` / `save` / `delete --yes` / `messages` |
| admin | `queue counts` 只读；clear/delete 双门禁 |
| api | `METHOD path` + `--dry-run` |
| skills | `skills list`；可选 `skills list --check` |

## 2. 配置模型（文档门禁）

- **单实例**：当前仅 `config.json` + `PLUMELOG_*` 环境变量；**无** multi-profile / `+onboard`（见 `docs/gap-vs-yunxiao.md`、`docs/architecture.md`）。
- Cookie jar：默认 OS config 目录下 `cookies.txt`；可用 `--cookie-jar` 覆盖。

## 3. 安装与 Release 资产

- 日常：`go install github.com/xiaoxiaolin0918/plumelog-cli@vX.Y.Z`
- Tag `v*` 触发 `.github/workflows/release.yml`，上传 linux/darwin/windows 多平台二进制 + checksums
- 无自更新命令；运维仓继续 `go build` / `go install` 亦可

## 4. 历史清单

旧版「待实跑 / 源码未齐」长表已废弃。若需考古，查 git 历史中本文件 2026-10-16 定稿版本；**不要**再按「待实跑」状态填表。
