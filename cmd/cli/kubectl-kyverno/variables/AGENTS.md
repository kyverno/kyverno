# AGENTS.md — cmd/cli/kubectl-kyverno/variables

This package resolves the variables that the `apply` and `test` commands inject into policy evaluation. Values come from two sources: a values file (`--values-file` / the test's `values` block, loaded by `new.go`) and ad-hoc `--set key=value` pairs (parsed by `parse.go`).

## `parse()` invariants (`parse.go`)

- Each entry is split on the **first** `=` only (`strings.Cut`), so a value may itself contain `=` (URLs with query strings, base64 padding, connection strings, CEL expressions). Do not go back to `strings.Split`.
- Entries without `=`, with an empty key, or with an empty value are ignored, not errors. The CLI logs them at verbosity 3.
- Keys containing `request.object.` are ignored: callers must not be able to override the admission request object through variables.
- The first occurrence of a key wins; duplicates are ignored.
- An empty result returns `nil`, not an empty map, and callers rely on that.

## Where the parsed variables go

- `Variables.ComputeVariables` merges values-file globals, per-resource values and the `--set` map into the variable set handed to legacy (`kyverno.io`) policy evaluation. On `main`, `kyverno apply` hard-blocks legacy policy kinds, so CEL policies do not read `--set` values; the parser is still shared with the `test` command and with release branches that evaluate legacy policies.

## Tests

- `parse_test.go` covers the parsing rules above, including values that contain `=`.
