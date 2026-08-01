# Conformance test guidance

`test/conformance/chainsaw` contains end-to-end tests for Kyverno running in a
real KinD cluster. These tests complement package-level unit tests by verifying
Kubernetes API interactions, admission behavior, controller reconciliation,
and installed manifests.

## Layout

- The first-level directories under `chainsaw/` group scenarios by behavior,
  such as `validate`, `mutate`, `generate`, `cleanup`, `reports`, `webhooks`,
  and `verify-images`.
- Individual scenarios normally contain `chainsaw-test.yaml` plus policy,
  resource, assertion, and optional README files.
- `_step-templates/` provides common operations such as creating a policy and
  waiting for it to become ready. Reuse these templates when they fit the test.
- Each suite can define `.chainsaw.yaml`; its timeout and cleanup settings are
  part of test reliability, not incidental boilerplate.

## Running and selecting tests

- CI partitions this suite in `.github/workflows/tests-conformance.yaml` and
  uses `.github/actions/tests/conformance/run/action.yaml` to create KinD,
  install Kyverno, apply repository CRDs, and run Chainsaw.
- A scenario is not generally self-contained on a developer workstation: it
  requires a prepared cluster and installed Kyverno. Follow the existing CI
  action or the root KinD targets instead of assuming `chainsaw test` alone is
  equivalent to CI.
- Select a focused suite only when its behavior clearly covers the change.
  Cross-cutting engine, webhook, API, or chart changes may require broader
  conformance coverage and should fall back to maintainer guidance when unsure.

## Test conventions

- Give each scenario an isolated, descriptive name and assert the externally
  observable Kubernetes result, not implementation details.
- Keep policy and resource manifests minimal enough to serve as a reproduction
  for the behavior under test.
- Use existing step templates and wait for policy readiness before asserting an
  admission or controller result.
- Do not weaken timeouts, skip scenarios, or quarantine a test to hide a
  failure without a linked issue and maintainer approval.

## Automation boundary

Automation may propose suites from an explicit source-to-test map and run
approved CI jobs. It must report its confidence and fall back to broader tests
or human review when no mapping is authoritative; it must not silently skip or
edit a conformance scenario.
