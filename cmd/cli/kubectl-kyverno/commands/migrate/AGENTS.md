# AGENTS.md — cmd/cli/kubectl-kyverno/commands/migrate

This package implements the `kubectl-kyverno migrate` CLI command, which migrates existing Custom Resource instances in a Kubernetes cluster to the storage version defined in their CustomResourceDefinition (CRD).

## Architecture & Execution Flow

1. **Command Wiring & Flag Parsing (`command.go`)**:
   - `Command() *cobra.Command`: registers options for `--kubeconfig`, `--context`, `--resource` (required), and `--remove-color`.
   - `RunE`:
     - Initializes CLI color configuration via `color.Init(options.RemoveColor)`.
     - Validates and trims resource names.
     - Constructs client configurations via `config.CreateClientConfigWithContext`.
     - Instantiates the apiextensions clientset (`clientset.NewForConfig`) and dynamic client (`dynamic.NewForConfig`).
     - Obtains the configured output writer (`cmd.OutOrStdout()`).
     - Iterates through requested CRDs, fetches each via `apiServerClient.ApiextensionsV1().CustomResourceDefinitions().Get(...)`, and invokes `migrate(...)`.

2. **Migration Logic (`migrate`)**:
   - Locates the storage version (`storage: true`) from `crd.Spec.Versions`. Returns an error if not found.
   - **No-Op Short-Circuit**: If `len(crd.Status.StoredVersions) == 1` and already equals the storage version name, prints the up-to-date message and exits without modifying resources.
   - **Resource Iteration**: Lists existing resources (cluster-scoped or across all namespaces depending on `crd.Spec.Scope`) using the dynamic client, then calls `client.Update(...)` on each item to re-persist it under the new storage version.
   - **Status Update**: Upon successful migration of all items, updates `crd.Status.StoredVersions` with the target version via `UpdateStatus`.

## Output Styling & Invariants

1. **Output Writer Routing**:
   - All diagnostic and progress messages must be written to the supplied `io.Writer` (`cmd.OutOrStdout()`), never directly to `fmt.Println` or `os.Stdout`, ensuring testability and honoring Cobra output redirection.

2. **Semantic Color Highlighting**:
   - **Bold Cyan** (`color.BoldCyan`): Resource names and stored version identifiers.
   - **Bold Green** (`color.BoldGreen`): Positive / up-to-date status (`stored version is already up to date, nothing to do`).
   - **Bold Yellow** (`color.BoldYellow`): In-progress operations (`migrating resources...`, `patching status...`).
   - When `--remove-color` is set, all output must be clean plain text free of ANSI escape sequences (`\x1b[...]`).

3. **Migration Atomicity Invariant**:
   - `crd.Status.StoredVersions` must only be updated after all existing resource updates succeed. If any `List` or `Update` operation fails, `UpdateStatus` must not be called, and the error must propagate.

## Testing Invariants

- **Environment & Global State Isolation**:
  - Tests expecting ANSI output must clear `NO_COLOR` using `t.Setenv("NO_COLOR", "")` to prevent ambient environment contamination.
  - Tests that toggle `fatihcolor.NoColor` must restore original state and reinitialize colors in `t.Cleanup`:
    ```go
    t.Setenv("NO_COLOR", "")
    origNoColor := fatihcolor.NoColor
    t.Cleanup(func() {
        fatihcolor.NoColor = origNoColor
        color.Init(origNoColor)
    })
    ```
- **Coverage Requirements**:
  - End-to-end command wiring must be exercised through `Command().Execute()` with a captured buffer and a mock/test server.
  - Both happy paths (already up-to-date and active migration) and failure paths (List error, Update error) must be verified with color enabled and with `--remove-color`.

