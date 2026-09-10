# AGENTS.md — pkg/toggle

The single sanctioned feature-flag mechanism. Two files: `toggle.go` (flag declarations + parsing) and `context.go`
(the `Toggles` interface + context DI seam).

## There are two different kinds of flag — don't mix them up

- **Bool toggles** go through the `Toggles` interface in `context.go`.
- **String-slice flags** (`HTTPBlocklist`, `HTTPAllowlist`) are read directly via `toggle.HTTPBlocklist.Values()` /
  `.Reset()` — they are **not** part of the `Toggles` interface and never go through `FromContext`. If your new flag
  is a list, follow the `StringSliceFlag` pattern in `toggle.go`, not the steps below.

## Adding a new bool toggle — the complete, verified checklist

Using `ProtectManagedResources` as the worked example (a flag that's fully wired end-to-end):

1. `toggle.go`: add the 4 consts (`<Name>FlagName`, `<Name>Description`, unexported `<name>EnvVar`, unexported
   `default<Name>`).
2. `toggle.go`: add the package var — `<Name> = newToggle(default<Name>, <name>EnvVar)`.
3. `context.go`: add the method to the `Toggles` interface — `<Name>() bool`.
4. `context.go`: implement it on `defaultToggles` — `func (defaultToggles) <Name>() bool { return <Name>.Enabled() }`.
5. **Register the CLI flag in every `cmd/*/main.go` binary that needs it.** This step is NOT centralized — each
   binary opts in individually with `flagset.Func(toggle.<Name>FlagName, toggle.<Name>Description, toggle.<Name>.Parse)`.
   `ProtectManagedResources` is registered in `cmd/kyverno/main.go` and `cmd/cleanup-controller/main.go` because
   only `pkg/controllers/cleanup` and `pkg/controllers/deleting` consume it. `AllowHTTPInNamespacedPolicies` /
   `HTTPBlocklist` / `HTTPAllowlist` are registered in **four** binaries because CEL HTTP calls happen in all of
   them. **Forgetting this step is the most common way to break a new toggle** — it compiles fine and the interface
   method works, but the flag is simply unreachable from any binary's CLI/env. `AutogenV2` is exactly this: it has
   consts, a var, an interface method, and a `defaultToggles` impl, but is registered in **no** binary and consumed
   nowhere — don't use it as a "this is how it's done" reference; use `ProtectManagedResources`.
6. Consume it at call sites via `toggle.FromContext(ctx).<Name>()`.

**Exception — the one centralized flag**: `EnableDeferredLoading` is wired once through `cmd/internal`'s shared
`Configuration` interface/`With<Feature>()` option pattern instead of being hand-copied per binary. Only follow this
pattern if the new toggle should apply uniformly to every binary that opts into that shared configuration path —
otherwise use the per-binary registration above.

## The context DI seam is barely used outside tests

`toggle.NewContext(...)` is never called in production code — only in `toggle_test.go`/`context_test.go`. In real
binaries, `FromContext(ctx)` always falls through to `defaultToggles{}`, which just reads the package-level
singleton vars set by CLI flag/env var. So `FromContext(ctx).X()` is functionally `X.Enabled()` in production; the
interface exists so tests can inject a fake `Toggles`.

## Adding an interface method requires updating the test mock too

`context_test.go` defines a hand-rolled `mockToggles` struct implementing every `Toggles` method — no generated
mock, no testify mock. Add your new method there too or the package won't compile. There's no shared/exported
toggle-mocking fixture for other packages' tests — outside code either sets env vars (`t.Setenv`) or calls
`toggle.<Name>.Parse(...)` on the real singleton directly (bool toggles have no reset helper, unlike
`StringSliceFlag.Reset()`).
