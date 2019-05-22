# Installation

The controller can be installed and operated in two ways: **Outside the cluster** and **Inside the cluster**. The controller **outside** the cluster is much more convenient to debug and verify changes in its code, so we can call it 'debug mode'. The controller **inside** the cluster is designed for use in the real world, and the **QA testing** should be performed when controller operate in this mode.


## Inside the cluster (normal use)

Just execute the command for creating all necesarry resources:
`kubectl create -f definitions/install.yaml`

In this mode controller will get TLS key/certificate pair and loads in-cluster config automatically on start.
To check if the controller is working, find it in the list of kyverno pods:

`kubectl get pods -n kyverno`

The pod with controller contains **'kyverno'** in its name. The STATUS column will show the health state of the controller. If controller doesn't start, see its logs:

`kubectl describe pod <kyverno-pod-name> -n kyverno`

or

`kubectl logs <kyverno-pod-name> -n kyverno`

