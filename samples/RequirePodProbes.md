# Require `livenessProbe` and `readinessProbe`

For each pod, a livenessProbe is carried out by the kubelet to determine when to restart a container. A readinessProbe is used by services and deployments to determine if the pod is ready to recieve network traffic. 

Both liveness and readiness probes need to be configured to manage the pod lifecycle during restarts and upgrades.

## Policy YAML 

[require_probes.yaml](best_practices/require_probes.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-probes
spec:
  rules:
  - name: check-probes
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Liveness and readiness probes are required"
      pattern:
        spec:
          containers:
          - livenessProbe:
              periodSeconds: ">0"      
            readinessProbe:
              periodSeconds: ">0"

````

