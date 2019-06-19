<small>*[documentation](/README.md#documentation) / Testing Policies*</small>


# Testing Policies 
The resources definitions for testing are located in [/test](/test) directory. Each test contains a pair of files: one is the resource definition, and the second is the kyverno policy for this definition.

## Test using kubectl
To do this you should [install kyverno to the cluster](/documentation/installation.md).

For example, to test the simplest kyverno policy for ConfigMap, create the policy and then the resource itself via kubectl:

````bash
cd test/ConfigMap
kubectl create -f policy-CM.yaml
kubectl create -f CM.yaml
````
Then compare the original resource definition in CM.yaml with the actual one:

````bash
kubectl get -f CM.yaml -o yaml
````

## Test using the Kyverno CLI

The Kyverno Command Line Interface (CLI) tool allows writing and testing policies without having to apply local policy changes to a cluster. You can also test policies without a Kubernetes clusters, but results may vary as default values will not be filled in.

### Building the CLI

You will need a [Go environment](https://golang.org/doc/install) setup.

1. Clone the Kyverno repo

````bash
git clone https://github.com/nirmata/kyverno/
````

2. Build the CLI

````bash
cd kyverno/cmd/kyverno
go build
````

Or, you can directly build and install the CLI using `go get`:

````bash
go get -u https://github.com/nirmata/kyverno/cmd/kyverno
````

### Using the CLI

The CLI loads default kubeconfig ($HOME/.kube/config) to test policies in Kubernetes cluster. If no kubeconfig is found, the CLI will test policies on raw resources.

To test a policy using the CLI type:

`kyverno apply @<policy> @<resource YAML file or folder>`

For example:

```bash
kyverno apply @../../examples/cli/policy-deployment.yaml @../../examples/cli/resources
```

To test a policy with the specific kubeconfig:

```bash
kyverno apply @../../examples/cli/policy-deployment.yaml @../../examples/cli/resources --kubeconfig $PATH_TO_KUBECONFIG_FILE
```

In future releases, the CLI will support complete validation and generation of policies.
