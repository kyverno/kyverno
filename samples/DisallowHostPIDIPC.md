# Disallow `hostPID` and `hostIPC`

Sharing the host's PID namespace allows an application pod to gain visibility of processes on the host, potentially exposing sensitive information. Sharing the host's IPC namespace also allows the container process to communicate with processes on the host. 

To avoid pod container from having visibility to host process space, validate that `hostPID` and `hostIPC` are set to `false`.

## Policy YAML 

[disallow_hostpid_hostipc.yaml](best_practices/disallow_hostpid_hostipc.yaml)

````yaml
apiVersion: kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-host-pid-ipc
  annotations:
    policies.kyverno.io/category: Security
    policies.kyverno.io/description: Sharing the host's PID namespace allows visibility of process 
      on the host, potentially exposing process information. Sharing the host's IPC namespace allows 
      the container process to communicate with processes on the host. To avoid pod container from 
      having visibility to host process space, validate that 'hostPID' and 'hostIPC' are set to 'false'.
spec:
  validationFailureAction: enforce
  rules:
  - name: validate-host-pid-ipc
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Use of host PID and IPC namespaces is not allowed"
      pattern:
        spec:
          =(hostPID): "false"
          =(hostIPC): "false"
````
