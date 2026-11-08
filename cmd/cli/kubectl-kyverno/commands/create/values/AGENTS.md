# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/values

This package implements the `kyverno create values` (and `kubectl-kyverno create values`) CLI command, which generates a Kyverno `values.yaml` manifest for policy evaluation and testing.

## Command Contract and Semantics

1. **Key-Value Splitting (`parseKeyValue`)**:
   - Values may contain equal signs (e.g., URLs, query parameters, expressions, base64 data).
   - Parsing uses `strings.SplitN(in, "=", 2)` so that any subsequent `=` characters belong to the value payload and are preserved intact.
   - Keys must not be empty. Malformed pairs without `=` or with empty keys return validation errors.

2. **Deduplication and Hierarchical Merging**:
   - **Policies**: Multiple `--rule` or `--resource` flags referencing the same policy name are merged under a single `v1alpha1.Policy` entry rather than duplicating policy blocks.
   - **Rules & Resources**: When multiple flags target the same rule or resource under a policy, their key-value pairs are merged into that rule/resource's `Values` map.
   - **Namespace Selectors**: Multiple `-n` / `--ns-selector` flags for the same namespace name are merged under a single `v1alpha1.NamespaceSelector` entry.

3. **Validation**:
   - `--rule` expects at least `<policy>,<rule>` followed by zero or more `<key>=<value>` pairs.
   - `--resource` expects at least `<policy>,<resource>` followed by zero or more `<key>=<value>` pairs.
   - `--ns-selector` expects `<namespace>,<key>=<value>...`.
   - Any malformed input returns a clear validation error rather than silently failing or dropping fields.

4. **Atomic Output Writing**:
   - Flag validation and template rendering execute in-memory before creating or modifying the output file (`-o/--output`). If validation fails, any pre-existing output file is preserved intact and never truncated.
