<small>[documentation](/README.md#documentation) / Installation</small>

# Installation

To install Kyverno in your cluster run:

`kubectl create -f definitions/install.yaml`

To check if the Kyverno controller

`kubectl get pods -n kyverno`

If the Kyverno controller doesn't start, you can check its status and logs:

`kubectl describe pod <kyverno-pod-name> -n kyverno`

`kubectl logs <kyverno-pod-name> -n kyverno`

# Installing in a Development Environment

To run Kyverno in a development environment see: https://github.com/nirmata/kyverno/wiki/Building

# Try Kyverno without a Kubernetes cluster

To write and test policies without installing Kyverno in a Kubernetes cluster you can try the [Kyverno CLI](documentation/testing-policies-cli.md).


<small>Read Next >> [Writing Policies](/documentation/writing-policies.md)</small>