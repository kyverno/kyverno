# Disallow `hostPID` and `hostIPC`

Sharing the host's PID namespace allows an application pod to gain visibility of processes on the host, potentially exposing sensitive information. Sharing the host's IPC namespace also allows the container process to communicate with processes on the host. 

To avoid pod container from having visibility to host process space, validate that `hostPID` and `hostIPC` are set to `false`.

## Policy YAML 

[disallow_hostpid_hostipc.yaml](best_practices/disallow_hostpid_hostipc.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-hostpid-hostipc
spec:
  rules:
  - name: validate-hostpid-hostipc
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Disallow use of host's pid namespace and host's ipc namespace"
      pattern:
        spec:
          (hostPID): "!true"
          hostIPC: false
````
