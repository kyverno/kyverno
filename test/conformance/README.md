# Kyverno Conformance Testing (`test/conformance`)

Kyverno uses [Chainsaw](https://kyverno.github.io/chainsaw/) to express end-to-end Kubernetes test scenarios declaratively, allowing test cases to be defined as version-controlled YAML rather than imperative scripts. These tests evaluate Kyverno in live Kubernetes clusters, validating policy engines, webhooks, background processing, cleanup policies, reports, and image verification.

## Related Documentation

- [Chainsaw Test Suite Reference](chainsaw/README.md)

## Directory Structure

Conformance tests are organized as scenario directories under `test/conformance/chainsaw/`:

```
test/conformance/
├── README.md                           # Conformance testing overview
└── chainsaw/                           # Chainsaw test scenarios and step templates
    ├── _step-templates/                # Reusable, parameterized step templates
    ├── validate/                       # Validation policy tests
    ├── mutate/                         # Mutation policy tests
    ├── generate/                       # Generation policy tests
    ├── verify-images/                  # Image verification tests
    ├── reports/                        # Policy report tests
    ├── cleanup/                        # Cleanup and TTL policy tests
    ├── webhooks/                       # Webhook configuration tests
    └── ...
```

## Category Overview

| Category | Description |
|---|---|
| `validate/`, `validating-policies/` | Validating policy rules and CEL validation policies |
| `mutate/`, `mutating-policies/` | Mutating policy rules and JSON patch / overlay operations |
| `generate/`, `generating-policies/` | Resource generation rules and background generation |
| `verify-images/`, `custom-sigstore/` | Container image signature and attestation verification |
| `reports/`, `openreports/` | PolicyReport and ClusterPolicyReport generation and updates |
| `cleanup/`, `deleting-policies/`, `ttl/` | Resource deletion and scheduled cleanup policies |
| `webhooks/`, `webhook-configurations/` | Dynamic webhook registration and configuration |
| `autogen/` | Pod controller rule auto-generation |
| `globalcontext/` | GlobalContextEntry cached data store evaluation |

## Related Implementation Packages

The table below outlines common relationships between conformance test categories and implementation packages in `pkg/`.

> **Note**: Test execution commonly involves multiple subsystems (e.g., admission webhooks, policy engine evaluation, and report aggregation). The mappings below highlight primary areas exercised by each category.

| Category | Primary Implementation Packages | Target Components |
|---|---|---|
| `webhooks/`, `webhook-configurations/` | `pkg/webhooks/`, `pkg/controllers/webhook/` | Admission Webhook Server, Webhook Controller |
| `mutate/`, `mutating-policies/` | `pkg/engine/mutate/`, `pkg/engine/` | Policy Engine (Mutation), Admission Controller |
| `validate/`, `validating-policies/`, `cel/` | `pkg/engine/validate/`, `pkg/cel/` | Policy Engine (Validation, CEL Engine) |
| `generate/`, `generating-policies/` | `pkg/controllers/generate/`, `pkg/background/` | Background Controller, Generate Controller |
| `cleanup/`, `deleting-policies/`, `ttl/` | `pkg/controllers/cleanup/` | Cleanup Controller, TTL Controller |
| `reports/`, `openreports/` | `pkg/controllers/report/`, `pkg/reports/` | Reports Controller, Report Aggregator |
| `autogen/` | `pkg/autogen/` | Policy Engine (Auto-gen Engine) |
| `verify-images/`, `custom-sigstore/` | `pkg/cosign/`, `pkg/notary/` | Image Verification Engine |
| `globalcontext/` | `pkg/globalcontext/` | Global Context Controller & Store |

## Test Execution Lifecycle

Conformance tests are organized as scenario directories containing `chainsaw-test.yaml`. During execution, Chainsaw discovers and runs these scenarios according to its configuration:

1. **Namespace Setup**: Depending on the test configuration, Chainsaw creates and manages namespaces for scenario execution.
2. **Template Resolution**: Steps referencing templates in `_step-templates/` resolve parameterized operations.
3. **Resource Application**: Policies and workload fixtures are applied to the cluster.
4. **Assertion Verification**: Assertions check resource state, status conditions, or expected admission rejections.
5. **Teardown**: Ephemeral namespaces are deleted upon scenario completion.

## Executing Tests

### Prerequisites
- A local Kubernetes cluster running (e.g., KinD via `make kind-create-cluster`).
- Kyverno deployed to the cluster (`make kind-deploy-kyverno` or `make kind-deploy-all`).
- `chainsaw` CLI installed.

### Execution Command
Run the conformance suite using `chainsaw`:

```bash
chainsaw test test/conformance/chainsaw
```

To execute a specific scenario directory (e.g., `webhooks/all-scale`):

```bash
chainsaw test test/conformance/chainsaw/webhooks/all-scale
```

## Adding a Test Scenario

1. Select or create the appropriate category folder under `test/conformance/chainsaw/`.
2. Create a subfolder using a clear, descriptive scenario name.
3. Common files found in many scenarios include:
   - `chainsaw-test.yaml`: Scenario step pipeline.
   - `policy.yaml`: Policy manifest under test.
   - Resource fixtures (e.g., `pod.yaml`, `resource.yaml`).
   - Assertion manifests (e.g., `assert.yaml`, `webhooks.yaml`).
4. Utilize step templates from `_step-templates/` for policy creation and readiness verification.
