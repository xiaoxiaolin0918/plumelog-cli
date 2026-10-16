---
name: plumelog-alarm
version: 0.1.0
description: "PlumeLog warning rules and monitor messages: list/save/delete/messages. delete is high-risk-write (needs --yes)."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog alarm --help"
---

# alarm（告警规则与监控消息）

开始前先读 [`../plumelog-shared/SKILL.md`](../plumelog-shared/SKILL.md)。

| 命令 | Risk |
|------|------|
| `alarm list` | read（`POST /getWarningRuleList`；null 常表示无 Redis） |
| `alarm messages` | read（`POST /query?index=plumelog_monitor_message_key`） |
| `alarm save` | write |
| `alarm delete` | **high-risk-write**（须 `--yes`） |

```bash
plumelog alarm list
plumelog alarm messages --size 20
plumelog alarm save --id rule-1 --app demo --error-count 5 --rule-time 1 --webhook-url https://example.invalid/hook
plumelog alarm save --dry-run --id rule-1 --file rule.json
plumelog alarm delete --dry-run --id rule-1
# 用户确认后：
plumelog alarm delete --yes --id rule-1
```

- `hookServe`：`--hook-serve` 默认 1=钉钉，2=企微
- `id` 是 Redis hash 字段，不是 body 内字段
- 删除必须先 dry-run，禁止静默 `--yes`