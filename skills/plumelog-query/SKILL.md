---
name: plumelog-query
version: 0.1.1
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
| `-q` / `--content` | `query_string` on `content`ï¼å« `.` èªå¨ `*...*`ï¼`--content-wildcard` å¼ºå¶å
è£¸çæ®µï¼ç»æåä¸å
ï¼ï¼`--no-content-auto-wildcard` å
³èªå¨ |
| `--extend field=value`（可重复） | `match_phrase` 扩展/MDC 字段 |
| `--from-time` / `--to-time` | `range` on `dtTime` + clientStart/EndDate；未来 `to` 夹到 now |
| `--size` / `--from` | 分页（默认 size 100 或 config `default_size`） |
| `--sort` | 覆盖排序；默认 `dtTime desc` |
| `--no-highlight` | 关闭 content highlight |
| `--dsl` / `--dsl-file` | 透传 body；与快捷 flag 互斥 |
| `--index` / `--range day\|week\|month` | 改走 `POST /query` |

未写时间时用 `default_time_window`（默认 1h）选日索引。运行日志索引形如 `plume_log_run_YYYYMMDD`。

## `-q` / `--content` åè¯é·é±ï¼Issue #1ï¼

`-q` èµ° ES `query_string`ï¼`default_field: content`ï¼ãstandard analyzer ä¼æ
`java.lang.OutOfMemoryError` åæ**åä¸ª token**ï¼åªæ `OutOfMemoryError` ææªéé
çç±»åçæ®µä¼**æ 0 å½ä¸­**ï¼
ææ¥äºæ
æ¶ææè¯¯å¤ãæ æ¥éãã

| åæ³ | è¡ä¸º |
|------|------|
| `-q OutOfMemoryError` | å¯è½æ¼æ¥ï¼æ  `.`ãæ éé
ï¼ |
| `-q java.lang.OutOfMemoryError` | CLI **èªå¨**å
æ `*java.lang.OutOfMemoryError*` |
| `-q OutOfMemoryError --content-wildcard` | å¼ºå¶å
æ `*OutOfMemoryError*`ï¼`AND` / `field:value` ç­ç»æå **ä¸**å
ï¼ |
| `-q '*OutOfMemoryError*'` | å·²æéé
ï¼CLI ä¸æ¹å |
| `-q java.lang.OutOfMemoryError --no-content-auto-wildcard` | å
³é­ dotted èªå¨éé
 |
| `-q 'timeout AND retry'` å³ä½¿å¸¦ `--content-wildcard` | ç»æå query_stringï¼**æ°¸ä¸**å
éé
 |

```bash
plumelog query search -q OutOfMemoryError --content-wildcard --size 1 --jq '.result.hits.total'
plumelog query search -q 'java.lang.OutOfMemoryError' --size 1 --jq '.result.hits.total'
plumelog query search -q 'java.lang.OutOfMemoryError' --no-content-auto-wildcard --size 1
```

