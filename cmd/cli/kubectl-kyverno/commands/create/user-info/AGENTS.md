# AGENTS.md — cmd/cli/kubectl-kyverno/commands/create/user-info

This package implements the `kubectl-kyverno create user-info` CLI command, which generates valid `UserInfo` manifests (`cli.kyverno.io/v1alpha1`) for simulating user identities, roles, and groups in Kyverno CLI test suites.

## Architecture and Execution Flow

1. **Flag Parsing and Input Validation (`command.go`)**:
   - Parses flags: `--name` (`-n`), `--username` (`-u`), `--role` (`-r`), `--cluster-role` (`-c`), `--group` (`-g`), and `--output` (`-o`).
   - Validates `--name` against Kubernetes DNS-1123 subdomain rules using `k8s.io/apimachinery/pkg/util/validation.IsDNS1123Subdomain` prior to rendering.

2. **Buffered Template Execution**:
   - Compiles and evaluates `templates.UserInfoTemplate` using an in-memory buffer (`bytes.Buffer`).
   - Output is only written to destination (file or stdout) after template execution succeeds.

3. **Output Dispatch**:
   - Writes to file specified by `--output` using standard `0o644` permissions.
   - If `--output` is unset, writes to `cmd.OutOrStdout()`.

## Manifest and Template Contracts

- **Schema and Metadata**:
  - Generated manifests specify `apiVersion: cli.kyverno.io/v1alpha1`, `kind: UserInfo`, and `metadata.name`.
- **YAML Scalar Escaping**:
  - All user-supplied scalar strings (`.Name`, `.AdmissionUserInfo.Username`, and items in `.Roles`, `.ClusterRoles`, and `.AdmissionUserInfo.Groups`) are rendered using `{{ printf "%q" . }}` in `templates/user-info.yaml`.
  - Prevents YAML type coercion of reserved literals such as `no`, `off`, `null`, `true`, numbers, or strings containing colons from altering data types or producing unparsable manifests.
- **Conditional List Emission**:
  - `.Roles`, `.ClusterRoles`, and `.AdmissionUserInfo.Groups` are guarded by `{{- if ... }}` checks to omit empty lists and prevent keys from evaluating to `null`.

## Render-Before-Write and Safety Guarantees

- **Atomic Write & No Partial or Corrupted Output**:
  - The command never creates or truncates target output files if validation fails or template execution encounters an error.
  - File output is written to a temporary file in the destination directory (`os.OpenFile` with `0o644` so process umask applies) and atomically renamed into place, preventing partial writes or truncation of existing files.
- **Concurrency & Global Immutability**:
  - Tests must never mutate package-level `templates.UserInfoTemplate` across tests. Use `commandWithTemplate` to test template execution failure paths to maintain `-race` cleanliness.
- **Umask-Aware Permissions**:
  - File permission assertions in tests must compare with a reference file created with `0o644` in the same directory rather than comparing raw mode bits, ensuring tests pass under restrictive developer umasks.
