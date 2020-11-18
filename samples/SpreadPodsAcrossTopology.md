# Spread pods across topology

When having a Kubernetes cluster that spans multiple availability zones, it is often desired to spread your Pods out among them in a way which controls where they land. This can be advantageous in ensuring that, should one of those zones fail, your application continues to run in a more predictable way and with less potential loss.

This sample policy configures all Deployments having the label of `required: true` to be spread amongst hosts which are labeled with the key name of `zone`. It does this only to Deployments which do not already have the field `topologySpreadConstraints` set.

**NOTE:** When deploying this policy to a Kubernetes cluster less than version 1.19, some feature gate flags will need to be enabled. Please see the [More Information](#more-information) section below.

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

* [Pod Topology Spread Constraints](https://kubernetes.io/docs/concepts/workloads/pods/pod-topology-spread-constraints/)

## Policy YAML

[spread_pods_across_topology.yaml](more/spread_pods_across_topology.yaml)

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: spread-pods
spec:
  rules:
    - name: spread-pods-across-nodes
      # Matches any Deployment with the label `distributed=required`
      match:
        resources:
          kinds:
          - Deployment
          selector:
            matchLabels:
              distributed: required
      # Mutates the incoming Deployment.
      mutate:
        patchStrategicMerge:
          spec:
            template:
              spec:
                # Adds the topologySpreadConstraints field if non-existent in the request.
                +(topologySpreadConstraints):
                - maxSkew: 1
                  topologyKey: zone
                  whenUnsatisfiable: DoNotSchedule
                  labelSelector:
                    matchLabels:
                      distributed: required
```
