# plumelog-cli 相对 yunxiao-cli 完善清单

> 日期：2026-10-20（更新：单实例声明 + Release 二进制）（Asia/Shanghai）  
> 对照：`D:\work\grokbot\yunxiao-cli`（skills / shared 契约 / doctor 命令面）  
> 依据：本机 `plumelog --help` 与子命令、实测门禁、`docs/architecture.md`、`docs/qa-run-*.md`、架构简报 `/workspace/plumelog-skills-gap.md`  
> **禁止**在本文写入真实密码 / Cookie。

状态约定：✅ 已有 · ⚠️ 部分 · ❌ 缺口

## 命令面快照

| 能力 | yunxiao | plumelog |
|------|---------|----------|
| auth login/status/logout | ✅ | ✅ |
| doctor | ✅ | ✅ |
| whoami | ✅ | ✅ 根级别名（脱敏字段） |
| config get/set/path/show | ✅ | ✅ |
| `--dry-run` / `--jq` / `--yes` | ✅ | ✅ |
| confirmation_required + exit 10 | ✅ | ✅ exit **10** + `error.subtype=confirmation_required` |
| profile / +onboard | ✅ | ❌ 单实例 + env/config（**明确不做** onboard） |
| skills list/install + 随包 | ✅ | ✅ `plumelog skills list|read|path|install` |
| update / self-update | ✅ | ❌（`go build`） |
| smoke-checklist.md | ✅ | ✅（本轮新增短页） |
| 域命令 + Risk 标注 | ✅ | ✅ |
| 高风险双门禁（admin 密码） | N/A | ✅ |

## 信封差异（Agent 必读）

| | yunxiao | plumelog（现状） |
|--|---------|------------------|
| 成功 | `{ok, data, meta}` | `{ok, result}` |
| 失败 `error` | 对象（type/subtype/…），常 stderr | **对象**（type/subtype/…），**stderr** |
| 缺 `--yes` | exit **10**，`subtype=confirmation_required` | exit **10** + `confirmation_required`（stderr） |

判断成功：看 **`ok`**，不要看上游 `code`。

---

## P0（先做 / 已部分完成）

- [x] **skills 底座**：`skills/plumelog-shared` + `references/output-and-risk.md`；域 skill：query / trace / app / alarm / admin / api；`skills/README.md`
- [x] **skill 写清门禁现状**：失败在 **stderr**；`error` 对象化；缺 `--yes` → exit **10** + `confirmation_required`（见 shared / output-and-risk）
- [x] **确认门禁对齐**：推荐改 CLI → exit **10** + `error: {type, subtype: confirmation_required, hint, action, risk}`，便于 Agent 稳定分支；或长期在 skill 声明「勿按 yunxiao 解析」（已改 CLI 为 exit 10 + confirmation_required）
- [x] **生产 URL / 凭据映射约定入 shared**：`http://plumelog.iipcloud.cn`；zhiyi-ops `.env` 的 `PLUMELOG_URL|USER|PASSWORD` → `PLUMELOG_BASE_URL|LOGIN_USERNAME|LOGIN_PASSWORD`（不落明文）

## P1

- [x] **`plumelog skills list|read|path|install|update`**：install/update → `~/.agents/skills`；源可为磁盘 `skills/` 或二进制 embed；`list --check` 比对 frontmatter version
- [x] **`docs/smoke-checklist.md`**：从 qa 裁短的发版手跑页（本轮已加）
- [x] **GitHub 正式仓发布**：PRIVATE https://github.com/xiaoxiaolin0918/plumelog-cli（勿提交 cookies/exe/密钥）
- [x] **api MSYS unmangle**：CLI 自动还原 git-bash 改写的路径（PLUMELOG_API_NO_UNMANGLE=1 可关）；skill 已同步

## P2

- [x] **根级 `whoami`**：别名封装 `auth status`（base_url / authenticated / has_cookie / cookie_path / path_prefix）
- [x] **multi-profile**：文档声明「单实例 + env/config，不做 +onboard」（README / architecture / 本文）——**不实现**
- [ ] **`plumelog update` / self-update**：仍无；**已有** tag 触发的多平台 Release 资产（`.github/workflows/release.yml`）+ `go install`
- [ ] **alias 子系统**（禁止嵌入 `--yes`）— 低优
- [ ] **Agent 非 TTY 环境变量**（`PLUMELOG_NO_TUI` 等）— 默认已是 JSON，影响小
- [x] 质检 Minor 已记：门禁 JSON 有 subtype；已有 whoami；已有 smoke 短页

## 已对齐、无需再开债

- Cookie jar 路径与脱敏、`config` 拒密码、`--path-prefix` auto 双前缀
- Risk 三级 + admin 双门禁实测生效；`--dry-run` 零网络且脱敏
- `doctor` / `api` 逃生舱 / README.zh-CN 30 秒上手
- 真实 login E2E PASS（见 `docs/qa-run-e2e-20261016.md`）

## 合并备选

小仓可将 **query+trace**、**app+alarm** 合并；**admin 勿并入只读域**，避免 Agent 误加载高风险面。
