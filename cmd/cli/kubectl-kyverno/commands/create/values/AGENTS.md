# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/values

This package implements the `kyverno create values` (and `kubectl-kyverno create values`) CLI command, which generates a Kyverno `values.yaml` manifest for policy evaluation and testing.

## Command Contract and Semantics

1. **Key-Value Splitting (`parseKeyValue`) & Flag Handling**:
   - Values may contain equal signs (e.g., URLs, query parameters, expressions, base64 data).
   - Parsing uses `strings.SplitN(in, "=", 2)` so that any subsequent `=` characters belong to the value payload and are preserved intact.
   - Keys must not be empty. Malformed pairs without `=` or with empty keys return validation errors.
   - Empty values are rendered explicitly as double-quoted strings (`""`) so that YAML unmarshaling preserves them as empty strings rather than `null` scalars.
   - `--global` uses `StringArrayVarP` (consistent with `--rule`, `--resource`, and `--ns-selector`) so pflag does not split comma-separated values (e.g., query strings in URLs like `https://example.com?a=1,b=2`).

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

5. **YAML Scalar Encoding (`formatValue` and `parseLabelKeyValue`)**:
   - Values containing sequences or characters sensitive in YAML plain scalars (such as inline comments ` #` or `\t#`, leading indicators `#`, `*`, `&`, `[`, `{`, `]`, `}`, `|`, `>`, `%`, `@`, `` ` ``, `!`, `?`, `- `, or mapping indicators `: `, line breaks, or leading/trailing whitespace) are double-quoted so they are not truncated as comments, parsed as mappings/sequences/aliases, or altered on unmarshal.
   - Other policy/rule scalars (numbers, booleans, null, and plain strings) remain unquoted to preserve native scalar typing.
   - In contrast, namespace selector labels (`parseLabelKeyValue`) are always explicitly double-quoted strings to strictly conform to Kubernetes `map[string]string` typing, preventing YAML unmarshal errors for boolean (`env=true`) or numeric (`tier=1`) label values.

6. **Empty Rule and Resource Values**:
   - Rules and resources specified without key-value pairs (e.g., `--rule mypol,rule1` or `--resource mypol,res1`) render `values: {}` in the YAML template rather than leaving `values:` empty, preventing YAML `null` map deserialization issues.
