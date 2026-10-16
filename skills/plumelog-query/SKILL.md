---
name: plumelog-query
version: 0.1.0
description: "PlumeLog log search: query search/dsl/apps with Home.vue shortcut filters, raw DSL, time window, highlight/sort. Risk: read."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog query --help"
---

# query（日志检索）

开始前先读 [`../plumelog-shared/SKILL.md`](../plumelog-shared/SKILL.md)。

链路检索见 [`../plumelog-trace/SKILL.md`](../plumelog-trace/SKILL.md)。

## 命令

| 命令 | 说明 | Risk |
|------|------|------|
| `query search` | 快捷过滤 → ES bool，默认 `POST /clientQuery` | read |
| `query dsl` | 原始 DSL（`--file` / stdin / `--dsl`） | read |
| `query apps` | 应用名：`cache`→`/getAppNames`，`es`→`/queryAppNames` | read |

```bash
plumelog query search --app demo --level ERROR -q timeout
plumelog query search --trace-id TRACE --from-time 1700000000000 --to-time 1700003600000
plumelog query search --dsl-file query.json --from-time 1700000000000 --to-time 1700003600000
plumelog query search --index plume_log_run_20240101 --range day --app demo
plumelog query apps
plumelog query apps --source es
plumelog query dsl --file query.json
```

## 快捷过滤（对齐 Home.vue）

| Flag | ES |
|------|-----|
| `--app` / `--app-name` | `match_phrase` `appName`（逗号变空格）；`--exclude-app` → `must_not` |
| `--env` | `match_phrase` `env` |
| `--level` / `--log-level` | `match_phrase` `logLevel` |
| `--trace-id` | `match_phrase` `traceId`；排序改 `dtTime asc, seq asc` |
| `--class` / `--class-name` | `match_phrase` `className` |
| `--server` / `--server-name` | `match_phrase` `serverName` |
| `-q` / `--content` | `query_string` on `content` |
| `--extend field=value`（可重复） | `match_phrase` 扩展/MDC 字段 |
| `--from-time` / `--to-time` | `range` on `dtTime` + clientStart/EndDate；未来 `to` 夹到 now |
| `--size` / `--from` | 分页（默认 size 100 或 config `default_size`） |
| `--sort` | 覆盖排序；默认 `dtTime desc` |
| `--no-highlight` | 关闭 content highlight |
| `--dsl` / `--dsl-file` | 透传 body；与快捷 flag 互斥 |
| `--index` / `--range day\|week\|month` | 改走 `POST /query` |

未写时间时用 `default_time_window`（默认 1h）选日索引。运行日志索引形如 `plume_log_run_YYYYMMDD`。

成功看 `ok` + `result`（ES 响应常在 `result.hits`）。可用 `--jq .result.hits.hits`。