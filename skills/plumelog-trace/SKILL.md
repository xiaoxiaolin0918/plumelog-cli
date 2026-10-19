---
name: plumelog-trace
version: 0.1.1
description: "PlumeLog trace lookup by traceId via POST /clientQuery?trace=true. Risk: read."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog trace --help"
---

# trace（链路）

开始前先读 [`../plumelog-shared/SKILL.md`](../plumelog-shared/SKILL.md)。

```bash
plumelog trace get --trace-id TRACE_ID
plumelog trace get --trace-id TRACE_ID --from-time 1700000000000 --to-time 1700003600000
plumelog trace get --trace-id TRACE_ID --app demo --size 100
```

- 端点：`POST /clientQuery?trace=true`，`match_phrase` on `traceId`
- 默认排序：`dtTime asc, seq asc`；默认 `--size` 1000
- 可选 `--app`、`--from`/`--size`、`--from-time`/`--to-time`、`--no-highlight`
- 链路索引形如 `plume_log_trace_YYYYMMDD`（由 clientStart/EndDate 选日）

也可用：`plumelog query search --trace-id TRACE_ID --trace`（search 侧）。独立 skill 便于 Agent 按「查链路」意图加载。
## Trace vs run-log（Issue #21）

| 命令 | 索引 | 适用 |
|------|------|------|
| `trace get --trace-id` | `plume_log_trace_YYYYMMDD` | 链路跨度/调用树 |
| `query search --trace-id` | `plume_log_run_YYYYMMDD` | 业务日志里的同一 traceId |

部分环境（尤其 test）链路索引可能无数据；`trace get` 命中 0 时 envelope 会带 **hint**，提示改用 `query search --trace-id`。不自动回退（避免 silently 混索引）。
