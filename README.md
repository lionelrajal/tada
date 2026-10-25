# Tada

A small command-line todo list written in Go. Tasks live in a JSON file, so
there is no server, no account and no sync to configure.

## Installation

Download the archive for your platform from the
[releases page](https://github.com/lionelrajal/tada/releases), verify it against
`checksums.txt`, and put the `tada` binary somewhere on your `PATH`.

```bash
tar -xzf tada_0.1.0_darwin_arm64.tar.gz
sudo mv tada /usr/local/bin/
tada --version
```

To build from source instead:

```bash
make build        # writes bin/tada
```

## Build & Release

```bash
make build        # Build the CLI into bin/tada
make test         # Run unit tests
make check        # gofmt, go vet and race tests - the same gates CI runs
make snapshot     # Dry-run the release pipeline into dist/ (needs goreleaser)
make release      # Offline cross-compile for all targets; publishes nothing
make clean        # Remove bin/ and dist/
```

Publishing is automated: pushing a `v*` tag builds every target with GoReleaser
and publishes a GitHub release. The versioning, branching and rollback policy
lives in [RELEASING.md](RELEASING.md).

## Usage

```bash
# Add a task
$ tada add "Buy milk" --due 2026-12-31 --priority 1
Added task 1: Buy milk

# Add several words without quoting them
$ tada add write the weekly report
Added task 2: write the weekly report

# List pending tasks (--all also shows completed ones)
$ tada list
1  [ ]  medium  2026-12-31  Buy milk
2  [ ]  low     -           write the weekly report

# Complete and delete tasks by ID
$ tada done 1
Completed task 1: Buy milk

$ tada rm 2
Removed task 2: write the weekly report

# Report the build you are running
$ tada --version
tada 0.1.0 (commit 1a2b3c4, built 2026-10-24T09:00:00Z)
```

### Flags

- `--data-file <path>` – Path to the JSON file that stores tasks. Default: `tasks.json` in the current directory.
- `--verbose` – Enable verbose output for debugging.

### Concurrent commands

Each command takes an exclusive lock on `<data-file>.lock` while it rewrites the
task file, so two `tada` processes started at the same time cannot overwrite
each other's changes. The lock is released by the operating system, so a killed
process never blocks later runs; only the empty `.lock` file is left behind.
If another process holds the lock for more than five seconds, the command fails
with "timed out waiting for the data file lock" and exits `1` instead of waiting
forever or writing anyway.

## Extending the Tool

The project follows a clear package structure:

```
cmd/      # Cobra command definitions
model/    # Domain logic (Task struct, validation)
store/    # Persistence layer (JSON file store)
```

To add a new command:
1. Create a new Go file in `cmd/` with a function returning a `*cobra.Command`.
2. Register it with `rootCmd.AddCommand()` in `newRootCmd` in `cmd/root.go`.
3. Use `app.Store()` to access persistence; the store is built from `--data-file` before any command runs.
4. Add any necessary flags via `cmd.Flags().<Type>Var`.

Feel free to replace the JSON store with a database by implementing the `store.Store` interface.

## Testing

Run unit tests with:
```bash
go test ./...
```

CI runs the same suite on Linux and Windows, because the data file lock is
implemented by a different OS primitive on each. Run `make check` before pushing;
it adds the formatting and vet gates CI enforces.

## License

MIT - see [LICENSE](LICENSE).
