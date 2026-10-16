# plumelog-cli skills

Agent 技能包，模式对齐 `yunxiao-cli/skills`：先读 **plumelog-shared**，再按域加载。

| Skill | 目录 | 用途 |
|-------|------|------|
| plumelog-shared | `plumelog-shared/` | 鉴权、config、doctor、信封、门禁、脱敏、全局 flag |
| plumelog-query | `plumelog-query/` | `query search\|dsl\|apps` |
| plumelog-trace | `plumelog-trace/` | `trace get` |
| plumelog-app | `plumelog-app/` | `app list` / `extend *` |
| plumelog-alarm | `plumelog-alarm/` | `alarm *` |
| plumelog-admin | `plumelog-admin/` | `admin *`（高风险单独，勿与只读域合并） |
| plumelog-api | `plumelog-api/` | `api METHOD path` 逃生舱 |

小仓可合并 query+trace、app+alarm；**admin 保持独立**。

## 安装

```bash
# 列出 / 读取 / 路径
plumelog skills list
plumelog skills read plumelog-shared
plumelog skills path
plumelog skills path plumelog-admin

# 安装到 Agent skills 根（默认 ~/.agents/skills；Windows: %USERPROFILE%\.agents\skills）
plumelog skills install
plumelog skills install --force
plumelog skills install --symlink
plumelog skills install --skill plumelog-shared --skill plumelog-query
```

源：二进制旁或源码树中的 `skills/plumelog-*`（须含 `SKILL.md`）。需要二进制：`go build -o plumelog.exe .`
## 相关文档

- `docs/gap-vs-yunxiao.md` — 相对 yunxiao 的完善清单（P0/P1/P2）
- `docs/smoke-checklist.md` — 发版后短 smoke
- `docs/architecture.md` — 契约