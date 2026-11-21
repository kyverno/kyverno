# AGENTS.md — cmd/cli/kubectl-kyverno/commands/docs

This package implements `kubectl-kyverno docs`, which writes Markdown reference pages for the CLI command tree to the directory given by `--output`.

## Output file permissions

- The output directory is created with `0o750` only when it does not already exist. An existing directory keeps its mode.
- The Markdown files are written by `genMarkdownTree` in `options.go`, not by `doc.GenMarkdownTreeCustom` from `github.com/spf13/cobra/doc`. Cobra's function creates each file with `os.Create` (`0666`), so with a permissive umask (for example `000`) the generated files would be group- and world-writable. `genMarkdownTree` is the same walk, but opens each file with `os.OpenFile(..., os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)`.
- **Invariant**: do not switch back to `doc.GenMarkdownTreeCustom` or `os.Create` for output files, and do not use `os.ModePerm`. `gosec` (G301/G306) flags numeric literals such as `0777` but not the `os.ModePerm` constant, so the linter will not catch that regression.
- As with any `O_TRUNC` open, an existing file keeps its mode; `0o600` applies only to files that are created.

## Keeping `genMarkdownTree` in sync with Cobra

`genMarkdownTree` copies the traversal and naming of `doc.GenMarkdownTreeCustom`: subcommands first, skipping commands that are not available or are additional help topics, and naming each file after `CommandPath()` with spaces replaced by `_` plus `.md`. Page content still comes from `doc.GenMarkdownCustom`. When bumping `github.com/spf13/cobra`, compare against upstream `doc/md_docs.go`. `TestGenMarkdownTreeMatchesCobra` fails if the file names or content diverge.

## Testing

- `options_test.go` is built only on non-Windows platforms (`//go:build !windows`) because it relies on Unix permission bits and `syscall.Umask`.
- `TestExecuteRestrictsOutputPermissions` re-runs the test binary as a child process and sets umask `0` there, so the result does not depend on the umask of the environment running the tests. Do not call `syscall.Umask` in the parent test process: the umask is process-wide and would affect tests running in parallel.
