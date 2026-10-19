---
name: plumelog-query
version: 0.1.4
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
plumelog query search --index plume_log_run_20240101 --app demo
plumelog query search --index plume_log_run_20240101 --from-time 2024-01-01 --to-time 2024-01-02 --app demo
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
| `-q` / `--content` | `query_string` on `content`，`default_operator: AND`（#23）；含 `.` 自动 `*...*`；`--content-wildcard` 强制包裸片段（结构化不包）；`--no-content-auto-wildcard` 关自动 |
| `--extend field=value`（可重复） | `match_phrase` 扩展/MDC 字段 |
| `--from-time` / `--to-time` | `range` on `dtTime` + clientStart/EndDate；未来 `to` 夹到 now |
| `--size` / `--from` | 分页（默认 size 100 或 config `default_size`） |
| `--sort` | 覆盖排序；默认 `dtTime desc` |
| `--no-highlight` | 关闭 content highlight |
| `--dsl` / `--dsl-file` | 透传 body；与快捷 flag 互斥 |
| `--index` / `--range day\|week\|month` | 改走 `POST /query` |

未写时间时用 `default_time_window`（默认 1h）选日索引。运行日志索引形如 `plume_log_run_YYYYMMDD`。

## `--index` 时间窗（Issue #4）

`POST /query` 在**无显式时间**时，服务端容易按「今天」窗处理；历史索引再叠今天窗会**静默 0 条**。

| 写法 | 行为 |
|------|------|
| `--index plume_log_run_20261016`（无时间 flag） | CLI（`search`/`dsl`）**自动**按索引日期对齐全天窗（stderr `note:`）；body `dtTime` 覆盖该日 |
| `--index … --from-time/--to-time` | 显式时间**优先**，不做自动对齐 |
| `--index … --range day\|week\|month` | `--range` 是**相对现在**的服务端窗；与历史索引组合可能仍偏今天 → stderr `warning:`；历史索引请用显式时间 |
| `--index` 名无 `YYYYMMDD` 且无时间 | stderr `warning:`，避免误判无日志 |

优先级（body 时间窗）：`--from-time/--to-time` > 索引日自动对齐 > `default_time_window`。`--range` 只影响 URL，不替代显式/自动对齐的 body 窗说明。

```bash
# 历史索引裸用（推荐）——自动对齐到 2026-10-16 全天
plumelog query search --index plume_log_run_20261016 --size 1 --jq '.result.hits.total'

# 显式时间（最稳）
plumelog query search --index plume_log_run_20261016 --from-time 2026-10-16 --to-time 2026-10-17 --size 1
```

`query dsl --index`（无时间 flag）与 `search` **同一套对齐**：stderr `note:`，并往 raw DSL body **写入/替换** `dtTime` 全日窗（避免仅 note、body 不改的不对称）。

```bash
# raw DSL + 历史索引：body 也会带上索引日 dtTime
echo '{"query":{"match_all":{}}}' | plumelog query dsl --index plume_log_run_20261016 --size 1

## `-q` 多词默认 AND（Issue #23）

多词 `-q` 不带引号时，ES `query_string` 默认 **OR**，常见 token 会假命中。
CLI 在构造 body 时固定 `default_operator: AND`（与 #1 dotted 通配同一构造路径）。
显式 `OR` / `AND` / `NOT` / 引号短语仍按 Lucene 语义；精确短语继续用引号。

```bash
# 多词：现在要求同时含各 token（不再 OR 假命中）
plumelog query search -q "Column index out of range" --size 5
# 精确短语
plumelog query search -q '"Column index out of range"' --size 5
```

## `-q` / `--content` 分词陷阱（Issue #1）

`-q` 走 ES `query_string`（`default_field: content`）。standard analyzer 会把
`java.lang.OutOfMemoryError` 切成**单个 token**；只搜 `OutOfMemoryError` 或未通配的类名片段会**恒 0 命中**，
排障事故时极易误判「无报错」。

| 写法 | 行为 |
|------|------|
| `-q OutOfMemoryError` | 可能漏报（无 `.`、无通配） |
| `-q java.lang.OutOfMemoryError` | CLI **自动**包成 `*java.lang.OutOfMemoryError*` |
| `-q OutOfMemoryError --content-wildcard` | 强制包成 `*OutOfMemoryError*`（`AND` / `field:value` 等结构化 **不**包） |
| `-q '*OutOfMemoryError*'` | 已有通配，CLI 不改写 |
| `-q java.lang.OutOfMemoryError --no-content-auto-wildcard` | 关闭 dotted 自动通配 |
| `-q 'timeout AND retry'` 即使带 `--content-wildcard` | 结构化 query_string，**永不**包通配 |

```bash
plumelog query search -q OutOfMemoryError --content-wildcard --size 1 --jq '.result.hits.total'
plumelog query search -q 'java.lang.OutOfMemoryError' --size 1 --jq '.result.hits.total'
plumelog query search -q 'java.lang.OutOfMemoryError' --no-content-auto-wildcard --size 1
```