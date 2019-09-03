# Best Practice Policies

This folder contains recommended policies

| Best practice                                  | Policy
|------------------------------------------------|-----------------------------------------------------------------------|-
| Run as non-root user                           | 
| Disallow privileged and privilege escalation   |
| Disallow use of host networking and ports      |
| Disallow use of host filesystem                |
| Disallow hostPOD and hostIPC                   |
| Require read only root filesystem              |
| Disallow node ports                            |
| Allow trusted registries                       |
| Require resource requests and limits           | [container_resources.yaml](container_resources.yaml)
| Require pod liveness and readiness probes      |
| Require an image tag                           |
| Disallow latest tag and pull IfNotPresent      |
| Require a namespace (disallow default)         |
| Disallow use of kube-system namespace          |
| Prevent mounting of service account secret     |
| Require a default network policy               |
| Require namespace quotas and limit ranges      |
