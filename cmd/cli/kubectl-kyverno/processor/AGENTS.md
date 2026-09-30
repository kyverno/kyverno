# AGENTS.md — cmd/cli/kubectl-kyverno/processor

`PolicyProcessor` is shared by `kyverno test` and `kyverno apply`. Behavior changes here affect both commands.

## MutatingPolicy: trigger vs. target

`ApplyPoliciesOnResource` evaluates MutatingPolicies in two separate passes:

1. **Trigger pass** — `eng.Handle(...)` runs against `p.Resource`. It is filtered with
   `mpolengine.NoTargetMatchConstraintPolicy()`, the same predicate the admission handlers use in
   `pkg/webhooks/resource/mpol/handler.go`. Policies with active `targetMatchConstraints` (`resourceRules` or
   `expression`) must never mutate the trigger here. Policies with no or empty `targetMatchConstraints` mutate the
   trigger inline.
2. **Target pass** — only runs when `p.TargetResources` is set. Each target is evaluated with `Evaluate(...)` and
   `targetMatchPredicate`; the patched target is attached to each rule with `WithPatchedTarget`, while the
   response's `Resource` stays the trigger.

Both callers must populate `TargetResources`: `commands/test` from `targetResources` in `kyverno-test.yaml`, and
`commands/apply` from `--target-resource` / `--target-resources`. Without it, target-only policies produce no result.

Regression tests: `TestRunTest_MutatingPolicyTargetMatchConstraintsDoNotMutateTrigger` (`commands/test`) and
`Test_Apply_MutatingPolicyTargetResources` (`commands/apply`).
