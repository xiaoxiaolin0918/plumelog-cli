# Update / background check

## One line

`plumelog update --check` compares the current binary to GitHub Releases; `plumelog update --yes` verifies SHA-256 and replaces this binary. Background check and silent replace are **on by default**.

## Commands

```text
plumelog update --check          # compare only; do not download
plumelog update --yes            # download + verify + replace (without --yes this is the same as --check)
plumelog update --yes --dry-run  # preview the asset that would be replaced
```

Asset names: `plumelog_<os>_<arch>[.exe]`. Checksums come from `checksums.txt`.

## Background check (default on)

Disable with any one of:

| How | Value |
|-----|-------|
| flag | `--no-check-update` |
| env | `PLUMELOG_CHECK_UPDATE=0` |
| config | `plumelog config set check_update false` |

After a successful command, the CLI may check for a newer release (cache ≤24h, short timeout). If one is found and auto-replace is off, it prints a **stderr** hint only and never writes it to JSON stdout. `--no-check-update` also disables silent auto-replace.

## Silent auto-replace (default on)

Disable with any one of (check/hint stays on unless check is also disabled):

| How | Value |
|-----|-------|
| flag | `--no-auto-update` |
| env | `PLUMELOG_AUTO_UPDATE=0` |
| config | `plumelog config set auto_update false` |

When a newer release is found and this switch is on, the CLI downloads and replaces the binary (SHA-256 verified). Failure rolls back when possible and is reported on stderr. `--dry-run` never replaces. Auto-update implies check-update.

`update`, `skills`, `help`, and `completion` skip the background hook.
