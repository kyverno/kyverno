<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Generate*</small>

# Generate Configurations 

```generate``` feature can be applied to created namespaces to create new resources in them. This feature is useful when every namespace in a cluster must contain some basic required resources. The feature is available for policy rules in which the resource kind is Namespace.

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
          annotations: {}
          labels:
            policyname: "default"
````
In this example, when this policy is applied, any new namespace will receive a new NetworkPolicy resource based on the specified template that by default denies all inbound and outbound traffic.

---
<small>*Read Next >> [Testing Policies](/documentation/testing-policies.md)*</small>

