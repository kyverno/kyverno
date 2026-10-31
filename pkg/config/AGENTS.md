# AGENTS.md — pkg/config

Runtime configuration for Kyverno components, loaded dynamically from the `kyverno` ConfigMap (or custom name/namespace). See [ARCHITECTURE.md](../../ARCHITECTURE.md) for how runtime configuration feeds controllers and admission webhooks.

## The Load / Unload Lifecycle Contract

The `Configuration` interface provides thread-safe access to runtime configuration settings (`Load(cm *corev1.ConfigMap)`).

- **`NewDefaultConfiguration(skipResourceFilters bool)`** initializes the in-memory configuration with canonical defaults before any ConfigMap is observed.
- **`load(cm *corev1.ConfigMap)`** parses configuration keys from `cm.Data`, validates their syntax, and sets runtime fields.
- **`unload()`** is called by `Load(nil)` when the ConfigMap is deleted or missing from the cluster.

### Critical Invariant: Prevent State Retention Bugs

When the ConfigMap is deleted or unloaded, **every configurable setting must be explicitly reset to its canonical default value in `unload()`**.

Failing to reset a field in `unload()` causes **state retention bugs** (e.g. [#17406](https://github.com/kyverno/kyverno/issues/17406)):
- If a user configures a setting (e.g. `matchConditions` or `updateRequestThreshold`) and later deletes or resets the ConfigMap, omitting that field from `unload()` leaves stale values active in memory indefinitely.
- Omitting default initialization in `NewDefaultConfiguration()` leaves uninitialized zero-values until the first ConfigMap sync.

### Checklist for Adding or Modifying a Configuration Setting

When introducing a new configuration option to `pkg/config`:

1. **Define Constants**: Add the ConfigMap key constant and default value constant (if applicable) in `config.go`.
2. **Initialize in `NewDefaultConfiguration`**: Explicitly set the default value in `NewDefaultConfiguration()` so unconfigured instances are safe before the first load.
3. **Parse in `load()`**:
   - Log at V(2) when the key is not set.
   - When present, parse and validate the value.
   - If invalid, log an error referencing the correct setting name (avoid copy-paste typos) and fall back safely.
   - If valid, assign the parsed value to the configuration struct and log confirmation at V(2).
4. **Reset in `unload()`**: Explicitly restore the setting to its default value (`nil`, empty string, `false`, or default constant) in `unload()`.
5. **Add Unit Tests in `config_test.go`**:
   - Verify `NewDefaultConfiguration()` initializes the setting to its default.
   - Verify `Load(cm)` correctly applies valid values.
   - Verify `Load(cm)` handles invalid values safely.
   - Verify `Load(nil)` (unload) resets the setting back to its default.
6. **Mocks**: If a new getter is added to the `Configuration` interface, update `pkg/config/mocks/mock_config.go` (`make codegen-client-all`).
