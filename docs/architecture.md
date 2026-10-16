# plumelog-cli 架构与 API/鉴权契约

> 面向「plumelog 实现」落地。对照上游 [fayechenlong/plumelog](https://github.com/fayechenlong/plumelog)（`MainController`、`LoginCheckInterceptor`、`Home.vue`）与参考模式 `yunxiao-cli`（cobra、auth/doctor/api、`--dry-run`/`--jq`/`--yes`）。
>
> 上游对照日：2026-10-16（master）。**禁止在日志/stdout/docs 示例中打印真实密码、adminPassWord、Session Cookie 值。**

---

## 1. 命令树（树状）

```
plumelog
├── auth
│   ├── login          # POST /login；写入 Cookie jar
│   ├── logout         # POST /logout；清空 Cookie jar
│   └── status         # 本地检查：是否有 Cookie、base-url、是否过期未知则探测 GET/轻量只读
├── doctor             # 连通性 + 鉴权 + 服务模式探测（getRunModel / getQueueCounts）
├── config
│   ├── get | set | path
│   └── show           # 脱敏展示配置（永不打印密码/Cookie 明文）
├── query
│   ├── search         # 主查询：快捷过滤器 → ES DSL → POST /clientQuery（或 /query）
│   ├── dsl            # 原始 DSL 透传（body 来自 --file / stdin）
│   └── apps           # 应用名列表：POST /queryAppNames 或 /getAppNames
├── trace
│   └── get            # 按 traceId 查链路：POST /clientQuery?trace=true
├── app
│   ├── list           # 同 query apps / getAppNames
│   ├── extend list    # GET 扩展字段：POST /getExtendfieldList?appName=
│   ├── extend add     # POST /addExtendfield
│   └── extend del     # POST /delExtendfield（高风险，需 --yes）
├── alarm
│   ├── list           # POST /getWarningRuleList
│   ├── save           # POST /saveWarningRuleList?id=
│   ├── delete         # POST /deleteWarningRule?id=（高风险，需 --yes）
│   └── messages       # 报警消息索引查询：POST /query?index=plumelog_monitor_message_key
├── admin
│   ├── queue counts   # POST /getQueueCounts
│   ├── queue clear    # POST /deleteQueue?adminPassWord=  【高风险】
│   ├── index info     # POST /getServerInfo?index=
│   ├── index delete   # POST /deleteIndex?index=&adminPassWord=  【高风险】
│   ├── index reset    # POST /resetIndex?index=  【高风险】
│   └── run-model      # POST /getRunModel
├── api                # 原始逃生舱：任意 METHOD + path（双前缀解析）
│   └── <METHOD> <path>
└── version / help
```

对齐 yunxiao-cli：命令 `--help` 标注 Risk：`read` | `write` | `high-risk-write`；`high-risk-write` 必须 `--yes`（及适用时的 `--admin-password`）。

---

## 2. 全局 Flags

| Flag | 类型 | 默认 | 说明 |
|------|------|------|------|
| `--base-url` | string | 配置文件 `base_url` / 环境变量 `PLUMELOG_BASE_URL` | 服务根，如 `http://host:8891`；**不要**带尾斜杠；路径前缀见 §4 |
| `--format` | string | `json` | `json` \| `pretty` \| `table`（实现期 table 可后续加） |
| `--jq` | string | `""` | jq 表达式过滤 JSON 输出（对齐 yunxiao-cli） |
| `--dry-run` | bool | false | 只打印将发出的 Method/URL/Headers(脱敏)/Body，不执行 |
| `--yes` / `-y` | bool | false | 确认高风险写操作 |
| `--admin-password` | string | `""` / `PLUMELOG_ADMIN_PASSWORD` | 映射上游 query 参数 `adminPassWord`；**永不写入日志明文**；可用交互/环境变量注入 |
| `--path-prefix` | string | `auto` | `""` \| `/plumelog` \| `auto`（先试配置，失败可回退；见未决项） |
| `--cookie-jar` | string | 见下 | 覆盖默认 Cookie jar 路径 |
| `--config` | string | 见下 | 覆盖配置文件路径 |
| `--timeout` | duration | `30s` | HTTP 超时（上游 UI axios 默认很长，CLI 宜短并可配） |

### Cookie jar 路径约定

| 项 | 约定 |
|----|------|
| 默认目录 | `$XDG_CONFIG_HOME/plumelog/`；Windows：`%APPDATA%\plumelog\`；若无则 `~/.config/plumelog/` |
| Cookie jar | `<config_dir>/cookies.txt`（Netscape / Go `cookiejar` 持久化格式二选一，实现固定一种并文档化） |
| 配置文件 | `<config_dir>/config.json` |
| 权限 | 文件建议 `0600`（Unix）；Windows 尽量收紧 ACL |
| Session | 上游为 **HttpSession**：`login` 成功后 `session.setAttribute("token", …)`；浏览器侧依赖 `JSESSIONID`（或容器等价 Cookie）+ `axios.defaults.withCredentials=true` |
| CLI 行为 | 所有需鉴权的 POST 自动附带 Cookie jar；`auth status` **只**输出 `has_cookie` / `cookie_path` / `base_url`，**不**打印 Cookie 值 |

环境变量优先级（建议）：

`PLUMELOG_BASE_URL` / `PLUMELOG_ADMIN_PASSWORD` / `PLUMELOG_LOGIN_USERNAME` / `PLUMELOG_LOGIN_PASSWORD` / `PLUMELOG_PATH_PREFIX`  
> 命令行 flag > 环境变量 > config.json。密码类仅用于请求，不回显。

---

## 3. 鉴权契约（Session Cookie）

### 上游行为摘要

| 点 | 上游事实 |
|----|----------|
| 登录 | `POST {/login,/plumelog/login}`，JSON body `LoginVO{username,password}` |
| 成功 | `session.setAttribute("token", new Object())`，返回 `Result`（默认 code 空/成功） |
| 失败 | `Result.INVALID_LOGIN`（code **402**），并 `removeAttribute("token")` |
| 登出 | `POST {/logout,/plumelog/logout}`，`removeAttribute("token")` |
| 拦截 | `LoginCheckInterceptor`：**仅 POST**；若 `InitConfig.loginUsername` 为空则**不鉴权**；否则无 `session.token` → `Result.UN_LOGIN`（code **401**） |
| 白名单 | URI 含 `sendRunLog` / `sendTraceLog` / `sendLog` 放行；`WebConfiguration` exclude：`/login`、`/logout`（另写了 `/plumelogServer/login|logout`，与 Controller 的 `/plumelog` 不完全一致，见未决项） |
| 前端 | `withCredentials=true`；响应 `code==401` 跳转登录页 |

### CLI 命令映射

| 命令 | 行为 |
|------|------|
| `auth login --username U` | 密码从 `--password` / `PLUMELOG_LOGIN_PASSWORD` / stdin 提示读取；POST login；持久化 Set-Cookie 到 jar；输出 `{ok, base_url, cookie_path}`（无明文） |
| `auth login`（无用户名且服务未开登录） | 探测：若 loginUsername 未配置，可空登录（上游允许）；记录「open auth」状态 |
| `auth logout` | POST logout + 删除/清空 jar |
| `auth status` | 读 jar 是否非空；可选 `--probe` 发只读 POST（如 `getRunModel`）判断 401 |

---

## 4. 上游路径映射表（`/` 与 `/plumelog` 双前缀）

所有路径在上游 `MainController` 均注册为 `{ "/foo", "/plumelog/foo" }`。CLI 请求 URL = `base_url` + `path_prefix` + `path`，其中 `path_prefix` ∈ `{ "", "/plumelog" }`。

| CLI 能力 | Method | 相对 path（不含前缀） | 关键参数 | Body | Risk |
|----------|--------|----------------------|----------|------|------|
| auth login | POST | `/login` | — | `{"username","password"}` | write |
| auth logout | POST | `/logout` | — | — | write |
| query search / dsl | POST | `/clientQuery` | `clientStartDate,clientEndDate,size,from,trace?` | ES DSL JSON | read |
| query（按 index/range） | POST | `/query` | `index?,size,from,range?=day\|week\|month` | ES DSL JSON | read |
| query apps（聚合） | POST | `/queryAppNames` | 可选 `appNameWithEnv` 标记式 query | ES agg DSL | read |
| query apps（缓存） | POST | `/getAppNames` | — | — | read |
| appName 单查 | POST | `/queryAppName` | — | ES DSL | read |
| trace get | POST | `/clientQuery` | 同上 + `trace=true` | DSL must traceId | read |
| alarm list | POST | `/getWarningRuleList` | — | — | read |
| alarm save | POST | `/saveWarningRuleList` | `id` | `WarningRule` JSON | write |
| alarm delete | POST | `/deleteWarningRule` | `id` | — | high-risk-write |
| alarm messages | POST | `/query` | `index=plumelog_monitor_message_key&from&size` | ES DSL | read |
| admin queue counts | POST | `/getQueueCounts` | — | — | read |
| admin queue clear | POST | `/deleteQueue` | `adminPassWord` | — | high-risk-write |
| admin index info | POST | `/getServerInfo` | `index` | — | read |
| admin index delete | POST | `/deleteIndex` | `index,adminPassWord` | — | high-risk-write |
| admin index reset | POST | `/resetIndex` | `index`（源码未强制 adminPassWord，CLI 仍要求 `--yes`） | — | high-risk-write |
| admin run-model | POST | `/getRunModel` | — | — | read |
| app extend list | POST | `/getExtendfieldList` | `appName` | — | read |
| app extend add | POST | `/addExtendfield` | `appName,field,fieldName` | — | write |
| app extend del | POST | `/delExtendfield` | `appName,field` | — | high-risk-write |
| （采集，一般不做 CLI） | POST | `/getlog`,`/sendLog` | `logKey,maxSendSize?` | logs[] for send | — |
| delete by query | POST | `/deleteByQuery` | `index,size,from` | ES DSL | high-risk-write（建议 CLI 暴露为 `admin delete-by-query`，须 `--yes`） |

**索引命名（只读约定，供 `--index` / 默认构造）：**

- 运行日志：`plume_log_run_YYYYMMDD*`（`IndexUtil.getRunLogIndex`）
- 链路日志：`plume_log_trace_YYYYMMDD*`
- 报警消息：`plumelog_monitor_message_key`（Warn.vue）

---

## 5. `query search` DSL 快捷过滤器（对照 Home.vue）

`Home.vue` 的 `getShouldFilter()` → `bool.must[]`；可选 `bool.must_not`（排除 appName）。主入口为 **`POST /clientQuery`**（按日起止算索引），而非裸 `/query`（除非用户指定 `--index` / `--range`）。

### 5.1 CLI flags ↔ ES 子句

| CLI flag | Home.vue `filter.*` | ES 子句 | 备注 |
|----------|---------------------|---------|------|
| `--app` / `--app-name` | `appName` | `match_phrase` on `appName`；多值逗号 → 空格拼接 | `--exclude-app` 时改入 `must_not` |
| `--env` | `env` | `match_phrase` on `env`（**不**把逗号替成空格） | |
| `--level` / `--log-level` | `logLevel` | `match_phrase` on `logLevel` | 常见：INFO/WARN/ERROR/DEBUG |
| `--trace-id` | `traceId` | `match_phrase` on `traceId` | 有值时 sort 改为 `dtTime asc, seq asc` |
| `--class` / `--class-name` | `className` | `match_phrase` on `className` | 类全路径 |
| `--server` / `--server-name` | `serverName` | `match_phrase` on `serverName` | IP 或 host |
| `--content` / `-q` | `searchKey` / `searchOptions` | `query_string`，`default_field: content` | 支持 Lucene query_string；多 tag 用 AND/OR 由 `--content-mode` 扩展（可选） |
| `--extend field=value`（可重复） | `extendOptions[]` | `match_phrase` on 动态 field | 扩展字段来自 MDC / extend map |
| `--from-time` / `--to-time` | `dateTimeRange` | `range` on `dtTime`：`gte`/`lt` 毫秒时间戳 | 默认建议近 1h～当日；`--to-time` 不可超过「现在」时钳制到 now（对齐 UI） |
| `--size` | `size` | query param `size` | 默认 100 |
| `--from` / `--offset` | `from` | query param `from` | 分页偏移 |
| `--sort` | `sort` | body `sort` | 默认 `[{"dtTime":"desc"}]`；有 traceId 时 `dtTime asc` + `seq asc` |
| `--highlight` | 固定 | `highlight.fields.content.fragment_size=2147483647` | 默认开，可用 `--no-highlight` 关 |
| `--dsl` / `--dsl-file` | — | **覆盖**快捷过滤器，整段 body 透传 | 与快捷 flag 互斥（或 `--dsl` 优先并警告） |
| `--index` | — | 改走 `POST /query?index=` | 显式索引列表 |
| `--range day\|week\|month` | — | `POST /query?range=` | 与 `--index` 二选一策略 |
| `--trace` | Trace 页 | `clientQuery&trace=true` | `trace get` 子命令默认打开 |

### 5.2 组装模板（实现伪代码）

```json
{
  "query": {
    "bool": {
      "must": [ /* getShouldFilter() */ ],
      "must_not": [ /* 仅 --exclude-app */ ]
    }
  },
  "highlight": {
    "fields": { "content": { "fragment_size": 2147483647 } }
  },
  "sort": [{ "dtTime": "desc" }]
}
```

`clientQuery` URL 示例：

```
POST {base}{prefix}/clientQuery?clientStartDate={ms}&clientEndDate={ms}&size={n}&from={n}
```

链路：

```
POST .../clientQuery?trace=true&clientStartDate=...&clientEndDate=...&size=1000&from=0
Body: { "query": { "bool": { "must": [ { "match_phrase": { "traceId": { "query": "..." } } } ] } } }
```

---

## 6. 配置文件字段（`config.json`）

```json
{
  "base_url": "http://127.0.0.1:8891",
  "path_prefix": "",
  "timeout_sec": 30,
  "default_size": 100,
  "default_time_window": "1h",
  "format": "json",
  "cookie_jar": "",
  "login_username": ""
}
```

- **不要**把 `login_password` / `admin_password` 默认写入磁盘；若用户强行 `config set`，实现应警告并可选仅写 OS keychain（未决）。
- `path_prefix`：`""` 或 `"/plumelog"`。

---

## 7. 高风险命令清单（须 `--yes` + 适用时的 `--admin-password`）

| 命令 | 须 `--yes` | 须 `--admin-password`（→ `adminPassWord`） | 上游 |
|------|------------|---------------------------------------------|------|
| `admin queue clear` | ✓ | ✓ | `/deleteQueue` |
| `admin index delete` | ✓ | ✓ | `/deleteIndex` |
| `admin index reset` | ✓ | 建议可选（上游未校验密码，仍 `--yes`） | `/resetIndex` |
| `admin delete-by-query`（若实现） | ✓ | 建议 ✓（额外防护） | `/deleteByQuery` |
| `alarm delete` | ✓ | — | `/deleteWarningRule` |
| `app extend del` | ✓ | — | `/delExtendfield` |
| `alarm messages clear`（若封装删监控索引） | ✓ | ✓ | `/deleteIndex?index=plumelog_monitor_message_key` |

`--dry-run` 时即使带 `--yes` 也不得真实执行。  
缺少 `--yes`：退出码非 0，提示 Risk: high-risk-write（对齐 yunxiao-cli）。

---

## 8. `api` 逃生舱

```
plumelog api POST /clientQuery --query clientStartDate=... --body-file dsl.json
plumelog api GET /whatever   # 注意：拦截器对非 POST 不鉴权，但仍带 Cookie
```

- path 可写 `/login` 或 `/plumelog/login`；若只写 `/login`，由 `--path-prefix` 拼接。
- 支持 `--dry-run` / `--jq` / `--format`。

---

## 9. doctor 检查项（建议）

1. `base_url` 可达（TCP/HTTP）
2. Cookie 存在性（若配置了 login）
3. `POST getRunModel` → 200 且非 401
4. `POST getQueueCounts`（redis 模式有意义）
5. 打印 `path_prefix`、配置路径、cookie 路径（脱敏）

---

## 10. 实现分期建议

| 阶段 | 内容 |
|------|------|
| P0 | root flags、config、auth login/logout/status、cookie jar、`query search`（快捷过滤器 + clientQuery）、`trace get`、`api`、`--dry-run/--jq/--yes` |
| P1 | `query apps`、`doctor`、`alarm list/save/delete`、`app extend *` |
| P2 | `admin *`、高风险门禁、`delete-by-query`、table 输出 |

---

## 11. 未决问题（不阻塞主契约）

1. **`/plumelog` vs `/plumelogServer`**：`WebConfiguration` exclude 写的是 `/plumelogServer/login`，Controller 映射是 `/plumelog/login`。部署若用 context-path `/plumelogServer`，CLI 的 `--path-prefix` 可能还需第三值；默认按 Controller 的 `""`/`/plumelog`。
2. **Session Cookie 名**：依容器（常为 `JSESSIONID`）；实现应用通用 cookiejar，勿写死名称（可在 doctor 中列出 cookie **names** only）。
3. **open auth**：`loginUsername` 为空时全站 POST 不校验；`auth login` 是否仍写占位 cookie — 建议仍 POST `/login` 以建立 session。
4. **`resetIndex` 未校验 adminPassWord**：CLI 是否强制 `--admin-password` 与上游一致放宽 — 建议强制 `--yes`，密码可选但推荐。
5. **采集类 API**（`getlog`/`sendLog`）：默认不进用户向命令树；仅 `api` 可达。
6. **CorsFilter / SameSite**：跨域部署时 CLI 直连 base_url，一般无浏览器 CORS 问题；若前置网关改写 Cookie，需实测。
7. **报警 Webhook 字段**：`WarningRule` 细节以实现期再对照 DTO；本契约只定路径。

---

## 12. 参考源码路径（远程只读，勿整仓克隆到业务机）

- `plumelog-server/.../controller/MainController.java`
- `plumelog-server/.../config/LoginCheckInterceptor.java`
- `plumelog-server/.../config/WebConfiguration.java`
- `plumelog-ui-front/src/views/Home.vue`（`getShouldFilter` / `doSearch` / `clientQuery`）
- `plumelog-ui-front/src/views/Trace.vue`、`Warn.vue`、`Size.vue`、`Expand.vue`
- `plumelog-ui-front/src/services/http.js`（`withCredentials`）

参考 CLI 骨架：`D:\work\grokbot\yunxiao-cli\cmd\root.go`、`auth.go`。
