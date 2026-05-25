# Automated Governance & Compliance

Organizations need continuous assurance that Kubernetes resources comply with internal standards and external regulatory requirements.

Kyverno enables governance teams to enforce, audit, and report on compliance automatically—without manual review cycles or custom tooling.

## Challenges

Common governance and compliance challenges include:

- Manual audits that are slow and inconsistent
- Lack of visibility into policy drift across clusters
- Inconsistent enforcement of security standards
- Difficulty proving compliance during audits
- Policy exceptions that aren’t tracked or documented

## How Kyverno Helps

Kyverno capabilities that support automated governance:

| Capability     | Purpose                                            |
| -------------- | -------------------------------------------------- |
| Validate       | Enforce compliance rules at admission time         |
| Mutate         | Correct non-compliant configurations automatically |
| Generate       | Create required compliance resources               |
| Cleanup        | Remove non-compliant or deprecated resources       |
| Policy Reports | Provide audit-ready compliance visibility          |

## Example Use Cases

- Enforcing CIS Kubernetes benchmarks
- Auditing for privileged containers or insecure settings
- Blocking workload deployments without required labels
- Generating audit-friendly reports across namespaces
- Creating compliance guardrails for regulated environments

## Example Policy: Block Privileged Containers

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-privileged-containers
  annotations:
    policies.kyverno.io/title: Disallow Privileged Containers
    policies.kyverno.io/category: Governance & Compliance
    policies.kyverno.io/severity: high
    policies.kyverno.io/description: >
      Blocks workloads that attempt to run privileged containers.
spec:
  validationFailureAction: Enforce
  rules:
    - name: block-privileged
      match:
        any:
          - resources:
              kinds:
                - Pod
      validate:
        message: "Privileged containers are not allowed."
        pattern:
          spec:
            containers:
              - =(securityContext):
                   =(privileged): false
```

## Related Resources

- [Kyverno Documentation – Policy Reports](https://kyverno.io/docs/policy-reports/)
- [Kyverno Documentation – Audit Mode vs Enforce Mode](https://kyverno.io/docs/writing-policies/validate/#validation-failure-action)
- [Kyverno Policy Library – Compliance](https://kyverno.io/policies/)
- [Kyverno Blog](https://kyverno.io/blog/)
- [Kyverno Documentation – Cleanup Policies](https://kyverno.io/docs/writing-policies/cleanup/)