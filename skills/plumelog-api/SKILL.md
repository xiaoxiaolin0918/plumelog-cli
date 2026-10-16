---
name: plumelog-api
version: 0.1.0
description: "PlumeLog raw HTTP escape hatch: api METHOD path with path-prefix auto, --body/--body-file/--query/--header. Destructive paths are high-risk-write."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog api --help"
---

# api（逃生舱）

开始前先读 [`../plumelog-shared/SKILL.md`](../plumelog-shared/SKILL.md)。

有 typed 命令时优先 typed（`query` / `admin` / …）；typed 不够再用 `api`。

```bash
plumelog api POST /getRunModel
plumelog api POST /clientQuery --query size=20 --query from=0 --body-file query.json
plumelog api POST /deleteQueue --dry-run
```

- path 相对 `--base-url` + `--path-prefix`；若已以 `/plumelog/` 开头则保留
- path 上的 query string 会与 `--query key=value`（可重复）合并
- `--body` / `--body-file`；`--header "Name: value"`（dry-run 脱敏 Cookie/Authorization）

## 风险由路径决定

已知破坏路径为 **high-risk-write**，须 `--yes`；其中 `deleteQueue` / `deleteIndex` 还须 `--admin-password`：

`deleteQueue`、`deleteIndex`、`resetIndex`、`deleteByQuery`、`deleteWarningRule`、`delExtendfield`

非 POST 方法不被登录拦截器检查，但 Cookie jar 仍会附带。

## Windows MSYS

git-bash 可能把 `/clientQuery` 改成 Windows 路径。用 PowerShell，或设 `MSYS_NO_PATHCONV=1`。