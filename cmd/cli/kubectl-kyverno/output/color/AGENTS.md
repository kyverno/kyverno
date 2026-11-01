# AGENTS.md — cmd/cli/kubectl-kyverno/output/color

This package implements semantic terminal coloring and formatting for `kubectl-kyverno` CLI output.

## Architecture and Global State

1. **Color Primitives & Semantic Helpers**:
   - Built on top of `ext/output/color` (which wraps `github.com/fatih/color`).
   - Wraps ANSI formatting into semantic CLI helpers:
     - Resource / Policy / Rule naming: `Policy(namespace, name)`, `Rule(name)`, `Resource(kind, namespace, name)`
     - Result statuses: `ResultPass()`, `ResultFail()`, `ResultWarn()`, `ResultError()`, `ResultSkip()`, `Excluded()`, `NotFound()`, `InvalidPolicy()`
     - Diagnostic helpers: `BoldGreen(...)`, `BoldRed(...)`, `BoldYellow(...)`, `BoldCyan(...)`, `FunctionName(name)`, `FunctionNote(note)`
     - Table styling: `HeaderBgColor`, `HeaderFgColor`

2. **Initialization and Linter Constraints**:
   - **No `func init()`**: Global color primitives in `ext/output/color` are declared and initialized directly at package level (`var ...`). The `gochecknoinits` linter strictly prohibits `func init()`.
   - **`Init(noColor bool)`**:
     - Delegates to `ext/output/color.Init(noColor, false)`, which re-instantiates package-level color objects (`BoldGreen`, `BoldRed`, `BoldYellow`, `BoldFgCyan`) and calls `c.DisableColor()` on each when `noColor` is `true`.
     - When `noColor` is `false`: re-enables table header colors (`HeaderBgColor = tablewriter.BgBlackColor`, `HeaderFgColor = tablewriter.FgGreenColor`).
     - When `noColor` is `true`: resets table header colors (`HeaderBgColor = 0`, `HeaderFgColor = 0`).
     - **Per-instance vs Global Suppression (`fatihcolor.NoColor`)**: `Init` only modifies its own package-level `Color` instances and table header styling; **it never writes to `fatih/color.NoColor`**. Callers cannot rely on `color.Init` to set or clear the global `fatihcolor.NoColor` boolean.

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

- **Global State Isolation**: Because tests often modify `fatihcolor.NoColor` directly (e.g. `fatihcolor.NoColor = false`) to enable ANSI codes, and `color.Init` configures per-instance colors and table header state, every test altering color settings must restore `fatihcolor.NoColor` and call `color.Init(origNoColor)` in `t.Cleanup`:
  ```go
  origNoColor := fatihcolor.NoColor
  t.Cleanup(func() {
      fatihcolor.NoColor = origNoColor
      color.Init(origNoColor)
  })
  ```
- **Deterministic Color Tests**: To ensure color output regardless of terminal detection or startup environment, tests asserting ANSI escape sequences must explicitly set `fatihcolor.NoColor = false`.
