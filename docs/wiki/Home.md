# plumelog-cli

Command-line client for a [PlumeLog](https://github.com/fayechenlong/plumelog) server (default port **8891**).

- **Repo:** https://github.com/xiaoxiaolin0918/plumelog-cli (PUBLIC, MIT)
- **Release:** [v0.1.2](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.2)
- **Binary:** `plumelog`
- **Auth:** Session Cookie (same as the web UI) — **not** OAuth / Bearer
- **Docs location:** these pages are **in-repo** under [`docs/wiki/`](README.md) (not the GitHub Wiki)

Chinese: [Home.zh-CN](Home.zh-CN.md)

## Pages

| Page | Audience |
|------|----------|
| [Getting-Started](Getting-Started.md) | Humans: install, doctor, first query |
| [Auth](Auth.md) | Login modes including `--web` |
| [Skills-for-AI](Skills-for-AI.md) | Agents: envelope, exit 10, skills |
| [Architecture](Architecture.md) | Contract summary + link to in-repo doc |

## Security

Never commit cookies, passwords, or `.env`. Prefer `PLUMELOG_LOGIN_PASSWORD` / `PLUMELOG_ADMIN_PASSWORD`. Docs use placeholders such as `http://127.0.0.1:8891`.
