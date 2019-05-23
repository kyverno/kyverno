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

The Kyverno Command Line Interface (CLI) tool enables writing and testing policies without requiring Kubernetes clusters and without having to apply local policy changes to a cluster.

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

To test a policy using the CLI type:

`kyverno <policy> <resource YAML file or folder>`

For example:

```bash
kyverno ../../examples/CLI/policy-deployment.yaml ../../examples/CLI/resources
```

In future releases, the CLI will support complete validation of policies and will allow testing policies against resources in Kubernetes clusters.
