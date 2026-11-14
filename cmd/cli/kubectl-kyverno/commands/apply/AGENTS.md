# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

`kyverno apply` loads policies leniently and reports what it could not load instead of dropping it. The loader side
of this contract is described in `cmd/cli/kubectl-kyverno/policy/AGENTS.md`; this file covers the apply side.

## Loading policies

- `loadPolicies` calls `policy.LoadSkippingInvalidFiles` for every policy path: once per local, OCI or stdin path,
  and once per YAML file returned by `gitutils.ListYamls` for git sources. Keep `kyverno test`, `oci push` and
  `fix policy` on the strict `policy.Load`; a broken policy must keep failing those.
- `LoadSkippingInvalidFiles` only returns an error for fatal problems (a blocked legacy policy,
  `policy.ErrValidatorSetup`), so `loadPolicies` stops on any error it returns. Everything else arrives in
  `NonFatalErrors` and is recorded with `recordPolicyLoadErrors` into `ApplyCommandConfig.policyLoadErrors`,
  which `applyCommandHelper` resets on every run and hands back through `SkippedInvalidPolicies.loadErrors`.
  If the loader ever returns a new kind of error, apply fails loudly rather than skipping a policy silently.

## Printing what was not loaded

- `printPolicyLoadErrors` prints the "Policies Not Loaded" section, one `path: reason` line per entry. When a
  later step of apply fails, `Command` still prints it, on stderr, before returning that error.
- `Command` sends that section to stdout normally, but to stderr when stdout is meant for another program
  (`--policy-report`, `--generate-exceptions`, `--stdin`), so the report, the generated exceptions or the mutated
  resource on stdout stay parseable. Add any new machine readable output mode to that condition.
- Recorded load errors do not change the exit code and are not counted by `--warnings-as-errors`, which only
  covers deprecation warnings recorded by `recordPolicyWarnings`. The fatal ones (a blocked legacy policy,
  `policy.ErrValidatorSetup`) are returned as errors and fail the command.

This package implements the `kubectl-kyverno apply` CLI command, which applies Kyverno policies (validating, mutating, generating, image-validating, deleting, and admission policies) to Kubernetes manifests or resources fetched from a cluster.

## Architecture and Execution Flow

1. **Policy & Resource Loading (`command.go`, `load.go`)**:
   - Parses flags and configurations (`ApplyCommandConfig`).
   - Loads policies, exceptions, variables, and resources from local files or git sources.
   - When `--cluster` is specified, initializes a live dynamic/kube client (`dclient.Interface`).

2. **Policy Evaluation Passes (`command.go`)**:
   - `applyCommandHelper` coordinates evaluation across multiple policy families in sequence:
     - Standard admission policies (`applyPolicies`): validating, mutating, generating, and validating/mutating admission policies.
     - Image-validating policies (`applyImageValidatingPolicies`).
     - Deleting policies (`applyDeletingPolicies`).
     - Authorization policies (HTTP/Envoy).

3. **Shared Namespace Resolver Invariant**:
   - CEL engine evaluations (such as ValidatingPolicies, MutatingPolicies, and GeneratingPolicies) require a `NamespaceResolver` to evaluate `namespaceObject` and `namespaceSelector`.
   - `nsResolver := processor.GetNamespaceResolver(variables, dClient, c.Cluster)` is instantiated **once** in `applyCommandHelper` and passed into `applyPolicies`, `applyImageValidatingPolicies`, and `applyDeletingPolicies`.
   - **Critical Invariant**: All policy passes within a single `apply` invocation must share the same `NamespaceResolver` instance so that namespace lookups cached in memory are retained across all evaluation passes and do not trigger redundant cluster queries.
