<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Mutate*</small>

# Mutate Configurations 

The ```mutate``` rule contains actions that should be applied to the resource before its creation. Mutation can be made using patches or overlay. Using ```patches``` in the JSONPatch format, you can make point changes to the created resource, and ```overlays``` are designed to bring the resource to the desired view according to a specific pattern.

Resource mutation occurs before validation, so the validation rules should not contradict the changes set in the mutation section.

## Patches

The patches are used to make direct changes in the created resource. In the next example the patch will be applied to all Deployments that contain a word "nirmata" in the name.

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-v1
spec :
  rules:
    - name: "Deployment of *nirmata* images"
      resource:
        kind: Deployment
        # Name is optional. By default validation policy is applicable to any resource of supported kind.
        # Name supports wildcards * and ?
        name: "*nirmata*"
      mutate:
        patches:
        # This patch adds sidecar container to every deployment that matches this policy
        - path: "/spec/template/spec/containers/0/"
          op: add
          value:
            - image: "nirmata.io/sidecar:latest"
              imagePullPolicy: "Always"
              ports:
              - containerPort: 443
````
There is one patch in the rule, it will add the new image to the "containers" list with specified parameters. Patch is described in [JSONPatch](http://jsonpatch.com/) format and support the operations ('op' field):
* **add**
* **replace**
* **remove**

Here is the example with of a patch which removes a label from the secret:
````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-remove-label
spec :
  rules:
    - name: "Remove unwanted label"
      resource:
        # Will be applied to all secrets, because name and selector are not specified
        kind: Secret
      mutate:
        patches:
        - path: "/metadata/labels/purpose"
          op: remove
````

Note, that if **remove** operation cannot be applied, then this **remove** operation will be skipped with no error.

## Overlay

The Mutation Overlay is the desired form of resource. The existing resource parameters are replaced with the parameters described in the overlay. If there are no such parameters in the target resource, they are copied to the resource from the overlay. The overlay is not used to delete the properties of a resource: use **patches** for this purpose.

The next overlay will add or change the hard limit for memory to 2 gigabytes in every ResourceQuota with label ```quota: low```:

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-change-memory-limit
spec :
  rules:
    - name: "Set hard memory limit to 2Gi"
      resource:
        # Will be applied to all secrets, because name and selector are not specified
        kind: ResourceQuota
        selector:
          matchLabels:
            quota: low
      mutate:
        overlay:
          spec:
            hard:
              limits.memory: 2Gi
````
The ```overlay``` keyword under ```mutate``` feature describes the desired form of ResourceQuota.

### Working with lists

The application of an overlay to the list without additional settings is pretty straightforward: the new items will be added to the list exсept of those that totally equal to existent items. For example, the next overlay will add IP "192.168.10.172" to all addresses in all Endpoints:

````yaml
apiVersion: policy.nirmata.io/v1alpha1
kind: Policy
metadata:
  name: policy-endpoints-
spec:
  rules:
  - resource:
      # Applied to all endpoints
      kind : Endpoints
    mutate:
      overlay:
        subsets:
        - addresses:
          - ip: 192.168.10.172
````

You can use overlays to merge objects inside lists using **anchor** items marked by parentheses. For example, this overlay will add/replace port to 6443 in all ports with name that start from the word "secure":
````yaml
apiVersion : policy.nirmata.io/v1alpha1
kind : Policy
metadata :
  name : policy-endpoints-should-be-more-secure
spec :
  rules:
  - resource:
      # Applied to all endpoints
      kind : Endpoints
    mutate:
      overlay:
        subsets:
        - ports:
          - (name): "secure*"
            port: 6443
````

The **anchors** marked in parentheses support **wildcards**:
1. `*` - matches zero or more alphanumeric characters
2. `?` - matches a single alphanumeric character

## Details

The behavior of overlays described more detailed in the project's wiki: [Mutation Overlay](https://github.com/nirmata/kyverno/wiki/Mutation-Overlay)

---
<small>*Read Next >> [Validate](/documentation/writing-policies-validate.md)*</small>
