<small>*[documentation](/README.md#documentation) / Installation*</small>

# Installation

To install Kyverno in your cluster run the following command on a host with kubectl access:

````sh
kubectl create -f https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml
````

To check the Kyverno controller status, run the command:

````sh
kubectl get pods -n kyverno
````

If the Kyverno controller is not running, you can check its status and logs for errors:

````sh
kubectl describe pod <kyverno-pod-name> -n kyverno
````

````sh
kubectl logs <kyverno-pod-name> -n kyverno
````

# Installing in a Development Environment

To run Kyverno in a development environment see: https://github.com/nirmata/kyverno/wiki/Building

# Try Kyverno without a Kubernetes cluster

The [Kyverno CLI](documentation/testing-policies-cli.md) allows you to write and test policies without installing Kyverno in a Kubernetes cluster.


<small>*Read Next >> [Writing Policies](/documentation/writing-policies.md)*</small>
