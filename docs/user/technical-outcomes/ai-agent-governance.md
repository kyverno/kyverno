# AI & Agent Governance

As AI workloads and agent-driven infrastructure become more common in Kubernetes environments,
organizations face new challenges in maintaining security, compliance, and operational consistency.
AI-specific workloads often introduce non-standard resource patterns, privileged access requirements,
and external dependencies that traditional policies were not designed to address.

Kyverno enables platform and security teams to define and enforce policy controls specifically
tailored to AI workloads and autonomous agent pipelines, without requiring changes to application
code or CI/CD workflows.

## Challenges

Common governance challenges for AI and agent-driven workloads include:

* Uncontrolled resource consumption (GPUs, memory, CPU) by AI workloads
* Privileged or overly permissive access for agent runners
* Missing or incorrect labels and metadata on AI-specific resources
* Unverified model container images and third-party base images
* Agents generating or mutating Kubernetes resources without policy guardrails
* Lack of audit trails for dynamically provisioned AI infrastructure
* Drift between what agents deploy and what governance requires

## How Kyverno Helps

Kyverno policy capabilities that support AI and agent governance include:

| Capability    | Purpose                                                              |
| ------------- | -------------------------------------------------------------------- |
| Validate      | Enforce resource limits, required labels, and access restrictions on AI pods |
| Mutate        | Automatically inject resource quotas, tolerations, and node selectors for GPU workloads |
| Generate      | Provision required supporting resources (NetworkPolicies, ResourceQuotas) when AI namespaces are created |
| Verify Images | Validate that model-serving containers and agent base images are signed and from trusted sources |
| Cleanup       | Remove stale agent-provisioned resources that no longer meet policy requirements |

## Example Use Cases

* Requiring GPU resource limits on all pods requesting `nvidia.com/gpu`
* Blocking agent-created pods that lack required `ai-workload` or `team` labels
* Automatically generating a `ResourceQuota` when a new AI project namespace is created
* Verifying cosign signatures on model-serving container images before admission
* Generating `NetworkPolicy` resources to isolate agent workloads from production services
* Auditing and cleaning up temporary resources created by autonomous pipelines

## Example Policy: Require Resource Limits on GPU Workloads

The following policy validates that any pod requesting GPU resources defines explicit CPU and memory limits:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-limits-for-gpu-workloads
  annotations:
    policies.kyverno.io/title: Require Resource Limits for GPU Workloads
    policies.kyverno.io/category: AI & Agent Governance
    policies.kyverno.io/severity: high
    policies.kyverno.io/description: >-
      Ensures that all pods requesting GPU resources define explicit CPU and memory limits
      to prevent resource starvation across the cluster.
spec:
  validationFailureAction: Enforce
  rules:
    - name: check-gpu-resource-limits
      match:
        any:
          - resources:
              kinds:
                - Pod
      preconditions:
        any:
          - key: "{{ request.object.spec.containers[].resources.requests.\"nvidia.com/gpu\" | length(@) }}"
            operator: GreaterThan
            value: 0
          - key: "{{ request.object.spec.containers[].resources.limits.\"nvidia.com/gpu\" | length(@) }}"
            operator: GreaterThan
            value: 0
      validate:
        message: "Pods requesting GPU resources must define CPU and memory limits."
        foreach:
          - list: "request.object.spec.containers"
            deny:
              conditions:
                any:
                  - key: "{{ element.resources.limits.cpu | length(@) }}"
                    operator: Equals
                    value: 0
                  - key: "{{ element.resources.limits.memory | length(@) }}"
                    operator: Equals
                    value: 0
```

## Supporting Resources

* [Kyverno Policy Library](https://kyverno.io/policies/) – Browse community-contributed policies including resource governance examples
* [Verify Images Documentation](https://kyverno.io/docs/writing-policies/verify-images/) – How to enforce image signing for AI model containers
* [Generate Rules](https://kyverno.io/docs/writing-policies/generate/) – Automatically create supporting resources for AI namespaces
* [Kyverno Blog](https://kyverno.io/blog/) – Community articles on supply chain and workload governance

## Future Enhancements

Future iterations of this section may include:

* Reference architecture for governing LLM-serving infrastructure
* Policies for AI agent frameworks (LangChain, AutoGPT, CrewAI-based deployments)
* Integration guidance for AI-specific admission scenarios
* Community talks and case studies from teams running AI workloads at scale
