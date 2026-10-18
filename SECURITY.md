# Security Policy

## Supported versions

Security fixes target the latest release on `main` (currently **v0.1.2** and newer patches).

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security problems that could expose credentials or allow unauthorized access to a PlumeLog server.

Instead, contact the maintainer privately (GitHub user [@xiaoxiaolin0918](https://github.com/xiaoxiaolin0918)) with:

- Affected version / commit
- Description and impact
- Reproduction steps **without** real passwords, cookies, or production hostnames if avoidable

## What not to commit or share

- Session cookies (`JSESSIONID` / Netscape cookie jars)
- `PLUMELOG_LOGIN_PASSWORD`, `PLUMELOG_ADMIN_PASSWORD`, or any `.env` values
- Production base URLs paired with live credentials
- Smoke / QA run notes that embed secrets

Local cookie jars live under the OS config directory (see README) and are gitignored (`*_cookies.txt`, `.env`).
