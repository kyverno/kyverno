# Engine Agent Guide

This file provides guidance for coding agents working in `pkg/engine/`.
The repository-level `AGENTS.md` also applies.

## Scope

`pkg/engine` contains the core policy evaluation logic used by Kyverno.

The main engine implementation is created by `NewEngine` in `engine.go` and
implements the interfaces defined in `pkg/engine/api`.

Primary evaluation entry points include:

- `Validate` — evaluates validation and verify-image checks.
- `Mutate` — evaluates mutation rules and returns the patched resource.
- `Generate` — evaluates generate rules.
- `VerifyAndPatchImages` — evaluates image verification rules that may mutate
  the resource.
- `ApplyBackgroundChecks` — evaluates generate and mutate-existing rules for
  background processing.

## Evaluation Flow

Validation, mutation, and image verification generally follow this flow:

1. Match the policy against the policy context.
2. Expand applicable rules with `autogen.Default.ComputeRules`.
3. Select the handler for the rule type.
4. Call `invokeRuleHandler`.
5. Match the rule against the resource.
6. Load rule context.
7. Evaluate preconditions.
8. Substitute rule properties.
9. Resolve policy exceptions.
10. Process the selected handler.
11. Collect rule responses and execution statistics.

`invokeRuleHandler` is shared infrastructure for this flow. Changes to it can
affect multiple rule types.

Generate and background processing use `filterRule` to determine whether
generate or mutate-existing rules apply. Do not assume they follow the same
handler path as validation and mutation.

## Important Packages

- `api/` — engine interfaces, policy context, responses, rule status/type, and
  execution statistics.
- `context/` — JSON evaluation context and context loading infrastructure.
- `handlers/validation/` — handlers for validation-related rules.
- `handlers/mutation/` — handlers for mutation and image mutation.
- `internal/` — shared internal evaluation helpers.
- `jmespath/` — JMESPath integration.
- `mutate/` — mutation and patch processing.
- `pattern/` — pattern matching.
- `policycontext/` — policy context implementation and helpers.
- `utils/` — engine resource/rule matching and related utilities.
- `validate/` — validation helpers.
- `variables/` — variable substitution and condition evaluation.

## Context Safety

The policy JSON context is mutable evaluation state.

Evaluation paths use `Checkpoint` and `Restore` to isolate context changes.
`invokeRuleHandler` also adds a patched resource back to the JSON context so
subsequent evaluation sees the current resource.

When changing evaluation flow:

- preserve checkpoint/restore boundaries;
- do not leak rule-specific context into later rule evaluation;
- verify whether subsequent rules must observe a patched resource;
- test old-resource behavior for update/delete-related paths when relevant.

## Rule Processing

Rules may be expanded by `autogen.Default.ComputeRules`. A change that appears
to affect one policy rule can therefore affect generated rules as well.

Respect `spec.applyRules`. Paths using `ApplyOne` may stop processing after an
applicable rule has produced a result.

Policy exceptions are part of evaluation and must not be bypassed when adding
new rule-processing paths.

## Making Changes

Prefer the smallest package that owns the behavior.

When changing shared code such as `engine.go`, `internal/`, `api/`, context
handling, matching, or variable evaluation, inspect callers across the engine
before changing semantics.

Avoid mixing behavior changes with unrelated refactoring.

Changes to engine API types or interfaces can have callers outside
`pkg/engine`; search the repository before changing them.

## Testing

Run tests for the package you changed first:

```sh
go test ./pkg/engine/<package>
```

For changes to shared engine behavior, run the full engine package tree:

```sh
go test ./pkg/engine/...
```

For changes to top-level evaluation behavior, inspect and update the relevant
tests in `pkg/engine`, including validation, mutation, image verification,
background processing, and exceptions as applicable.

Before submitting a code change, also follow the repository-level formatting,
linting, code-generation, and testing requirements in `/AGENTS.md`.

## Generated Files

Do not manually edit generated files.

The repository-level `AGENTS.md` documents generated-code locations and the
required code-generation commands. Changes to API definitions may require
regeneration outside this directory.