# plumelog-cli 相对 yunxiao-cli 完善清单

> 日期：2026-10-16（Asia/Shanghai）  
> 对照：`D:\work\grokbot\yunxiao-cli`（skills / shared 契约 / doctor 命令面）  
> 依据：本机 `plumelog --help` 与子命令、实测门禁、`docs/architecture.md`、`docs/qa-run-*.md`、架构简报 `/workspace/plumelog-skills-gap.md`  
> **禁止**在本文写入真实密码 / Cookie。

状态约定：✅ 已有 · ⚠️ 部分 · ❌ 缺口

## 命令面快照

| 能力 | yunxiao | plumelog |
|------|---------|----------|
| auth login/status/logout | ✅ | ✅ |
| doctor | ✅ | ✅ |
| whoami | ✅ | ❌（用 `auth status` / `--probe`） |
| config get/set/path/show | ✅ | ✅ |
| `--dry-run` / `--jq` / `--yes` | ✅ | ✅ |
| confirmation_required + exit 10 | ✅ | ❌ exit **1** + 扁平 string `error` |
| profile / +onboard | ✅ | ❌ |
| skills list/install + 随包 | ✅ | ⚠️ 仓内已有 `skills/`；**无** CLI 子命令 |
| update / self-update | ✅ | ❌（`go build`） |
| smoke-checklist.md | ✅ | ✅（本轮新增短页） |
| 域命令 + Risk 标注 | ✅ | ✅ |
| 高风险双门禁（admin 密码） | N/A | ✅ |

## 信封差异（Agent 必读）

| | yunxiao | plumelog（现状） |
|--|---------|------------------|
| 成功 | `{ok, data, meta}` | `{ok, result}` |
| 失败 `error` | 对象（type/subtype/…），常 stderr | **string**，常 stdout |
| 缺 `--yes` | exit **10**，`subtype=confirmation_required` | exit **1**，`error` 为拒绝文案 |

判断成功：看 **`ok`**，不要看上游 `code`。

---

## P0（先做 / 已部分完成）

- [x] **skills 底座**：`skills/plumelog-shared` + `references/output-and-risk.md`；域 skill：query / trace / app / alarm / admin / api；`skills/README.md`
- [x] **skill 写清门禁现状**：exit 1 + 扁平 string；明确尚未对齐 yunxiao exit 10 + `confirmation_required`（见 shared）
- [ ] **（产品债，二选一）对齐 confirmation 信封**：推荐改 CLI → exit **10** + `error: {type, subtype: confirmation_required, hint, action, risk}`，便于 Agent 稳定分支；或长期在 skill 声明「勿按 yunxiao 解析」（当前采用后者文档，前者待改代码）
- [x] **生产 URL / 凭据映射约定入 shared**：`http://plumelog.iipcloud.cn`；zhiyi-ops `.env` 的 `PLUMELOG_URL|USER|PASSWORD` → `PLUMELOG_BASE_URL|LOGIN_USERNAME|LOGIN_PASSWORD`（不落明文）

## P1

- [ ] **`plumelog skills list|read|path|install`**：install → `~/.agents/skills`（copy/symlink，`--force`）；随发布物带上 `skills/`
- [x] **`docs/smoke-checklist.md`**：从 qa 裁短的发版手跑页（本轮已加）
- [ ] **GitHub 正式仓发布**：可见性按用户选定（PRIVATE/INTERNAL）；勿提交 `_e2e_cookies.txt` / 密钥 / 本地 exe（见 `.gitignore`）
- [ ] **api skill 已写的 MSYS 注意**：若 Windows Agent 常用 `api`，可考虑 CLI 侧 unmangle（与 yunxiao 对齐）

## P2

- [ ] **根级 `whoami`**：别名封装 `auth status`（脱敏 base_url / has_cookie / cookie_path / path_prefix）；短期内 skill 继续用 auth status
- [ ] **轻量 multi-profile** 或文档声明「单实例用 env/config，不做 +onboard」
- [ ] **`plumelog update` / self-update**：含旁路刷新 `skills/`（运维仓可暂用 `go build`）
- [ ] **alias 子系统**（禁止嵌入 `--yes`）— 低优
- [ ] **Agent 非 TTY 环境变量**（`PLUMELOG_NO_TUI` 等）— 默认已是 JSON，影响小
- [ ] 质检 Minor 已记：门禁 JSON 无 subtype；无独立 whoami；原无 smoke（现已补短页）

## 已对齐、无需再开债

- Cookie jar 路径与脱敏、`config` 拒密码、`--path-prefix` auto 双前缀
- Risk 三级 + admin 双门禁实测生效；`--dry-run` 零网络且脱敏
- `doctor` / `api` 逃生舱 / README.zh-CN 30 秒上手
- 真实 login E2E PASS（见 `docs/qa-run-e2e-20261016.md`）

## 合并备选

小仓可将 **query+trace**、**app+alarm** 合并；**admin 勿并入只读域**，避免 Agent 误加载高风险面。