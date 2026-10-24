# Testing

Run the following commands from the repository root with Go installed. The
tests use temporary configuration files and do not modify your SSH settings,
connect to servers or require Alfred or iTerm to be running.

## Unit and regression tests

```sh
go test ./...
```

For individual test results:

```sh
go test -v ./...
```

Test files live alongside the Go implementation in the repository root, so
Go can test private functions in package `main` directly. No wrapper or extra
setup is required.

The test sources are:

- `utils_test.go` covers nested includes, multiple paths, wildcard ordering,
  relative paths, quoted paths, multiple aliases, duplicate aliases, symlink
  cycles, missing files and malformed input.
- `security_test.go` covers literal shell syntax in include paths, command
  substitution, shell argument escaping, invalid aliases, leading SSH option
  markers, hidden characters, invalid UTF-8, malformed workflow arguments,
  directories, named pipes and resource limits.

Shell-escaping tests run a local shell with temporary marker files and an
intercepted `ssh` function. They verify that payloads are treated as data and
that generated commands pass exactly one hostname after `--`; no SSH connection
is made. Discovery tests also verify that `Match exec`, `ProxyCommand` and
`LocalCommand` are not executed while collecting aliases.

Named-pipe and shell tests target Unix systems, including macOS, where the
workflow runs.

## Fuzz testing

The tokenizer and workflow argument validation can be fuzz-tested:

```sh
go test -fuzz=FuzzSSHConfigWords -fuzztime=5s -parallel=2
```

This runs the regression tests first, then fuzzes tokenization and checks that
accepted aliases cannot inject extra lines into the AppleScript argument.
The seed corpus includes quotes, escapes, command substitutions, control
characters, invisible Unicode characters and invalid UTF-8.

Increase `-fuzztime` for a longer run. Fuzzing complements the explicit
regression tests; it does not prove the absence of all security issues.

## Static checks

```sh
go vet ./...
```

Return to the [project README](../README.md).
