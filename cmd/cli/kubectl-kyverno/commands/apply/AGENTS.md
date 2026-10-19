# AGENTS.md — cmd/cli/kubectl-kyverno/commands/apply

This document covers implementation details and conventions for `kubectl-kyverno apply`.

## Policy Validation and Warning Output Conventions

- **Pre-execution Policy Validation (`c.validatePolicies`)**:
  - `applyPolicies` invokes `c.validatePolicies` prior to applying policies to resources.
  - Captures both warnings and errors returned by `policyvalidation.Validate(pol, nil, nil, true, sa, sa)`.
  - Service account username computation `sa` is performed once outside the policy loop.
  - **Warning Output**: Warnings (such as field deprecation notices) are emitted directly to the command output writer `out` as `Warning: policy [namespace/]<name>: <message>` (without leading space indentation, consistent with apply command warning conventions).
  - **Duplicate Warning Suppression**: When a policy fails validation (`err != nil`), warnings matching `err.Error()` are suppressed to prevent fatal errors from appearing as warnings.
  - **Deprecation Warning Accumulation**: Non-duplicate warnings are appended to `c.deprecationWarnings` for evaluation against `--warnings-as-errors`.
  - **Failure Handling**:
    - Policies with validation errors increment `rc.IncrementError(1)`.
    - Errors prefixed with `variable 'element.name'` are categorized under `skipInvalidPolicies.invalid`, while other validation errors are recorded under `skipInvalidPolicies.skipped`.

## Warnings as Errors Checkpoints

`c.failOnDeprecationWarnings()` evaluates whether `--warnings-as-errors` is enabled and any deprecation warnings were recorded. It is checked at three checkpoints in `applyCommandHelper`:
1. After loading policies via `c.loadPolicies`
2. After loading policy exceptions
3. After policy validation in `c.applyPolicies`
