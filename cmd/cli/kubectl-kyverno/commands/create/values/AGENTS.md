# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/values

This package implements the `kyverno create values` (and `kubectl-kyverno create values`) CLI command, which generates a Kyverno `values.yaml` manifest for policy evaluation and testing.

## Command Contract and Semantics

1. **Key-Value Splitting (`parseKeyValue`)**:
   - Values may contain equal signs (e.g., URLs, query parameters, expressions, base64 data).
   - Parsing uses `strings.SplitN(in, "=", 2)` so that any subsequent `=` characters belong to the value payload and are preserved intact.
   - Keys must not be empty. Malformed pairs without `=` or with empty keys return validation errors.
   - Empty values are rendered explicitly as double-quoted strings (`""`) so that YAML unmarshaling preserves them as empty strings rather than `null` scalars.

2. **Deduplication and Hierarchical Merging**:
   - **Policies**: Multiple `--rule` or `--resource` flags referencing the same policy name are merged under a single `v1alpha1.Policy` entry rather than duplicating policy blocks.
   - **Rules & Resources**: When multiple flags target the same rule or resource under a policy, their key-value pairs are merged into that rule/resource's `Values` map.
   - **Namespace Selectors**: Multiple `-n` / `--ns-selector` flags for the same namespace name are merged under a single `v1alpha1.NamespaceSelector` entry.

3. **Validation**:
   - `--rule` expects at least `<policy>,<rule>` followed by zero or more `<key>=<value>` pairs.
   - `--resource` expects at least `<policy>,<resource>` followed by zero or more `<key>=<value>` pairs.
   - `--ns-selector` expects `<namespace>,<key>=<value>...`.
   - Any malformed input returns a clear validation error rather than silently failing or dropping fields.

4. **Failure-Safe Output Writing**:
   - Flag validation and template rendering execute in-memory before writing the output file (`-o/--output`).
   - Output files are written and closed via a temporary file in the destination directory and atomically renamed into place. If validation, write, or close fails, pre-existing output files remain intact and untruncated.
   - Resolved symlink destinations and pre-existing file permissions are preserved intact (dangling symlinks return an error, and newly created files default to restrictive 0600 permissions).

5. **YAML Scalar Encoding (`formatValue`)**:
   - Values containing sequences or characters sensitive in YAML plain scalars (such as inline comments ` #` or `\t#`, leading indicators `#`, `*`, `&`, `[`, `{`, `]`, `}`, `|`, `>`, `%`, `@`, `` ` ``, `!`, `?`, `- `, or mapping indicators `: `, line breaks, or leading/trailing whitespace) are double-quoted so they are not truncated as comments, parsed as mappings/sequences/aliases, or altered on unmarshal.
   - Other scalars (numbers, booleans, null, and plain strings) remain unquoted to preserve native scalar typing.
