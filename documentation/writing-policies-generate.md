<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Generate*</small>

# Generate Configurations 

```generatate``` feature can be applied to created namespaces to create new resources in them. This feature is useful when every namespace in a cluster must contain some basic required resources. The feature is available for policy rules in which the resource kind is Namespace.

## Example

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : basic-policy
spec :
  rules:
    - name: "Basic confog generator for all namespaces"
      resource:
        kind: Namespace
      generate:
        # For now the next kinds are supported:
        #  ConfigMap
        #  Secret
      - kind: ConfigMap
        name: default-config
        copyFrom:
          namespace: default
          name: config-template
        data:
          DB_ENDPOINT: mongodb://mydomain.ua/db_stage:27017
        labels:
          purpose: mongo
      - kind: Secret
        name: mongo-creds
        data:
          DB_USER: YWJyYWthZGFicmE=
          DB_PASSWORD: YXBwc3dvcmQ=
        labels:
          purpose: mongo
````

In this example, when this policy is applied, any new namespace will receive 2 new resources after its creation:
* ConfigMap copied from default/config-template with added value DB_ENDPOINT.
* Secret with values DB_USER and DB_PASSWORD.

Both resources will contain a label ```purpose: mongo```

---
<small>*Read Next >> [Testing Policies](/documentation/testing-policies.md)*</small>

