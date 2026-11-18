# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/test

This package implements the `kubectl-kyverno create test` command, which scaffolds declarative `Test` manifests (`kyverno-test.yaml`) under the `cli.kyverno.io/v1alpha1` API schema.

## Command Responsibilities and Behavior

1. **Flag Parsing and Input Validation (`command.go`)**:
   - Accepts test metadata flags: `--name` (`-n`), `--policy` (`-p`), `--resource` (`-r`), `--values` (`-f`), and output path `--output` (`-o`).
   - Accepts expected outcome entries through `--pass`, `--fail`, `--warn`, `--error`, and `--skip`.
   - Result flags use `StringArrayVar` to allow repeated specifications.
   - Every result argument is validated by `parseResult`.

2. **Result Entry Specification Contract**:
   - Each result entry is a comma-separated string with either 5 or 6 fields:
     `policy,[rule],resource,[namespace],kind[,patchedResource]`
   - **`policy`** (required): Policy name. Must not be empty.
   - **`rule`** (optional): Rule name. Can be empty `""` (e.g. `policy,,resource,namespace,kind`) to accommodate ruleless policies such as CEL `ValidatingPolicy` or `MutatingPolicy`.
   - **`resource`** (required): Target resource name. Must not be empty.
   - **`namespace`** (optional): Target resource namespace. Empty indicates a cluster-scoped resource.
   - **`kind`** (required): Resource kind (e.g., `Pod`, `Deployment`, `ClusterRole`). Must not be empty.
   - **`patchedResource`** (optional 6th field): File path containing expected mutated resource YAML.
   - **Fail-Fast Validation**:
     `parseResult` returns `nil` if `len(parts) < 5 || len(parts) > 6` or if `policy`, `resource`, or `kind` is empty. The command returns an error on invalid entries rather than silently skipping them.

3. **Template Rendering and Manifest Schema (`templates/test.yaml`)**:
   - Generated manifests adhere to the modern `cli.kyverno.io/v1alpha1` Test schema:
     ```yaml
     apiVersion: cli.kyverno.io/v1alpha1
     kind: Test
     metadata:
       name: {{ printf "%q" .Name }}
     ```
     Manifests must never emit the deprecated top-level `name: {{ .Name }}` header.
     `metadata.name` is safely quoted via `%q` so that numeric or boolean names (e.g. `123`, `true`) deserialize as string.
   - **Resource entries**: Rendered under `resources:` as a list. When `namespace` is specified, entries are emitted as `"<namespace>/<resource>"`. When namespace is empty, entries are emitted as `"<resource>"`. All resource identifiers are safely quoted via `%q`.
   - **Conditional rule rendering**: The `rule:` key is emitted only when `.Rule` is non-empty (`{{- if .Rule }}`), omitting empty rule fields for ruleless policies.
   - **Conditional patchedResources rendering**: `patchedResources: {{ printf "%q" .PatchedResource }}` is emitted only when a 6th field is provided.

## Verification Requirements

When modifying this package:
- Ensure generated manifests load cleanly via `test.LoadTest(nil, path)` and do not trigger deprecation warnings via `deprecations.CheckTest(nil, path, test)`.
- Verify all result status types (`pass`, `fail`, `warn`, `error`, `skip`) and 5/6-part syntax are covered by unit tests in `command_test.go`.
