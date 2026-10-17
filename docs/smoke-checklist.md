# plumelog-cli smoke checklist

发版 / 本机构建后的短手跑清单。详细验收见 `docs/qa-checklist.md`。  
**禁止**在记录中粘贴密码或 Cookie 值。

默认二进制：仓库根 `plumelog` / `plumelog.exe`。

| # | 步骤 | 期望 |
|---|------|------|
| 1 | `plumelog version` | `ok: true`，有 version |
| 2 | `plumelog doctor` | 连通性/路径可读；无 Cookie 明文 |
| 3 | `plumelog auth status`（可选 `--probe`） | `has_cookie` 元数据；无 Cookie 值 |
| 4 | `plumelog query search --app demo --level ERROR -q smoke --dry-run` | `dry_run: true`，不发网 |
| 5 | `plumelog admin queue clear --dry-run` | 预览且 `adminPassWord` 打码；不发网 |
| 6 | `plumelog admin queue clear`（无 `--yes`） | `ok: false`，exit **10**，stderr 对象含 `confirmation_required` |
| 7 | `plumelog api POST /getRunModel --dry-run` | 预览成功 |
| 8 | （若已装 skills）打开 `skills/plumelog-shared/SKILL.md` | 文件存在；或日后 `plumelog skills list` |

可选真实只读（需已 login，指向非破坏环境）：

| # | 步骤 | 期望 |
|---|------|------|
| A | `plumelog auth login`（密码来自 env，勿入日志） | `ok: true`，jar 写入 |
| B | `plumelog query apps` 或 `query search --size 1` | `ok: true` |

高风险写（clear/delete）**不要**纳入常规 smoke；仅在专用演练且用户确认后带 `--yes`。