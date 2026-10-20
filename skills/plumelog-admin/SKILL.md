---
name: plumelog-admin
version: 0.1.1
description: "PlumeLog admin: queue counts/clear, index info/delete/reset, run-model, delete-by-query. High-risk ops require --dry-run first, then --yes; queue clear and index delete also need --admin-password."
metadata:
  requires:
    bins: ["plumelog"]
  cliHelp: "plumelog admin --help"
---

# admin（队列 / 索引 / 运行模式）

开始前先读 [`../plumelog-shared/SKILL.md`](../plumelog-shared/SKILL.md)。

**Agent 默认只跑只读**：`queue counts`、`index info`、`run-model`。高风险写必须用户显式同意；生产环境禁止未经确认的 clear/delete。

## 只读

```bash
plumelog admin queue counts
plumelog admin index info --index "plume_log_run_*"
plumelog admin run-model
```

## 高风险（必须先 --dry-run）

| 命令 | `--yes` | `--admin-password` | 上游 |
|------|---------|-------------------|------|
| `admin queue clear` | 必 | 必（query `adminPassWord`） | `POST /deleteQueue` |
| `admin index delete` | 必 | 必 | `POST /deleteIndex` |
| `admin index reset` | 必 | 不发送 | `POST /resetIndex` |
| `admin delete-by-query` | 必 | 不发送 | `POST /deleteByQuery` + DSL body |

```bash
# 1) 预览（零网络；密码在 URL 中打码）
plumelog admin queue clear --dry-run
plumelog admin index delete --dry-run --index plume_log_run_20240101
plumelog admin index reset --dry-run --index plume_log_run_20240101
plumelog admin delete-by-query --dry-run --index plume_log_run_20240101 --dsl '{"query":{"match_all":{}}}'

# 2) 向用户展示 dry-run 结果；仅在明确同意后：
plumelog admin queue clear --yes --admin-password "$PLUMELOG_ADMIN_PASSWORD"
plumelog admin index delete --yes --index plume_log_run_20240101 --admin-password "$PLUMELOG_ADMIN_PASSWORD"
plumelog admin index reset --yes --index plume_log_run_20240101
plumelog admin delete-by-query --yes --index plume_log_run_20240101 --dsl-file delete.json
```

### 门禁失败（现状）

缺确认时 **exit 10** + `confirmation_required` 对象（见 shared）。**不是** 旧文档曾写的 exit 10 / `confirmation_required`。识别后停下来问用户，禁止静默加 `--yes`。

生产（`http://plumelog.iipcloud.cn`）上：**不要**对真实 clear/delete 去掉 `--dry-run`，除非用户点名并知情。