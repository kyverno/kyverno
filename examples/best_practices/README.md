# Best Practice Policies

| Best practice                                  | Policy
|------------------------------------------------|-----------------------------------------------------------------------|
| Run as non-root user                           | [policy_validate_deny_runasrootuser.yaml](policy_validate_deny_runasrootuser.yaml)                                                                           |
| Disallow privileged and privilege escalation   | [policy_validate_container_disallow_priviledgedprivelegesecalation.yaml](policy_validate_container_disallow_priviledgedprivelegesecalation.yaml)             |
| Disallow use of host networking and ports      |  [policy_validate_host_network_port.yaml](policy_validate_host_network_port.yaml)                                                                            |
| Disallow use of host filesystem                |  [policy_validate_host_path.yaml](policy_validate_host_path.yaml)                                                                                            |
| Disallow hostPID and hostIPC                   |  [policy_validate_hostpid_hosipc.yaml](policy_validate_hostpid_hosipc.yaml)                                                                     |
| Require read only root filesystem              | [policy_validate_not_readonly_rootfilesystem.yaml](policy_validate_not_readonly_rootfilesystem.yaml)                                                                      |
| Disallow node ports                            |                                                                                                                  |
| Allow trusted registries                       | [policy_validate_image_registries.yaml](policy_validate_image_registries.yaml)                                                                               |
| Require resource requests and limits           | [policy_validate_pod_resources.yaml](policy_validate_pod_resources.yaml)                                                                                     |
| Require pod liveness and readiness probes      | [policy_validate_pod_probes.yaml](policy_validate_pod_probes.yaml)                                                                                           |
| Require an image tag                           | [policy_validate_image_tag_notspecified_deny.yaml](policy_validate_image_tag_notspecified_deny.yaml)                                                         |
| Disallow latest tag and pull IfNotPresent      | [policy_validate_image_latest_ifnotpresent_deny.yaml](policy_validate_image_latest_ifnotpresent_deny.yaml)                                                   |
| Require a namespace (disallow default)         | [policy_validate_default_namespace.yaml](policy_validate_default_namespace.yaml)                                                                     |
| Disallow use of kube-system namespace          |                                                                       |
| Prevent mounting of service account secret     |                                                                       |
| Require a default network policy               | [policy_validate_default_network_policy.yaml](policy_validate_default_network_policy.yaml)                                                                      |
| Require namespace quotas and limit ranges      | [policy_validate_namespace_quota.yaml](policy_validate_namespace_quota.yaml)                                                                      |
