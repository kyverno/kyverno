<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Generate Resources*</small>

# Generating Resources 

The ```generate``` rule can used to create additional resources when a new resource is created. This is useful to create supporting resources, such as new role bindings for a new namespace.

The `generate` rule supports `match` and `exclude` blocks, like other rules. Hence, the trigger for applying this rule can be the creation of any resource and its possible to match or exclude API requests based on subjects, roles, etc. 

The generate rule triggers during a API CREATE operation and does not support [background processing](/documentation/writing-policies-background.md). To keep resources synchronized across changes you can use `synchronize : true`.

This policy sets the Zookeeper and Kafka connection strings for all namespaces.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "zk-kafka-address"
spec:
  rules:
    - name: "zk-kafka-address"
      match:
        resources:
          kinds:
            - Namespace
      generate:
        synchronize: true
        kind: ConfigMap
        name: zk-kafka-address
        # generate the resource in the new namespace
        namespace: "{{request.object.metadata.name}}"
        data:
          kind: ConfigMap
          data:
            ZK_ADDRESS: "192.168.10.10:2181,192.168.10.11:2181,192.168.10.12:2181"
            KAFKA_ADDRESS: "192.168.10.13:9092,192.168.10.14:9092,192.168.10.15:9092"
```

## Example 1

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: basic-policy
spec:
  rules:
    - name: "Generate ConfigMap"
      match:
        resources:
          kinds: 
          - Namespace
      generate:
        kind: ConfigMap # Kind of resource 
        name: default-config # Name of the new Resource
        namespace: "{{request.object.metadata.name}}" # namespace that triggers this rule
        synchronize : true
        clone:
          namespace: default
          name: config-template
    - name: "Generate Secret (insecure)"
      match:
        resources:
          kinds: 
          - Namespace
      generate:
        kind: Secret
        name: mongo-creds
        namespace: "{{request.object.metadata.name}}" # namespace that triggers this rule
        data:
          data:
            DB_USER: YWJyYWthZGFicmE=
            DB_PASSWORD: YXBwc3dvcmQ=
        metadata:
          labels:
            purpose: mongo
````

In this example new namespaces will receive 2 new resources after its creation:
  * A `ConfigMap` cloned from `default/config-template`.
  * A `Secret` with values `DB_USER` and `DB_PASSWORD`, and label `purpose: mongo`.


## Example 2
````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: "default"
spec:
  rules:
  - name: "deny-all-traffic"
    match:
      resources: 
        kinds:
        - Namespace
        name: "*"
    generate: 
      kind: NetworkPolicy
      name: deny-all-traffic
      namespace: "{{request.object.metadata.name}}" # namespace that triggers this rule
      data:
        spec:
          # select all pods in the namespace
          podSelector: {}
          policyTypes: 
          - Ingress
        metadata:
          labels:
            policyname: "default"
````

In this example new namespaces will receive a `NetworkPolicy` that by default denies all inbound and outbound traffic.

---

<small>*Read Next >> [Variables](/documentation/writing-policies-variables.md)*</small>

