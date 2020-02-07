<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Generate Resources*</small>

# Generate Resources 

```generate``` is used to create additional resources when a resource is created. This is useful to create supporting resources, such as role bindings for a new namespace.

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
  * A ConfigMap cloned from default/config-template.
  * A Secret with values DB_USER and DB_PASSWORD, and label ```purpose: mongo```.


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
        podSelector:
          matchLabels: {}
          matchExpressions: []
        policyTypes: []
        metadata:
          labels:
            policyname: "default"
````

In this example new namespaces will receive a NetworkPolicy that default denies all inbound and outbound traffic.

---

<small>*Read Next >> [Variables](/documentation/writing-policies-variables.md)*</small>

