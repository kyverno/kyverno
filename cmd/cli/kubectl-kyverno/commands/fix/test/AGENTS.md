# AGENTS.md — cmd/cli/kubectl-kyverno/commands/fix/test

This package implements `kubectl-kyverno fix test`, which loads CLI test files (`kyverno-test.yaml` by default, set with `--file-name`), upgrades them with the helpers in `cmd/cli/kubectl-kyverno/fix`, and prints warnings for what it changed.

## Save behavior

- Nothing is written unless `--save` is set. With `--save`, a file is rewritten only when the fixed object differs from the loaded one, or when `--force` is set.
- Three kinds of files can be saved, each to the path it was loaded from: the test file itself, the user info file referenced by `userinfo`, and the values file referenced by `variables`.
- A test file can contain several YAML documents; they are written back to the same file separated by `---`.

## Output file permissions

- All three writes use `os.WriteFile(path, yamlBytes, 0o600)`, matching the file mode used elsewhere in the CLI.
- `os.WriteFile` applies the mode only when it creates a file. Today every save path rewrites a file that was just loaded, so existing permissions are kept and the mode is a safeguard for any future code path that creates new files.
- **Invariant**: do not use `os.ModePerm` or a group/world-writable mode for these writes. `options.go` starts with a file-level `//nolint:gosec`, so `gosec` does not check this file at all, and it would not flag `os.ModePerm` anyway (G306 only matches numeric literals). Review mode changes here by hand.
