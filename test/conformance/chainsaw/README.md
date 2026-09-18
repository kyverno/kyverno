# Chainsaw Test Suite (`test/conformance/chainsaw`)

This directory contains declarative end-to-end conformance scenarios executed via [Chainsaw](https://kyverno.github.io/chainsaw/).

## Related Documentation

- [Conformance Overview](../README.md)

## Directory Structure

```
test/conformance/chainsaw/
├── .chainsaw.yaml                      # Root configuration settings
├── _step-templates/                    # Shared step templates
└── <category>/<scenario-name>/         # Individual test scenarios
    ├── chainsaw-test.yaml              # Step pipeline definition
    └── ...
```

## Shared Step Templates (`_step-templates`)

Common test operations are defined as reusable `StepTemplate` resources in `_step-templates/`:

- `create-policy.yaml`: Applies a policy manifest file passed via variable binding.
- `cluster-policy-ready.yaml`: Waits for a `ClusterPolicy` to report `Ready: true`.
- `validating-policy-ready.yaml`: Waits for a `ValidatingPolicy` to report `Ready: true`.
- `mutating-policy-ready.yaml`: Waits for a `MutatingPolicy` to report `Ready: true`.

Example usage in `chainsaw-test.yaml` (from `test/conformance/chainsaw/webhooks/all-scale/chainsaw-test.yaml`):

```yaml
steps:
- name: create policy
  use:
    template: ../../_step-templates/create-policy.yaml
    with:
      bindings:
      - name: file
        value: policy.yaml
- name: wait policy ready
  use:
    template: ../../_step-templates/cluster-policy-ready.yaml
    with:
      bindings:
      - name: name
        value: require-labels
```

## Configuration (`.chainsaw.yaml`)

Configuration files (such as `test/conformance/chainsaw/webhooks/.chainsaw.yaml`) define scenario execution controls:

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha2
kind: Configuration
metadata:
  name: configuration
spec:
  discovery:
    fullName: true
  execution:
    failFast: true
  namespace:
    fastDelete: true
  timeouts:
    apply: 10s
```

## Debugging Test Failures

1. Inspect output diffs emitted by Chainsaw during step execution.
2. Run single scenario directories for faster iteration:
   ```bash
   chainsaw test test/conformance/chainsaw/<category>/<scenario>
   ```
3. Preserve test namespaces on failure using `--keep-resources`:
   ```bash
   chainsaw test test/conformance/chainsaw/<category>/<scenario> --keep-resources
   ```
4. Check Kyverno controller logs for evaluation details:
   ```bash
   kubectl logs -n kyverno -l app.kubernetes.io/instance=kyverno --tail=200
   ```
