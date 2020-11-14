# Create Pod Anti-Affinity

In cases where you wish to run applications with multiple replicas, it may be required to ensure those Pods are separated from each other for availability purposes. While a `DaemonSet` resource would accomplish similar goals, your `Deployment` object may need fewer replicas than there are nodes. Pod anti-affinity rules ensures that Pods are separated from each other. Inversely, affinity rules ensure they are co-located.

This sample policy configures all Deployments with Pod anti-affinity rules with the `preferredDuringSchedulingIgnoredDuringExecution` option. It requires the topology key exists on all nodes with the key name of `kubernetes.io/hostname` and requires that that label `app` is applied to the Deployment.

In order to test the policy, you can use this sample Deployment manifest below.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: busybox
    distributed: required
  name: busybox
spec:
  replicas: 2
  selector:
    matchLabels:
      app: busybox
      distributed: required
  template:
    metadata:
      labels:
        app: busybox
        distributed: required
    spec:
      containers:
      - image: busybox:1.28
        name: busybox
        command: ["sleep", "9999"]
```

## More Information

* [Inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity)

## Policy YAML

[create_pod_antiaffinity.yaml](more/create_pod_antiaffinity.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: insert-podantiaffinity
spec:
  rules:
    - name: insert-podantiaffinity
      match:
        resources:
          kinds:
            - Deployment
      preconditions:
        # This precondition ensures that the label `app` is applied to Pods within the Deployment resource.
      - key: "{{request.object.metadata.labels.app}}"
        operator: NotEquals
        value: ""
      mutate:
        patchStrategicMerge:
          spec:
            template:
              spec:
                # Add the `affinity` key and others if not already specified in the Deployment manifest.
                +(affinity):
                  +(podAntiAffinity):
                    +(preferredDuringSchedulingIgnoredDuringExecution):
                      - weight: 1
                        podAffinityTerm:
                          topologyKey: "kubernetes.io/hostname"
                          labelSelector:
                            matchExpressions:
                            - key: app
                              operator: In
                              values:
                              - "{{request.object.metadata.labels.app}}"
```
