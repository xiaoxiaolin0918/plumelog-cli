---
name: plumelog-app
version: 0.1.1
description: "PlumeLog application names and per-app extend/MDC fields. extend del is high-risk-write (needs --yes)."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog app --help"
---

# app（应用名与扩展字段）

开始前先读 [`../plumelog-shared/SKILL.md`](../plumelog-shared/SKILL.md)。

| 命令 | Risk |
|------|------|
| `app list` | read（同 `query apps`） |
| `app extend list` | read |
| `app extend add` | write |
| `app extend del` | **high-risk-write**（须 `--yes`） |

```bash
plumelog app list
plumelog app list --source es
plumelog app extend list --app demo
plumelog app extend add --app demo --field userId --field-name "user id"
plumelog app extend del --dry-run --app demo --field userId
# 用户确认后：
plumelog app extend del --yes --app demo --field userId
```

写操作先 `--dry-run`。删扩展字段禁止静默加 `--yes`。