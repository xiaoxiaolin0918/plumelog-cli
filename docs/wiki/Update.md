# Update / background check

## One line

`plumelog update --check` compares the current binary to GitHub Releases; `plumelog update --yes` verifies SHA-256 and replaces this binary. Background check and silent replace are **off by default**.

## Commands

```text
plumelog update --check          # compare only; do not download
plumelog update --yes            # download + verify + replace (without --yes this is the same as --check)
plumelog update --yes --dry-run  # preview the asset that would be replaced
```

Asset names: `plumelog_<os>_<arch>[.exe]`. Checksums come from `checksums.txt`.

## Background check (default off)

Enable any one of:

| How | Value |
|-----|-------|
| flag | `--check-update` |
| env | `PLUMELOG_CHECK_UPDATE=1` |
| config | `plumelog config set check_update true` |

After a successful command, the CLI may check for a newer release (cache ≤24h, short timeout). If one is found it prints a **stderr** hint only and never writes it to JSON stdout.

## Silent auto-replace (default off, separate switch)

| How | Value |
|-----|-------|
| flag | `--auto-update` |
| env | `PLUMELOG_AUTO_UPDATE=1` |
| config | `plumelog config set auto_update true` |

Only when a newer release is found **and** this switch is on does the CLI download and replace the binary. Failure rolls back when possible and is reported on stderr. Turning on auto-update also enables check-update.

`update`, `skills`, `help`, and `completion` skip the background hook.
