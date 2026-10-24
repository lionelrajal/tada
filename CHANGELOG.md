# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/lionelrajal/tada/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/lionelrajal/tada/releases/tag/v0.1.0
