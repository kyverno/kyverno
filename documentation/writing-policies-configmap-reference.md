<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Configmap Lookup*</small>

# Configmap Reference in Kyverno Policy

There are many cases where the values that are passed into kyverno policies are dynamic, In such a cases the values are added/ modified inside policy itself.

The Configmap Reference allows the reference of configmap values inside kyverno policy rules as a JMESPATH, So for any required changes in the values of a policy, a modification has to be done on the referred configmap.

# Defining Rule Context

To refer Configmap inside any Rule provide the context inside each rule defining the list of configmaps which will be referenced in that Rule.

````yaml
  rules:
    - name: example-configmap-lookup
      # added context to define the configmap information which will be referred 
      context:
      # unique name to identify configmap
      - name: dictionary
        configMap: 
          # configmap name - name of the configmap which will be referred
          name: mycmap
          # configmap namepsace - namespace of the configmap which will be referred
          namespace: test
````

Referenced Configmap Definition

````yaml
apiVersion: v1
data:
  env: production
kind: ConfigMap
metadata:
  name: mycmap
````

# Referring Value

The configmaps that are defined inside rule context can be referred using the unique name that is used to identify configmap inside context.

We can refer it's value using a JMESPATH `{{<name>.<data>.<key>}}`. 

For the above context we can refer it's value by `{{dictionary.data.env}}`, which will be substitued with `production` during policy application.

# Deal with Array of Values

The substitute variable can be an array of values. It allows the JSON format when defining it in the configMap.

For example, a list of allowed roles can be stored in configMap, and the kyverno policy can refer to this list to deny the disallowed request.

Here is the allowed roles in configMap:
````yaml
apiVersion: v1
data:
  allowed-roles: "[\"cluster-admin\", \"cluster-operator\", \"tenant-admin\"]"
kind: ConfigMap
metadata:
  name: roles-dictionary
  namespace: test
````


This is a rule to deny the Deployment operation, if the value of annotation `role` is not in the allowed list:
````yaml
spec:
  validationFailureAction: enforce
  rules:
  - name: validate-role-annotation
    context:
      - name: roles-dictionary
        configMap: 
          name: roles-dictionary
          namespace: test
    match:
      resources:
        kinds:
        - Deployment
    preconditions:
    - key: "{{ request.object.metadata.annotations.role }}"
      operator: NotEquals
      value: ""
    validate:
      message: "role {{ request.object.metadata.annotations.role }} is not in the allowed list {{ \"roles-dictionary\".data.\"allowed-roles\" }}"
      deny:
        conditions: 
        - key: "{{ request.object.metadata.annotations.role }}"
          operator: NotIn
          value:  "{{ \"roles-dictionary\".data.\"allowed-roles\" }}"
````



<small>*Read Next >> [Testing Policies](/documentation/testing-policies.md)*</small>
