# AGENTS.md — cmd/cli/kubectl-kyverno/output/color

This package implements semantic terminal coloring and formatting for `kubectl-kyverno` CLI output.

## Architecture and Global State

1. **Color Primitives & Semantic Helpers**:
   - Built on top of `github.com/fatih/color`.
   - Wraps raw ANSI formatting into semantic color helpers:
     - Policy/Rule naming: `Policy(prefix, name)`, `Rule(name)`, `RuleFail(name)`
     - Status diagnostics: `BoldGreen(...)`, `BoldRed(...)`, `BoldYellow(...)`, `BoldCyan(...)`
     - Table styling: `HeaderBgColor`, `HeaderFgColor`

2. **Initialization and Linter Constraints**:
   - **No `func init()`**: Global color primitives are declared and initialized directly at package level (`var ...`). The `gochecknoinits` linter strictly prohibits `func init()`.
   - **`Init(removeColor bool)`**:
     - When `removeColor` is `true`: calls `fatihcolor.NoColor = true` and resets table header colors (`HeaderBgColor = 0`, `HeaderFgColor = 0`).
     - When `removeColor` is `false`: re-enables table header colors and sets `fatihcolor.NoColor = false` (unless the user environment specifies `NO_COLOR`).

## Structured-Output and Color Suppression Invariants

1. **Machine-Readable Streams**:
   - Any CLI execution mode producing structured, non-human, or piped output must explicitly suppress color by calling `color.Init(true)` or forcing `removeColor = true`.
   - This invariant applies to:
     - `--policy-report` (ClusterReport / PolicyReport YAML/JSON)
     - `--generate-exceptions` (PolicyException YAML manifests)
     - `--stdin` (mutated resources piped to downstream commands such as `kubectl`)
   - **Invariant**: ANSI escape sequences (`\x1b[...]`) must NEVER contaminate machine-readable or piped payloads.

2. **Pre-Execution & Diagnostic Outputs**:
   - Early diagnostic outputs (e.g. `printSkippedAndInvalidPolicies`, mutation/generation rule error/skip messages) must honor color suppression so that structured output streams remain valid parseable YAML/JSON.

## Testing Invariants

- **Global State Isolation**: Because `fatihcolor.NoColor` and `color.Init` mutate global state, every test altering color settings must restore the original state in `t.Cleanup`:
  ```go
  origNoColor := fatihcolor.NoColor
  t.Cleanup(func() {
      fatihcolor.NoColor = origNoColor
      color.Init(origNoColor)
  })
  ```
- **Deterministic Color Tests**: To prevent test flakiness across developer machines and CI environments where `NO_COLOR` might be set, tests asserting ANSI escape sequences must explicitly clear `NO_COLOR` with `t.Setenv("NO_COLOR", "")` and set `fatihcolor.NoColor = false`.
