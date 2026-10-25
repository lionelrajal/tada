# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0] - 2026-10-25

### Added

- `tada edit <id>...` changes the title, due date or priority of existing
  tasks while keeping their IDs, so a typo no longer means deleting a task and
  adding it back under a new number. Title words may be passed unquoted, an
  empty `--due` clears the date, and every ID is checked before anything is
  written.
- `tada list` filters tasks: positional terms match the title ignoring case and
  must all match, a `~term` hides matches, and `--overdue`, `--due-before` and
  `--priority` narrow the list by field. The `~` form is used because `-term`
  is consumed by the flag parser before the command can see it.
- `tada list --sort due|priority|created` reorders the output. Insertion order
  remains the default, `due` sorts undated tasks last, and ties break by ID.
  `created` orders by ID until completion timestamps exist.
- `--json` on `tada list` and `tada add` emits one compact JSON document using
  the data file's own field names, for scripting, status bars and `fzf`.
- `--due` accepts relative dates: `today`, `tomorrow`, `yesterday`, weekday
  names such as `fri`, `next-fri`, and offsets such as `+3d` or `-1w`. Values
  are normalised to `YYYY-MM-DD` before storage, so the file format is
  unchanged. A weekday never resolves to today.

### Fixed

- Concurrent `tada` processes can no longer lose tasks. Writes take an
  exclusive lock on `<data-file>.lock` for the whole read-modify-write, and a
  lock held for more than five seconds fails loudly with exit code `1` instead
  of being ignored. Locking uses `github.com/gofrs/flock`, which maps to
  `flock` on unix and `LockFileEx` on Windows.

## [0.1.0] - 2026-10-24

### Added

- `tada add` with `--due` and `--priority` flags; titles may be passed unquoted.
- `tada list` with `--all` to include completed tasks.
- `tada done` and `tada rm` accepting one or more task IDs.
- `tada --version` reporting the build version, commit and date.
- JSON storage with atomic writes, stable ID assignment and support for the
  earlier bare-array file format.
- `schema_version` field in the data file, so a newer store can refuse to
  rewrite data it does not fully understand.
- Release pipeline built on GoReleaser and GitHub Actions, documented in
  [RELEASING.md](RELEASING.md).

### Changed

- The project is named **Tada**; the module path is
  `github.com/lionelrajal/tada` and the binary is `tada`.
- Commands receive their store explicitly instead of through a package-level
  singleton and the `TODO_DATA_FILE` environment variable.
- `--verbose` now actually reports the data file in use; it was previously
  read before flags were parsed.
- Failures print to stderr and exit with status 1 instead of reporting success.

### Fixed

- The project did not compile: the store package was missing its implementation
  and the `add` command imported placeholder module paths.

[Unreleased]: https://github.com/lionelrajal/tada/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/lionelrajal/tada/releases/tag/v0.2.0
[0.1.0]: https://github.com/lionelrajal/tada/releases/tag/v0.1.0
