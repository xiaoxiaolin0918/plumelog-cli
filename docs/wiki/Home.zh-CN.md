# plumelog-cli（中文）

[PlumeLog](https://github.com/fayechenlong/plumelog) 服务的命令行客户端（默认端口 **8891**）。

- **仓库：** https://github.com/xiaoxiaolin0918/plumelog-cli（PUBLIC，MIT）
- **发布：** [v0.1.2](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.2)
- **二进制：** `plumelog`
- **鉴权：** 与网页相同的 Session Cookie — **不是** OAuth / Bearer
- **文档位置：** 本目录页面在**仓库内** [`docs/wiki/`](README.md)，**不是** GitHub Wiki

English: [Home](Home.md)

## 页面

| 页面 | 说明 |
|------|------|
| [Getting-Started](Getting-Started.md) | 安装、doctor、首次查询 |
| [Auth](Auth.md) | 登录方式（含 `--web`） |
| [Skills-for-AI](Skills-for-AI.md) | Agent：信封、exit 10、skills |
| [Architecture](Architecture.md) | 契约摘要 + 链到仓内文档 |

## 安全

不要提交 Cookie、密码、`.env`。优先用环境变量。文档示例使用 `http://127.0.0.1:8891` 等占位主机。
