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

## License

BSD 3-Clause, see [LICENSE](LICENSE).
// sync-submodules test marker
