## MutatingPolicy GenerateName Exclusion Conformance Test

### Overview
This conformance test verifies that during pre-name-generation mutating admission review—where incoming requests have an empty `metadata.name` (`name == ""`) and only `metadata.generateName` is present—workloads cannot bypass mutating policies by spoofing `excludeResourceRules.resourceNames`.

### Verification Steps
1. Deploy a `MutatingPolicy` that mutates `deployments` by injecting label `mutated: "true"`, with `excludeResourceRules: [resourceNames: [trusted-deployment]]`.
2. Submit a deployment using `metadata.generateName: "trusted-deployment-"`. Because mutating webhooks execute before Kubernetes generates the name suffix, `request.name` is empty. The policy engine must not exclude this workload, ensuring the mutation label is applied.
3. Submit a deployment with resolved `metadata.name: "trusted-deployment"`. The policy engine legitimately excludes this workload, verifying the mutation label is NOT applied.
