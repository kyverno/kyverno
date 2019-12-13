# Require `livenessProbe` and `readinessProbe`

Liveness and readiness probes need to be configured to correctly manage a pods lifecycle during deployments, restarts, and upgrades.

For each pod, a periodic `livenessProbe` is performed by the kubelet to determine if the pod's containers are running or need to be restarted. A `readinessProbe` is used by services and deployments to determine if the pod is ready to receive network traffic.

## Policy YAML 

[require_probes.yaml](best_practices/require_probes.yaml)

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-pod-probes
spec:
  rules:
  - name: validate-livenessProbe-readinessProbe
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

