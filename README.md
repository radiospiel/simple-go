# simple-go

A collection of small, independent Go packages (assertions, logging, JSON
helpers, observables, path matching, and other utilities) extracted for
standalone use.

## Packages

| Package | Description |
| --- | --- |
| [`ansi`](docs/ansi.md) | ANSI color escape sequences |
| [`assert`](docs/assert.md) | Test assertion helpers |
| [`conduit`](docs/conduit.md) | Path-scoped synchronization primitive |
| [`dump`](docs/dump.md) | Debug value dumping |
| [`fnmatch`](docs/fnmatch.md) | Shell-style pattern matching |
| [`json`](docs/json.md) | JSON helpers |
| [`logger`](docs/logger.md) | Structured logging with rotation and following |
| [`must`](docs/must.md) | Panic-on-error helpers |
| [`observable`](docs/observable.md) | Observable, path-addressable data wrapper |
| [`preconditions`](docs/preconditions.md) | Precondition/invariant checks |
| [`tasks`](docs/tasks.md) | Exclusive task execution |
| [`utils`](docs/utils.md) | General-purpose utilities |

Full API documentation is available in [docs/](docs/README.md).

## Building and testing

```sh
make build   # build all packages
make test    # run unit tests
make vet     # run go vet
make docs    # regenerate docs/ from source comments
```

Run `make help` for the full list of targets.

## How to integrate into another repo

To consume `simple-go` as a git submodule rather than a vendored copy:

1. **Add this repo as a git submodule**, at whatever path the consuming repo
   wants the sources to live:

   ```sh
   git submodule add https://github.com/radiospiel/simple-go simple-go
   ```

2. **Set up imports to point at the submodule's packages.** Each package
   lives under `src/` in this repo, so import it as
   `github.com/radiospiel/simple-go/src/<package>` (e.g.
   `github.com/radiospiel/simple-go/src/logger`). If the consuming repo
   previously vendored these packages under a different import path, every
   file importing them needs to be updated to the path above.

3. **Wire the module into `go.mod`** with a `replace` directive so builds
   always use the checked-out submodule content instead of fetching a
   published version:

   ```
   require github.com/radiospiel/simple-go v0.0.0-00010101000000-000000000000

   replace github.com/radiospiel/simple-go => ./simple-go
   ```

   Then run `go mod tidy`. Note that `simple-go` may require a newer Go
   toolchain than the consuming repo's `go.mod` currently declares (checked
   via minimal version selection across all dependencies); `go mod tidy` will
   bump the `go` directive automatically if so.

4. **Add a submodule-sync script** that initializes/updates the submodule
   and pushes back any local changes made inside it — so edits made to
   `simple-go` while working in the consuming repo aren't stranded — and
   wire it in as a prerequisite of the consuming repo's build and test
   steps (e.g. as a Makefile target dependency).

5. **Make sure CI checks out submodules.** This step is specific to GitHub
   Actions: `actions/checkout` doesn't fetch submodules by default, so every
   job that builds Go code needs:

   ```yaml
   - uses: actions/checkout@v4
     with:
       submodules: true
   ```

   Consider adding a CI check that fails if the submodule pin has drifted
   from `simple-go`'s `main` branch, to catch stale references:

   ```sh
   cd simple-go
   git fetch origin main
   [ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || exit 1
   ```

## License

BSD 3-Clause, see [LICENSE](LICENSE).
