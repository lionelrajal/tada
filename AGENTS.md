# Working in this repository

## Branch workflow

`main` is protected: it accepts changes only through a pull request.

- Never commit feature or bug work directly to `main`.
- Work on a branch named `<type>/<short-topic>`, e.g. `fix/lost-update-race` or
  `feat/list-filtering`.
- Open a pull request against `main` and merge it there. Both
  `Test (ubuntu-latest)` and `Test (windows-latest)` are required checks, so a
  pull request cannot merge until CI is green.
- Squash merging is fine; keep the history linear and readable.

Documentation-only changes follow the same rule.

## Before pushing

```bash
make check   # gofmt, go vet, go test -race
```

CI runs those gates on every pull request, on Linux and Windows, because the
data-file lock uses a different OS primitive on each (`flock` on unix,
`LockFileEx` on Windows). A change to `store/` should be verified on both; do
not assume a POSIX-only run covers Windows. Merging to `main` does not start a
run of its own: the pull request checks are the gate, and the release workflow
re-runs the suite on the tagged commit before publishing.

## Releases

Releases are cut by pushing a `v*` tag to a commit on `main`. The release
workflow verifies the tag and tests it on Linux and Windows, then stops: a
pushed tag never publishes on its own. Publishing needs a maintainer to run the
`Release` workflow by hand with the tag typed into `tag` and `confirm`. See
[RELEASING.md](RELEASING.md). Never tag from a feature branch.

## Data format

`tasks.json` carries `schema_version` (currently `1`). Any format change bumps
`SchemaVersion`, ships a migration, and is called out in `CHANGELOG.md`; an
older binary must never silently drop fields it does not understand.
