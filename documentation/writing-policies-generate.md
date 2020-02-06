<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Generate*</small>

# Generate Configurations 

```generate``` is used to create additional resources when a resource is created. This is useful to create supporting resources, such as role bindings for a new namespace.

## Example 1
- rule 
Creates a ConfigMap with name `default-config` for all 
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
        selector:
          matchLabels:
            LabelForSelector : "namespace2"
      generate:
        kind: ConfigMap # Kind of resource 
        name: default-config # Name of the new Resource
        namespace: "{{request.object.metadata.name}}" # Create in the namespace that triggers this rule
        clone:
          namespace: default
          name: config-template
    - name: "Generate Secret"
      match:
        resources:
          kinds: 
          - Namespace
        selector:
          matchLabels:
            LabelForSelector : "namespace2"
      generate:
        kind: Secret
        name: mongo-creds
        namespace: "{{request.object.metadata.name}}" # Create in the namespace that triggers this rule
        data:
          data:
            DB_USER: YWJyYWthZGFicmE=
            DB_PASSWORD: YXBwc3dvcmQ=
        metadata:
          labels:
            purpose: mongo
````

In this example, when this policy is applied, any new namespace that satisfies the label selector will receive 2 new resources after its creation:
  * ConfigMap copied from default/config-template.
  * Secret with values DB_USER and DB_PASSWORD, and label ```purpose: mongo```.


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
      namespace: "{{request.object.metadata.name}}" # Create in the namespace that triggers this rule
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

In this example, when the policy is applied, any new namespace will receive a NetworkPolicy based on the specified template that by default denies all inbound and outbound traffic.

---

<small>*Read Next >> [Variables](/documentation/writing-policies-variables.md)*</small>

