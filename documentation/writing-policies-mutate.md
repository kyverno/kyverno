<small>*[documentation](/README.md#documentation) / [Writing Policies](/documentation/writing-policies.md) / Mutate*</small>

# Mutate Configurations 

The ```mutate``` rule contains actions that will be applied to matching resource before their creation. A mutate rule can be written as a JSON Patch or as an overlay. By using a ```patch``` in the (JSONPatch - RFC 6902)[http://jsonpatch.com/] format, you can make precise changes to the resource being created. Using an ```overlay``` is convenient for describing the desired state of the resource.

Resource mutation occurs before validation, so the validation rules should not contradict the changes performed by the mutation section.

## Patches

This patch adds an init container to all deployments.

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-v1
spec :
  rules:
    - name: "add-init-secrets"
      resource:
        kinds:
        - Deployment
      mutate:
        patches:
        - path: "/spec/template/spec/initContainers/0/"
          op: add
          value:
            - image: "nirmata.io/kube-vault-client:v2"
              name: "init-secrets"

````
[JSONPatch](http://jsonpatch.com/) supports the following operations (in the 'op' field):
* **add**
* **replace**
* **remove**

With Kyverno, the add and replace have the same behavior i.e. both operations will add or replace the target element.

Here is the example of a patch that removes a label from the secret:
````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-remove-label
spec :
  rules:
    - name: "Remove unwanted label"
      resource:
        kinds:
          - Secret
      mutate:
        patches:
        - path: "/metadata/labels/purpose"
          op: remove
````

Note, that if **remove** operation cannot be applied, then this **remove** operation will be skipped with no error.

## Overlay

An mutation overlay describes the desired form of resource. The existing resource values are replaced with the values specified in the overlay. If a value is specified in the overlay but not present in the target resource, then it will be added to the resource. The overlay cannot be used to delete values in a resource: use **patches** for this purpose.

The following mutation overlay will add (or replace) the memory request and limit to 10Gi for every Pod with a label ```memory: high```:

````yaml
apiVersion : kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-change-memory-limit
spec :
  rules:
    - name: "Set hard memory limit to 2Gi"
      resource:
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

Applying overlays to a list type without is fairly straightforward: new items will be added to the list, unless they already ecist. For example, the next overlay will add IP "192.168.10.172" to all addresses in all Endpoints:

````yaml
apiVersion: kyverno.io/v1alpha1
kind: Policy
metadata:
  name: policy-endpoints
spec:
  rules:
  - name: "Add IP to subsets"
    resource:
      kinds :
        - Endpoints
    mutate:
      overlay:
        subsets:
        - addresses:
          - ip: 192.168.42.172
````


### Conditional logic using anchors

An **anchor** field, marked by parentheses, allows conditional processing of configurations. Processing stops when the anchor value does not match. Once processing stops, any child elements or any remaining siblings in a list, will not be processed.

 For example, this overlay will add or replace the value 6443 for the port field, for all ports with a name value that starts with "secure":

````yaml
apiVersion: kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-set-port
spec :
  rules:
  - name: "Set port"
    resource:
      kinds :
        - Endpoints
    mutate:
      overlay:
        subsets:
        - ports:
          - (name): "secure*"
            port: 6443
````

The **anchors** values support **wildcards**:
1. `*` - matches zero or more alphanumeric characters
2. `?` - matches a single alphanumeric character


### Add if not present

A variation of an anchor, is to add a field value if it is not already defined. This is done by using the ````+(...)```` notation for the field.

 For example, this overlay will set the port to 6443, if a port is not already defined:

````yaml
apiVersion: kyverno.io/v1alpha1
kind : Policy
metadata :
  name : policy-set-port
spec :
  rules:
  - name: "Set port"
    resource:
      kinds :
        - Endpoints
    mutate:
      overlay:
        subsets:
        - (ports):
            +(port): 6443
````

## Additional Details

Additional details on mutation overlay behaviors are available on the wiki: [Mutation Overlay](https://github.com/nirmata/kyverno/wiki/Mutation-Overlay)

---
<small>*Read Next >> [Generate](/documentation/writing-policies-generate.md)*</small>
