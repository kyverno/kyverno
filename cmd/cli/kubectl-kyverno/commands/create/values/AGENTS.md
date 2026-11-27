# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/values

This package implements the `kyverno create values` CLI command.

## Key Package Invariants and Gotchas

1. **Namespace Labels vs. Rule/Resource Values**:
   - Namespace selector labels (`map[string]string`) are always quoted so that numeric, boolean, or special-character label values strictly deserialize as strings in Kubernetes manifests.
   - Global values, rule values, and resource values preserve native scalar typing (integers, booleans, floats, null) while escaping and quoting strings via a standard YAML encoder.

2. **Flag Delimiters**:
   - Comma (`,`) is the delimiter for `--rule`, `--resource`, and `--ns-selector` entries (e.g. `<policy>,<rule>,<key>=<value>`). Names cannot contain commas because commas delimit fields.
   - `--global` uses `StringArrayVarP` (preserving commas within values; multiple `-g` flags for separate values).
