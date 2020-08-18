<small>*[documentation](/README.md#documentation) / kyverno-cli*</small>


# Kyverno CLI

The Kyverno Command Line Interface (CLI) is designed to validate policies and test the behavior of applying policies to resources before adding the policy to a cluster. It can be used as a kubectl plugin and as a standalone CLI.

## Build the CLI

You can build the CLI binary locally, then move the binary into a directory in your PATH.

```bash
git clone https://github.com/nirmata/kyverno.git
cd github.com/nirmata/kyverno
make cli
mv ./cmd/cli/kubectl-kyverno/kyverno /usr/local/bin/kyverno
```

You can also use [Krew](https://github.com/kubernetes-sigs/krew)
```bash
# Install kyverno using krew plugin manager
kubectl krew install kyverno 

#example 
kubectl kyverno version  

```

## Install via AUR (archlinux)

You can install the kyverno cli via your favourite AUR helper (e.g. [yay](https://github.com/Jguer/yay))

```
yay -S kyverno-git
```

## Commands

#### Version

Prints the version of kyverno used by the CLI.

Example: 
```
kyverno version
```

#### Validate
Validates a policy, can validate multiple policy resource description files or even an entire folder containing policy resource description 
files. Currently supports files with resource description in yaml.

Example:
```
kyverno validate /path/to/policy1.yaml /path/to/policy2.yaml /path/to/folderFullOfPolicies
```

Use the -o <yaml/json> flag to display the mutated policy.

Example:
```
kyverno validate /path/to/policy1.yaml /path/to/policy2.yaml /path/to/folderFullOfPolicies -o yaml
```


#### Apply
Applies policies on resources, and supports applying multiple policies on multiple resources in a single command.
Also supports applying the given policies to an entire cluster. The current kubectl context will be used to access the cluster.

Displays mutate results to stdout, by default. Use the -o <path> flag to save mutated resources to a file or directory.

Apply to a resource:
```
kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml
```

Apply to all matching resources in a cluster:
```
kyverno apply /path/to/policy.yaml --cluster > policy-results.txt
```

Apply multiple policies to multiple resources:
```
kyverno apply /path/to/policy1.yaml /path/to/folderFullOfPolicies --resource /path/to/resource1.yaml --resource /path/to/resource2.yaml --cluster
```

Saving the mutated resource in a file/directory:
```
kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml -o <file path/directory path>
```

Apply policy with variables:

Use --set flag to pass the values for variables in a policy while applying on a resource.

```
kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --set <variable1>=<value1>,<variable2>=<value2>
```

Use --values_file for applying multiple policies on multiple resources and pass a file containing variables and its values.

```
kyverno apply /path/to/policy1.yaml /path/to/policy2.yaml --resource /path/to/resource1.yaml --resource /path/to/resource2.yaml -f /path/to/value.yaml
```

Format of value.yaml :

```
policies:
  - name: <policy1 name>
    resources:
      - name: <resource1 name>
        values:
          <variable1 in policy1>: <value>
          <variable2 in policy1>: <value>
      - name: <resource2 name>
        values:
          <variable1 in policy1>: <value>
          <variable2 in policy1>: <value>
  - name: <policy2 name>
    resources:
      - name: <resource1 name>
        values:
          <variable1 in policy2>: <value>
          <variable2 in policy2>: <value>
      - name: <resource2 name>
        values:
          <variable1 in policy2>: <value>
          <variable2 in policy2>: <value>
```

Example:

Policy file(add_network_policy.yaml):

```
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-networkpolicy
  annotations:
    policies.kyverno.io/category: Workload Management
    policies.kyverno.io/description: By default, Kubernetes allows communications across 
      all pods within a cluster. Network policies and, a CNI that supports network policies, 
      must be used to restrict communinications. A default NetworkPolicy should be configured 
      for each namespace to default deny all ingress traffic to the pods in the namespace. 
      Application teams can then configure additional NetworkPolicy resources to allow 
      desired traffic to application pods from select sources.
spec:
  rules:
  - name: default-deny-ingress
    match:
      resources: 
        kinds:
        - Namespace
        name: "*"
    generate: 
      kind: NetworkPolicy
      name: default-deny-ingress
      namespace: "{{request.object.metadata.name}}"
      synchronize : true
      data:
        spec:
          # select all pods in the namespace
          podSelector: {}
          policyTypes: 
          - Ingress
```
Resource file(required_default_network_policy.yaml) :

```
kind: Namespace
apiVersion: v1
metadata: 
    name: "devtest"
```
Applying policy on resource using set/-s flag:

```
kyverno apply /path/to/add_network_policy.yaml --resource /path/to/required_default_network_policy.yaml -s request.object.metadata.name=devtest
```

Applying policy on resource using --values_file/-f flag:

yaml file with variables(value.yaml) :

```
policies:
  - name: default-deny-ingress
    resources:
      - name: devtest
        values:
          request.namespace: devtest
```

```
kyverno apply /path/to/add_network_policy.yaml --resource /path/to/required_default_network_policy.yaml -f /path/to/value.yaml
```


<small>*Read Next >> [Sample Policies](/samples/README.md)*</small>
