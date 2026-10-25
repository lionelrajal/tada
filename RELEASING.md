# Release management

This document is the policy for how Tada is versioned, branched, built and
published. The automation in `.github/workflows/` and `.goreleaser.yaml`
implements it; where the two disagree, this document wins and the automation
should be fixed.

## Compatibility contract

Only three things are promises to users. Everything else (internal packages,
the on-disk file layout beyond the fields below, log wording) may change in any
release.

| Surface | What is promised |
| --- | --- |
| CLI | Command names, flags, their short aliases, and exit codes (0 success, 1 failure) |
| `tasks.json` | The `schema_version` field and the meaning of existing task fields |
| Output | The `ID  [x]  priority  due  title` columns of `tada list`, which scripts may parse |

## Versioning

[Semantic Versioning 2.0](https://semver.org/), tagged as `vMAJOR.MINOR.PATCH`.
The first release is **v0.1.0**.

While the major version is 0, the project is still settling its CLI and data
format, so a minor bump may contain breaking changes. Patch releases fix bugs
and never break anything.

| Change | Version while `0.x` | Version from `1.0` |
| --- | --- | --- |
| Breaking CLI, output or schema change | minor | major |
| Backwards-compatible feature | minor | minor |
| Bug fix, docs, internal refactor | patch | patch |

**1.0.0** is declared when the command set, flags and `tasks.json` fields are
considered final.

Version numbers are a human decision recorded in the tag, not a count of
commits. `tada --version` reports the tag it was built from.

## Branching and commits

Trunk-based development:

- `main` is always releasable and protected: pull request plus green CI.
- Work happens on short-lived branches (`feat/...`, `fix/...`), which are
  deleted after merge. The current `UI` branch is one of these.
- Releases are cut from tags on `main`. There is no `develop` branch and no
  release branch for normal releases.
- Tags are immutable: never deleted, moved, force-pushed or reused. If a
  release is wrong, publish a new patch version.

Commits follow [Conventional Commits](https://www.conventionalcommits.org/),
because the changelog and the bump decision are derived from them:

```
feat: add due-date filtering to list
fix: keep unquoted add titles intact
docs: explain the release runbook
feat!: rename --priority to --level
```

`!` (or a `BREAKING CHANGE:` footer) marks a breaking change. `docs:`, `test:`,
`chore:`, `ci:` and `style:` commits are excluded from the generated changelog.

## Release runbook

1. Merge the branch into `main`; wait for CI to pass.
2. Decide the next version from the table above.
3. Add the entry to `CHANGELOG.md` under a new version heading, dated today.
4. Verify the build locally:

   ```bash
   make check                               # gofmt, vet, race tests
   make snapshot                            # full GoReleaser dry run into dist/
   ./dist/tada_*_darwin_arm64/tada --version
   ```
5. Tag and push:

   ```bash
   git tag -a v0.1.0 -m "v0.1.0"
   git push origin v0.1.0
   ```

6. The `Release` workflow verifies the tagged commit is on `main` and runs the
   full suite on Linux and Windows. If that passes, it pauses on the protected
   `release` environment until a maintainer approves the deployment in the
   Actions UI. Only then does it build, archive and publish the GitHub release.
   A failing suite stops the run before the approval is even requested.
7. Confirm the release page lists every artifact, then download one and run
   `tada --version` plus `tada add`/`tada list` to check it is usable.
8. Only then announce it.

## Pre-releases

Tag `vX.Y.Z-rc.N` (or `-beta.N`). GoReleaser marks it a prerelease
automatically. Pre-releases are for testing and are never the version quoted in
install instructions or documentation.

## Hotfixes and rollback

- **Hotfix:** branch `fix/...` from `main`, fix, PR, patch bump, new tag.
- **Broken release:** mark the GitHub release as a prerelease (or edit its
  notes to point at the replacement) and ship a patch. Do not delete or retag.
- **Security release:** same flow, but prepare the fix privately, publish the
  patched version and the advisory together, and note it under a `Security`
  heading in the changelog.

## Artifacts

Every release publishes, for darwin, linux and windows on amd64 and arm64:

- `tada_<version>_<os>_<arch>.tar.gz` (`zip` on Windows),
- `checksums.txt` with SHA-256 sums,
- generated changelog notes grouped into Features / Bug fixes / Other.

Archives include `LICENSE`, `README.md` and `CHANGELOG.md`, so a downloaded
binary carries its licence and usage notes.

## Automation

| File | Trigger | Purpose |
| --- | --- | --- |
| `.github/workflows/ci.yml` | every pull request, plus manual dispatch | `gofmt` check, `go vet`, tests on Linux and Windows, `go build` |
| `.github/workflows/release.yml` | push of a `v*` tag | verifies the tag is on `main`, runs the suite on Linux and Windows, waits for approval on the `release` environment, then GoReleaser publishes |
| `.goreleaser.yaml` | invoked by the release workflow | cross-compiles, archives, checksums, writes notes |
| `Makefile` | local | `make check` mirrors CI; `make snapshot` dry-runs the pipeline |

CI and release jobs use the Go version pinned in `go.mod` and the default
`GITHUB_TOKEN`; no long-lived secrets are stored. The release workflow holds
`contents: write` only for the job that publishes. Builds use `-trimpath` and
CGO is disabled, so the same source produces the same binaries on any runner.

## Data format compatibility

`tasks.json` carries `schema_version` (currently `1`):

- A file without the field is format 1 and loads normally; the field is written
  on the next save.
- A file with a **newer** version is rejected with an error rather than
  rewritten, because an older binary would silently drop fields it does not
  know about.
- Any change to the format bumps `SchemaVersion` in `store/store.go`, ships
  with a migration that runs on load, and is called out as breaking if old
  binaries cannot read the result.

## Known gaps

- `go install github.com/lionelrajal/tada@latest` only works once the
  repository is public at that path.
- No Homebrew tap, Docker image or artifact signing yet. Signing the checksums
  (cosign) is the next supply-chain step if the project gains users.

## Maintainer checklist

- [ ] `main` is green and contains everything meant to ship
- [ ] `CHANGELOG.md` has an entry for the new version
- [ ] `make check` and `make snapshot` pass locally
- [ ] version bump matches the change type
- [ ] tag created on `main` and pushed
- [ ] release artifacts and checksums verified on the release page
- [ ] a downloaded binary runs `--version`, `add` and `list`
