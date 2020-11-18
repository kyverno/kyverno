# Require `livenessProbe` and `readinessProbe` are different

Pod liveness and readiness probes are often used as a check to ensure either the health of an already running Pod or when one is ready to receive traffic. For a sample policy with more information and which contains a validation rule that both are present, see [require_probes.yaml](RequirePodProbes.md).

This sample checks to ensure that `livenessProbe` and `readinessProbe` are configured differently. When these two probes are configured but are set up the same way, race conditions can result as Kubernetes continues to kill and recreate a Pod never letting it enter a running state. This sample satisfies a common best practice in which these probes, if extant, not overlap and potentially cause this condition.

In this sample policy, a series of `deny` rules exist, one per container, to compare the `livenessProbe` map to the `readinessProbe`. If any container in a Pod potentially having multiple is found to have identical probes, its creation will be blocked. Note that in this sample policy the `validationFailureAction` is set to `enforce` due to the use of a `deny` rule rather than a `validate` rule. By using the annotation `pod-policies.kyverno.io/autogen-controllers`, it modifies the default behavior and ensures that only Pods originating from DaemonSet, Deployment, and StatefulSet objects are validated.

If you may potentially have more than four containers in a Pod against which this policy should operate, duplicate one of the rules found within and change the array member of the `containers` key in fields `key` and `value`. For example, to match against a potential fifth container, duplicate a rule and change `containers[3]` to `containers[4]`.

## More Information

* [Kyverno Deny Rules](https://kyverno.io/docs/writing-policies/validate/#deny-rules)
* [Kyverno Auto-Gen Rules for Pod Controllers](https://kyverno.io/docs/writing-policies/autogen/)
* [Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)

## Policy YAML

[ensure_probes_different.yaml](more/ensure_probes_different.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: validate-probes
  annotations:
    # Only applies to pods originating from DaemonSet, Deployment, or StatefulSet.
    pod-policies.kyverno.io/autogen-controllers: DaemonSet,Deployment,StatefulSet
spec:
  validationFailureAction: enforce
  background: false
  rules:
    # Checks the first container in a Pod.
    - name: validate-probes-c0
      match:
        resources:
          kinds:
            - Pod
      validate:
        message: "Liveness and readiness probes cannot be the same."
        # A `deny` rule is different in structure than a `validate` rule and inverts the check. It uses `conditions` written in JMESPath notation upon which to base its decisions.
        deny:
          conditions:
          # In this condition, it checks the entire map structure of the `readinessProbe` against that of the `livenessProbe`. If both are found to be equal, the Pod creation
          # request will be denied.
          - key: "{{ request.object.spec.containers[0].readinessProbe }}"
            operator: Equals
            value: "{{ request.object.spec.containers[0].livenessProbe }}"
    # Checks the second container in a Pod.
    - name: validate-probes-c1
      match:
        resources:
          kinds:
            - Pod
      validate:
        message: "Liveness and readiness probes cannot be the same."
        deny:
          conditions:
          - key: "{{ request.object.spec.containers[1].readinessProbe }}"
            operator: Equals
            value: "{{ request.object.spec.containers[1].livenessProbe }}"
    # Checks the third container in a Pod.
    - name: validate-probes-c2
      match:
        resources:
          kinds:
            - Pod
      validate:
        message: "Liveness and readiness probes cannot be the same."
        deny:
          conditions:
          - key: "{{ request.object.spec.containers[2].readinessProbe }}"
            operator: Equals
            value: "{{ request.object.spec.containers[2].livenessProbe }}"
    # Checks the fourth container in a Pod.
    - name: validate-probes-c3
      match:
        resources:
          kinds:
            - Pod
      validate:
        message: "Liveness and readiness probes cannot be the same."
        deny:
          conditions:
          - key: "{{ request.object.spec.containers[3].readinessProbe }}"
            operator: Equals
            value: "{{ request.object.spec.containers[3].livenessProbe }}"
```
