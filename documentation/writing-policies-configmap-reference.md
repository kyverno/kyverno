<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Configmap Lookup*</small>

# Configmap Reference in Kyverno Policy

There are many cases where the values that are passed into kyverno policies are dynamic, In such a cases the values are added/ modified inside policy itself.

The Configmap Reference allows the reference of configmap values inside kyverno policy rules as a JMESPATH, So for any required changes in the values of a policy, a modification has to be done on the referred configmap.

# Defining Rule Context

To refer Configmap inside any Rule provide the context inside each rule defining the list of configmaps which will be referenced in that Rule.

```
  rules:
    - name: add-sidecar-pod
      # added context to define the configmap information which will be referred 
      context:
      # unique name to identify configmap
      - name: mycmapRef
        configMap: 
          # configmap name - name of the configmap which will be referred
          name: mycmap
          # configmap namepsace - namespace of the configmap which will be referred
```

Referenced Configmap Definition

```
apiVersion: v1
data:
  env: production, sandbox, staging
kind: ConfigMap
metadata:
  name: mycmap
```

# Referring Value

The configmaps that are defined inside rule context can be referred using the unique name that is used to identify configmap inside context.

We can refer it's value using a JMESPATH

`{{<name>.<data>.<key>}}`

So for the above context we can refer it's value using

`{{mycmapRef.data.env}}`



<small>*Read Next >> [Testing Policies](/documentation/testing-policies.md)*</small>
