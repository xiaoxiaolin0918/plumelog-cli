# plumelog-cli

Command-line client for a [PlumeLog](https://github.com/fayechenlong/plumelog) server (default port **8891**).

- **Repo:** https://github.com/xiaoxiaolin0918/plumelog-cli (PUBLIC, MIT)
- **Release:** [v0.1.1](https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.1)
- **Binary:** `plumelog`
- **Auth:** Session Cookie (same as the web UI) — **not** OAuth / Bearer

Chinese: [[Home.zh-CN]]

## Pages

| Page | Audience |
|------|----------|
| [[Getting-Started]] | Humans: install, doctor, first query |
| [[Auth]] | Login modes including `--web` |
| [[Skills-for-AI]] | Agents: envelope, exit 10, skills |
| [[Architecture]] | Contract summary + link to in-repo doc |

## Security

Never commit cookies, passwords, or `.env`. Prefer `PLUMELOG_LOGIN_PASSWORD` / `PLUMELOG_ADMIN_PASSWORD`. Docs use placeholders such as `http://127.0.0.1:8891`.
