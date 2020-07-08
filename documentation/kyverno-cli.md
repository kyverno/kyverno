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
kuberctl kyverno version  

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

#### Apply
Applies policies on resources, and supports applying multiple policies on multiple resources in a single command.
Also supports applying the given policies to an entire cluster. The current kubectl context will be used to access the cluster.
 Will return results to stdout.

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


<small>*Read Next >> [Sample Policies](/samples/README.md)*</small>
