---
title: Generate Resources
description: 
---

# Generating Resources 

The ```generate``` rule can used to create additional resources when a new resource is created. This is useful to create supporting resources, such as new role bindings for a new namespace.

The `generate` rule supports `match` and `exclude` blocks, like other rules. Hence, the trigger for applying this rule can be the creation of any resource and its possible to match or exclude API requests based on subjects, roles, etc. 

The generate rule is triggered during a API CREATE operation. To keep resources synchronized across changes you can use the `synchronize` property. When `synchronize`  is set to `true`  the generated resource is kept in-sync with the source resource (which can be defined as part of the policy or may be an existing resource), and generated resources cannot be modified by users. If  `synchronize` is set to  `false` then users can update or delete the generated resource directly.

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
---