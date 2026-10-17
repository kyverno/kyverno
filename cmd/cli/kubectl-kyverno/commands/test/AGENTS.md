# AGENTS.md — cmd/cli/kubectl-kyverno/commands/test

This package implements the `kubectl-kyverno test` CLI command, which executes declarative test suites defined by `kyverno-test.yaml` manifests to validate Kyverno policies against target resources and configurations.

## Architecture and Execution Flow

1. **Test Loading (`test.go`, `load.go`)**:
   - Parses `kyverno-test.yaml` declarations including policies, resources, variables, and expected results.
   - Discovers and loads test files from directories, Git repositories, or standard input.

2. **Policy Execution (`test.go`)**:
   - Policies are applied against input resources to generate engine responses.
   - Responses are partitioned into:
     - `responses.Trigger`: Normal admission evaluation results per resource.
     - `responses.TriggerByOperation`: Dedicated evaluation runs partitioned by admission operation (`CREATE`, `UPDATE`, `DELETE`).
     - `responses.Target`: Results for target resources affected by mutate/generate existing policies.
     - `responses.SkippedPolicies`: Policies skipped during validation or precondition checks.
     - `responses.DeletingPolicies`: Policies evaluated in delete contexts.

3. **Result Comparison and Rendering (`output.go`, `command.go`)**:
   - `printTestResult` iterates over expected test results and compares them against engine responses.
   - Results are converted into tabular rows (`table.Table`) and formatted as table, JSON, YAML, or JUnit output (`printOutputFormats`).

## Target Response and Output Contracts

- **Target Resource Key Convention**:
  Target resource keys in `responses.Target` are serialized as `apiVersion,kind,namespace,name` (or `group/version,kind,namespace,name`). Parsing uses `strings.SplitN(resource, ",", 4)` so resource names containing commas do not shift field indices.
- **`extractPatchedTargetFromEngineResponse` Contract**:
  - Scans `response.PolicyResponse.Rules` for a rule with a non-nil `rule.PatchedTarget()`. Note that `rule.PatchedTarget()` is only populated by the engine when mutation succeeded (`RuleStatusPass`); rules that skipped, errored, or failed return a `nil` patched target.
  - Matches against `apiVersion`, `kind`, `resourceName`, and `resourceNamespace`.
  - **Namespace scoping**: When `resourceNamespace` is empty, it adopts the namespace of the candidate target (`r.GetNamespace()`) on a per-rule basis using a local variable. It **must never mutate** the input parameter across loop iterations.
  - Returns `(*unstructured.Unstructured, *engineapi.RuleResponse)` or `(nil, nil)` if no rule produced a matching patched target.
- **Nil Safety & Preserving Rule Outcomes**:
  - Because `responses.Target[resource]` may contain engine responses that did not produce a patched target (e.g. skipped, errored, or failed rules, or empty responses), callers **must verify** `r != nil && rule != nil` before dereferencing or invoking `checkResult(...)`.
  - Ruleless policies (e.g. `MutatingPolicy`) are exempted from rule-name matching so `checkResult` compares patched resources even if `test.Rule` is specified.
  - When no patched target was produced (`r == nil || rule == nil`), matching rule responses in `response.PolicyResponse.Rules` are evaluated via `checkRuleResultOnly(...)` to preserve the engine's actual rule outcome (`Skip`, `Error`, `Fail`) against expected results instead of dropping into the fallback.
- **Target Policy Scoping & Competing Responses**:
  - `responses.Target[resource]` may contain engine responses from multiple policies (or cluster policies and namespaced policies with the same name across different namespaces). Target evaluation filters by `policyName` and requires `response.Policy().GetNamespace()` to strictly match `policyNamespace` (including for unqualified cluster policies where namespace is empty), and skips unattributed responses where `response.Policy() == nil`. This ensures competing cluster or namespaced policy responses under the same resource key are cleanly excluded.
- **Target Evaluation Provenance**:
  - `responseTargetsResource` checks for target-evaluation provenance (`kyverno.io/target` property on rule responses) so ordinary admission trigger responses cannot satisfy target test assertions when trigger and target resources share the same object identity.
- **Fallback for Unmatched Target Results**:
  - When a target test result matches no engine response rows (`len(rows) == 0`):
    - If the policy is recorded in `responses.SkippedPolicies`, the result is classified as `Skip` (reason `Invalid Policy`) and increments `rc.Skip`.
    - Otherwise (ordinary missing target or no matching rule), it is classified as `Fail` (reason `Not found`) and increments `rc.Fail`.
