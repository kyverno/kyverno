# AGENTS.md — pkg/engine

The legacy (`kyverno.io`) rule-evaluation engine. No package doc comment exists anywhere in this tree — this file is
the closest thing to one. See [ARCHITECTURE.md](../../ARCHITECTURE.md) for how this relates to `pkg/cel` (the
parallel engine for `policies.kyverno.io`, not layered under this package).

## Handler dispatch is not a registry

`pkg/engine/handlers/handler.go` defines one `Handler` interface every rule-type handler implements
(`Process(ctx, logger, PolicyContext, resource, Rule, EngineContextLoader, exceptions) (patched, []RuleResponse)`).
Which handler runs is decided by an if/else chain built per rule in `validation.go`/`mutation.go`, not a lookup
table — for validate: `hasVerifyManifest > hasValidatePss > hasValidateCEL > default validateResource`.
`ApplyRules == ApplyOne` short-circuits after the first rule with a non-zero applied count.

**Rules in the same policy see each other's patches.** `engine.go`'s `invokeRuleHandler` checkpoints the JSON
context, evaluates the rule, restores the checkpoint, then re-adds the *patched* resource — so rule 2 in a policy
sees rule 1's mutation in its JMESPath context, not the original resource.

**`mutateExisting` rules are silently skipped during admission, on purpose:**
```go
// For mutateExisting rules during admission, skip mutation.
// The mutation will be applied by the background controller via UpdateRequest.
// This prevents the trigger resource from being mutated during admission.
if policyContext.AdmissionOperation() && rule.HasMutateExisting() {
    return nil, nil
}
```
(`mutation.go`). Outside admission (i.e. the background pass), it requires a non-nil client or errors. If you're
debugging "why didn't my mutateExisting rule do anything during the webhook call" — this is why; it's deferred to
`pkg/background`, not a bug.

## Variable substitution order matters

`pkg/engine/variables/vars.go`'s `substituteAll` always runs **reference substitution (`$(...)`) before variable
substitution (`{{ }}`)** — chained reference→variable patterns depend on this order. Two more substitution-time
gotchas easy to miss when writing/testing policies:
- The `@` self-reference symbol (used in `anyPattern`/`foreach`) resolves to `target` if available, else falls back
  to `request.object`.
- For **DELETE** admission requests, any variable path containing `request.object` is silently rewritten to
  `request.oldObject`.

## Mutation: strategic-merge and JSON6902 are mutually exclusive per rule

`mutate/mutation.go`'s `NewPatcher`: if `patchStrategicMerge` is set, `patchesJson6902` is ignored entirely — never
both applied on one rule. If the patched bytes end up byte-identical to the original, the result is
`RuleStatusSkip` ("no patches applied"), not `Pass` — don't count skip as success in tooling. A failed anchor/global
condition in a strategic-merge overlay produces a silent no-op patch (`{}`), not an engine error
(`mutate/patch/strategicMergePatch.go`). JSON6902 `add` ops auto-create missing intermediate paths/arrays rather
than failing (see `mutate/patch/patchJSON6902_test.go`'s `Test_MissingPaths`) — real, tested behavior worth knowing
before assuming an `add` to a non-existent path will error.

## Context (`context`, `policycontext`) has no global state — by design

`PolicyContext` (`policycontext/policy_context.go`) is a plain per-request value object with no package-level
mutable state; everything is threaded explicitly and copied via `With*` builders. `context.Context`'s
`Checkpoint()`/`Restore()` implement a manual snapshot stack, used both per-policy and per-rule — **nested
checkpoints**, so a missing `Restore()` leaks context mutations into subsequent rules or policies. This is
copy-on-write discipline, not accidental — don't add a package-level var to work around a threading inconvenience.

## `pkg/engine/apicall` is a security-sensitive, actively-patched area

External API/context calls (used to resolve variables) go through a hardened executor with real SSRF/DNS-rebinding
protection: URL/hostname allow-and-block lists checked before DNS resolution, then **every resolved IP** re-checked
against blocked CIDRs (including IPv4-mapped/NAT64/6to4 IPv6 spellings of cloud metadata IPs like
`169.254.169.254`), and a single dial timeout spanning DNS resolution *and* connection to close the TOCTOU window
where the resolved IP changes between check and connect. The blocked-CIDR error message deliberately omits the
resolved IP — surfacing it through denial/PolicyReport/Events text would turn a policy denial into an oracle for
resolving arbitrary hostnames. Recent history (`825311802`, `fc1ffa919`, `8b6686abf`, `3c6b941f0`) shows this exact
area got patched four times in quick succession — treat any change here as security-review-worthy, not a routine
tweak. Also: `egressLogger()` builds its logger lazily per call rather than as a package var, because a package-level
logger would be created before `logging.Setup()` runs and would silently discard everything written to it.

## `pkg/engine/api` core types are immutable value types

`EngineResponse` and `RuleResponse` are only ever modified via `With*` copy methods, never mutated in place —
matching the copy-on-write style of `PolicyContext`. `RuleResponse.emitWarn` is auto-set `true` for
`Error|Fail|Warn` statuses by `NewRuleResponse`.

## Known open TODOs worth knowing before you hit them

- `forceMutate.go`: "if we apply autogen, tests will fail" (unresolved).
- `internal/imageverifier.go`: `opts.IgnoreSCT = true` is hardcoded, pending an option to allow SCT when attestors
  aren't provided.
- `anchor/error.go`: a comment admits an error-propagation workaround shouldn't be needed but currently is.
