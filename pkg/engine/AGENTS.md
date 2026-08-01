# Policy engine guidance

`pkg/engine` evaluates Kyverno policies against a policy context. It is shared by
admission handling, background processing, and CLI-facing flows, so a change
here can affect more than the component whose failure first exposed it.

## Entry points

- `engine.go` constructs the engine and exposes validation, mutation,
  generation, image-verification, and background-check operations.
- `api/` defines the engine-facing interfaces, policy context, responses, and
  supporting data types.
- `handlers/` implements rule handlers; `context/` loads policy context;
  `mutate/`, `validate/`, and `variables/` contain the corresponding evaluation
  helpers.
- `internal/` contains implementation details used by the engine. Do not make
  its APIs dependencies of packages outside `pkg/engine` without discussing the
  boundary first.

## Testing

- Add or update focused Go tests alongside the changed package. Run them with
  `go test ./pkg/engine/...` while iterating.
- Run `make test-unit` before requesting review for changes with broader engine
  impact.
- Policy-visible behavior may also need a CLI fixture or a Chainsaw conformance
  test. Choose those based on the affected execution path, not only the changed
  directory.

## Change safety

- Preserve the distinction between admission-time evaluation and background
  evaluation. They can share engine code but have different inputs and effects.
- Treat mutation patches, variable substitution, image verification, and policy
  exceptions as high-impact behavior. Include a minimal policy/resource proof
  in the PR when changing them.
- API type changes are made under `api/`, not here, and require the repository
  code-generation workflow described in the root `AGENTS.md`.

## Automation boundary

Automation may identify relevant tests and draft a summary, but it must not
claim that a focused engine unit-test run fully covers an admission, CLI, or
background behavior change. Escalate when the affected execution path is
unclear.
