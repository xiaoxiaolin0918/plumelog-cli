# Changelog


## Unreleased

### Added

- **`plumelog update`**: `--check` compares current vs GitHub Releases latest; `--yes` downloads the platform asset, verifies `checksums.txt` SHA-256, and replaces this binary. Manifest `update` argv is `["update"]` for cli-manager.

### Docs

- **GitHub Wiki deprecated/disabled**; sole docs source is in-repo `docs/wiki/`.


All notable changes to **plumelog-cli** are documented here. The binary name is `plumelog`.

## [0.1.4] - 2026-10-20

### Changed
- **trace get** shares `doWithAutoLogin` + `finishUpstream` with generic invoke (success-only enricher for empty-hit hint); `query apps` uses the same auto-login helper
- Skill frontmatter: `plumelog-admin|alarm|api|app` → **0.1.1**
- Docs: SECURITY/AGENTS current line **v0.1.4**; `docs/qa-checklist.md` rewritten for post-v0.1.3 reality; multi-profile explicitly **out of scope** (single instance + env/config)

### Added
- GitHub Actions **Release** workflow (`.github/workflows/release.yml`): on `v*` tag, upload linux/darwin/windows amd64/arm64 binaries + `checksums.txt`

### Notes
- Install: `go install …@v0.1.4` **or** download Release assets
- Old JSESSIONID sessions (if leaked in local QA) must be invalidated manually in the browser / server — CLI cannot revoke server sessions

## [0.1.3] - 2026-10-19

### Breaking

- **`query apps` result shape**: `result` is now always `{ "apps": [...], "count": N, ... }` (optional `app_like` / `sample` / enrich fields). Previously (≤0.1.2) cache-source responses often exposed `result` as a **bare JSON array**. Migrate `--jq` / scripts from `.result[0]` to **`.result.apps`**.

### Added

- Human-readable and relative `--from-time` / `--to-time` (`now`, `-3d`, `-2h`, `+15m`, RFC3339, local date layouts, epoch) — Issue #18
- `query apps --app-like` / `--sample` / `--enrich` — Issue #20
- `trace get` zero-hit `hint` pointing at `query search --trace-id` — Issue #21

### Fixed

- Multi-word `-q` / `--content` now sets Lucene `default_operator: AND` — Issue #23
- `--range day|week|month` expands body `dtTime` to 24h / 7d / **30d** when times are omitted (`month` = 30 days, not calendar month) — Issue #22
- Empty upstream body → `ok:false`, `error.subtype=empty_body`, non-zero exit (stub `result.hits`) — Issue #19

### Docs

- README / README.zh-CN / `docs/architecture.md` / skills aligned to #18–#23; version identity **0.1.3**

## [0.1.2] - 2026-10-19

- Bump CLI version identity to **0.1.2**
- Web login: Chinese UI, 服务地址, 记住并自动登录 with encrypted `login.cred`
- Docs/wiki polish since v0.1.1

## [0.1.1] - earlier

- See GitHub Releases for prior notes.

[0.1.4]: https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.4
[0.1.3]: https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.3
[0.1.2]: https://github.com/xiaoxiaolin0918/plumelog-cli/releases/tag/v0.1.2
