<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Generate*</small>

# Generate Configurations 

```generate``` is used to create default resources for a namespace. This feature is useful for managing resources that are required in each namespace.

## Example 1

````yaml
apiVersion: kyverno.io/v1alpha1
kind: Policy
metadata:
  name: basic-policy
spec:
  rules:
    - name: "Basic config generator for all namespaces"
      resource:
        kinds: 
        - Namespace
      selector:
        matchLabels:
          LabelForSelector : "namespace2"
      generate:
        kind: ConfigMap
        name: default-config
        clone:
          namespace: default
          name: config-template
    - name: "Basic config generator for all namespaces"
      resource:
        kinds: 
        - Namespace
      selector:
        matchLabels:
          LabelForSelector : "namespace2"
      generate:
        kind: Secret
        name: mongo-creds
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
apiVersion: kyverno.io/v1alpha1
kind: Policy
metadata:
  name: "default"
spec:
  rules:
  - name: "deny-all-traffic"
    resource: 
      kinds:
       - Namespace
      name: "*"
    generate: 
      kind: NetworkPolicy
      name: deny-all-traffic
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
<small>*Read Next >> [Testing Policies](/documentation/testing-policies.md)*</small>

