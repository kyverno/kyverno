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

*This feature will be available soon*

## Autotest

*Will be available after Kyverno CLI is implemented*