# AGENTS.md — cmd/cli/kubectl-kyverno/processor

`PolicyProcessor` is shared by `kyverno test` and `kyverno apply`. Behavior changes here affect both commands.

## MutatingPolicy: trigger vs. target

`ApplyPoliciesOnResource` evaluates MutatingPolicies in two separate passes:

1. **Trigger pass** — `eng.Handle(...)` runs against `p.Resource`. It is filtered with
   `mpolengine.NoTargetMatchConstraintPolicy()`, the same predicate the admission handlers use in
   `pkg/webhooks/resource/mpol/handler.go`. Policies with active `targetMatchConstraints` (`resourceRules` or
   `expression`) must never mutate the trigger here. Policies with no or empty `targetMatchConstraints` mutate the
   trigger inline.
2. **Target pass** — only runs when `p.TargetResources` is set, and only for the policies whose trigger rules match
   this resource: `triggeredMutateExistingPolicies` (`mutate_existing.go`) checks `matchConstraints` and
   `matchConditions`, because the target engine has no matcher and never looks at the trigger. It is the check
   the webhook makes through `MatchedMutateExistingPolicies`, without that method's admission-enabled gate, so
   background-only policies keep mutating their targets as the background controller would. Like the admission
   matcher, a namespaced policy is only considered for a trigger in its own namespace. The target pass then keeps
   only those policies with `policyKeyIn`, an exact `mpolengine.PolicyKey` (namespace/name) match. Do not use
   `MatchNames` here: it also matches by bare name, so a triggered clustered policy would let a same-named
   namespaced policy from another namespace through. Targets selected by a CEL expression (`discoverCELTargets`)
   are stored per policy key and per `celTargetKey` (group, kind, namespace and name), so policies with the same
   name never share selections and a selected ConfigMap never matches a Secret with the same name. Only the
   triggered policies have their CEL target expression evaluated, so an expression written for another trigger
   never runs against this one, where it could fail and abort the whole run.
   Each target is evaluated with `Evaluate(...)`, `policyKeyIn(triggered)` and `targetMatchPredicate`, and gets
   its own response whose `Resource` is the target. A passing rule carries the
   patched target through `WithPatchedTarget`; a rule that fails or errors carries no patched target and is
   counted, never printed as mutated. `processMutateExistingEngineResponse` prints only the patched targets
   (`printOutput` with a nil resource), so the trigger is never written out as if it had been mutated.

Both callers must populate `TargetResources`: `commands/test` from `targetResources` in `kyverno-test.yaml`, and
`commands/apply` from `--target-resource` / `--target-resources`. Without it, target-only policies produce no result.

Regression tests: `TestRunTest_MutatingPolicyTargetMatchConstraintsDoNotMutateTrigger` (`commands/test`) and
`Test_Apply_MutatingPolicyTargetResources` (`commands/apply`).
