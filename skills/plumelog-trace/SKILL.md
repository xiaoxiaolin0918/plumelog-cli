---
name: plumelog-trace
version: 0.1.0
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