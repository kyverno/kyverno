<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Mutate Resources*</small>

# Mutating Resources

The ```mutate``` rule can be used to add, replace, or delete elements in matching resources. A mutate rule can be written as a JSON Patch or as an overlay. 

By using a ```patch``` in the [JSONPatch - RFC 6902](http://jsonpatch.com/) format, you can make precise changes to the resource being created. Using an ```overlay``` is convenient for describing the desired state of the resource.

Resource mutation occurs before validation, so the validation rules should not contradict the changes performed by the mutation section.

This policy sets the imagePullPolicy to Always if the image tag is latest:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: set-image-pull-policy
spec:
  rules:
    - name: set-image-pull-policy
      match:
        resources:
          kinds:
            - Pod
      mutate:
        overlay:
          spec:
            containers:
              # match images which end with :latest
              - (image): "*:latest"
                # set the imagePullPolicy to "Always"
                imagePullPolicy: "Always"
```

## JSONPatch - RFC 6902

A JSON Patch rule provides an alternate way to mutate resources.

[JSONPatch](http://jsonpatch.com/) supports the following operations (in the 'op' field):
* **add**
* **replace**
* **remove**

With Kyverno, the add and replace have the same behavior i.e. both operations will add or replace the target element.

This patch policy adds, or replaces, entries in a `ConfigMap` with the name `config-game` in any namespace.

````yaml
apiVersion : kyverno.io/v1
kind : ClusterPolicy
metadata :
  name : policy-generate-cm
spec :
  rules:
    - name: pCM1
      match:
        resources:
          name: "config-game"
          kinds :
          - ConfigMap
      mutate:
        patchesJson6902: |-
          - path: "/data/ship.properties"
            op: add
            value: |
              type=starship
              owner=utany.corp
          - path : "/data/newKey1"
            op : add
            value : newValue1
````

Here is the example of a patch that removes a label from the secret:

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: policy-remove-label
spec:
  rules:
    - name: "Remove unwanted label"
      match:
        resources:
          kinds:
            - Secret
      mutate:
        patchesJson6902: |-
          - path: "/metadata/labels/purpose"
            op: remove
````

This policy adds elements to list:

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: insert-container
spec:
  rules:
    - name: insert-container
      match:
        resources:
          kinds:
            - Pod
      mutate:
        patchesJson6902: |-
          - op: add
            path: /spec/containers/1
            value: {"name":"busyboxx","image":"busybox:latest"}
          - op: add
            path: /spec/containers/0/command
            value:
            - ls
````

Note, that if **remove** operation cannot be applied, then this **remove** operation will be skipped with no error.

## Strategic Merge Patch

A `patchStrategicMerge` patch is [stategic-merge](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md)-style patch. The `patchStrategicMerge` overlay resolves to a partial resource definition.

This policy sets the imagePullPolicy, adds command to container `nginx`:
````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: strategic-merge-patch
spec:
  rules:
    - name: set-image-pull-policy-add-command
      match:
        resources:
          kinds:
            - Pod
      mutate:
        patchStrategicMerge:
          metadata:
            labels:
              name: "{{request.object.metadata.name}}"
          spec:
            containers:
              - name: "nginx"
                image: "nginx:latest"
                imagePullPolicy: "Never"
                command:
                - ls
````

## Mutate Overlay

A mutation overlay describes the desired form of resource. The existing resource values are replaced with the values specified in the overlay. If a value is specified in the overlay but not present in the target resource, then it will be added to the resource. 

The overlay cannot be used to delete values in a resource: use **patches** for this purpose.

The following mutation overlay will add (or replace) the memory request and limit to 10Gi for every Pod with a label `memory: high`:

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: policy-change-memory-limit
spec:
  rules:
    - name: "Set hard memory limit to 2Gi"
      match:
        resources:
          kinds:
            - Pod
          selector:
            matchLabels:
              memory: high
      mutate:
        overlay:
          spec:
            containers:
            # the wildcard * will match all containers in the list
            - (name): "*"
              resources:
                requests:
                  memory: "10Gi"
                limits:
                  memory: "10Gi"

````

### Working with lists

Applying overlays to a list type is fairly straightforward: new items will be added to the list, unless they already exist. For example, the next overlay will add IP "192.168.10.172" to all addresses in all Endpoints:

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: policy-endpoints
spec:
  rules:
  - name: "Add IP to subsets"
    match:
      resources:
        kinds:
          - Endpoints
    mutate:
      overlay:
        subsets:
        - addresses:
          - ip: 192.168.42.172
````



### Conditional logic using anchors

An **anchor** field, marked by parentheses and an optional preceeding character, allows conditional processing for mutations. 

The mutate overlay rules support two types of anchors:

| Anchor      	     | Tag 	| Behavior                                    	       |
|--------------------|-----	|----------------------------------------------------- |
| Conditional 	     | ()  	| Use the tag and value as an "if" condition           |
| Add if not present | +() 	| Add the tag value, if the tag is not already present |


The **anchors** values support **wildcards**:
1. `*` - matches zero or more alphanumeric characters
2. `?` - matches a single alphanumeric character

#### Conditional anchor

A `conditional anchor` evaluates to `true` if the anchor tag exists and if the value matches the specified value. Processing stops if a tag does not exist or when the value does not match. Once processing stops, any child elements or any remaining siblings in a list, will not be processed.

 For example, this overlay will add or replace the value `6443` for the `port` field, for all ports with a name value that starts with "secure":

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: policy-set-port
spec:
  rules:
  - name: "Set port"
    match:
      resources:
        kinds :
          - Endpoints
    mutate:
      overlay:
        subsets:
        - ports:
          - (name): "secure*"
            port: 6443
````

If the anchor tag value is an object or array, the entire object or array must match. In other words, the entire object or array becomes part of the "if" clause. Nested `conditional anchor` tags are not supported.

### Add if not present anchor

A variation of an anchor, is to add a field value if it is not already defined. This is done by using the `add anchor` (short for `add if not present anchor`) with the notation `+(...)` for the tag.

An `add anchor` is processed as part of applying the mutation. Typically, every non-anchor tag-value is applied as part of the mutation. If the `add anchor` is set on a tag, the tag and value are only applied if they do not exist in the resource.

For example, this policy matches and mutates pods with `emptyDir` volume, to add the `safe-to-evict` annotation if it is not specified.

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata: 
  name: add-safe-to-evict
  annotations:
    pod-policies.kyverno.io/autogen-controllers: none
spec: 
  rules: 
  - name: "annotate-empty-dir"
    match: 
      resources: 
        kinds: 
        - Pod
    mutate: 
      overlay:
        metadata:
          annotations:
            +(cluster-autoscaler.kubernetes.io/safe-to-evict): true
        spec:          
          volumes: 
          - (emptyDir): {}
````

#### Anchor processing flow

The anchor processing behavior for mutate conditions is as follows:

1. First, all conditional anchors are processed. Processing stops when the first conditional anchor return a `false`. Mutation proceeds only of all conditional anchors return a `true`. Note that for `conditional anchor` tags with complex (object or array) values the entire value (child) object is treated as part of the condition, as explained above.

2. Next, all tag-values without anchors and all `add anchor` tags are processed to apply the mutation. 


## Additional Details

Additional details on mutation overlay behaviors are available on the wiki: [Mutation Overlay](https://github.com/nirmata/kyverno/wiki/Mutation-Overlay)

---
<small>*Read Next >> [Generate Resources](/documentation/writing-policies-generate.md)*</small>
