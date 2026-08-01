# AGENTS.md — test/conformance

This directory contains end-to-end conformance tests using [chainsaw](https://kyverno.github.io/chainsaw/latest/quick-start/).

## Structure

```
test/conformance/chainsaw/
├── 000-base/                    # Base resources and setup
├── 001-validating-policy/       # ValidatingPolicy tests
├── 002-mutating-policy/         # MutatingPolicy tests
├── 003-generating-policy/       # GeneratingPolicy tests
├── 004-deleting-policy/         # DeletingPolicy tests
├── 005-image-verify/            # Image verification tests
├── 006-cluster-policy/          # ClusterPolicy (legacy) tests
├── 007-policy-exception/        # PolicyException tests
├── 008-background/              # Background controller tests
├── 009-cleanup/                 # CleanupPolicy tests
├── 010-reports/                 # PolicyReport tests
├── 011-admission-policy/        # AdmissionPolicy tests
├── 012-cel/                     # CEL expression tests
├── 013-helm/                    # Helm chart tests
├── 014-verify/                  # Verify tests
├── 015-clean/                   # Cleanup tests
├── 016-multi/                   # Multi-policy tests
├── 017-userinfo/                # User info tests
├── 018-context/                 # Context tests
├── 019-userinfo/                # User info tests
├── 020-kubectl-kyverno/         # CLI tests
├── 021-cluster-cleanup/         # ClusterCleanup tests
├── 022-generate/                # Generate tests
├── 023-mutate/                  # Mutate tests
├── 024-validate/                # Validate tests
├── 025-conformance/             # Conformance suite
├── 026-verify-images/           # Image verification tests
├── 027-oc/                      # OpenShift tests
├── 028-dry-run/                 # Dry run tests
├── 029-sigstore/                # Sigstore tests
├── 030-api/                     # API tests
├── 031-background-scan/         # Background scan tests
├── chainsaw.yaml                # Chainsaw configuration
└── README.md                    # Running instructions
```

## Running Tests

```bash
# Run all conformance tests
make kind-deploy-all && make test-conformance

# Run specific test suite
chainsaw test --test-dir test/conformance/chainsaw/001-validating-policy

# Run with custom kubeconfig
KUBECONFIG=~/.kube/config chainsaw test --test-dir test/conformance/chainsaw/
```

## Test Format

Each test is a YAML file with:
- `apiVersion: chainsaw.kyverno.io/v1alpha1`
- `kind: Test`
- `metadata.name`: test name
- `spec.steps`: sequence of operations (apply, assert, delete, script)

## Conventions

- **Test names**: descriptive, kebab-case
- **Resources**: use `kyverno.io` namespace for Kyverno resources
- **Assertions**: use `check` with jq expressions
- **Cleanup**: always include cleanup steps
- **Parallel**: tests in different directories can run in parallel

## For AI Agents

- **Entry point**: `chainsaw.yaml` defines global config
- **Adding tests**: Create new directory under `chainsaw/` with test files
- **Dependencies**: Use `spec.steps[].dependsOn` for ordering
- **Variables**: Use `{{ .Values.xxx }}` for parameterization
- **Debugging**: `chainsaw test --verbose --test-dir <path>`