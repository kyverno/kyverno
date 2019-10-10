# Best Practice Policies

| Best practice                                  | Policy                             |             scenario|
|------------------------------------------------|------------------------------------|---------------------|
| Run as non-root user                           | [policy_validate_deny_runasrootuser.yaml](policy_validate_deny_runasrootuser.yaml)                  |     best_practices            |
| Disallow automount api credentials                           | []()                  |     best_practices            |
| Disallow privileged and privilege escalation   | [policy_validate_container_disallow_priviledgedprivelegesecalation.yaml](policy_validate_container_disallow_priviledgedprivelegesecalation.yaml)             |      best_practices            |
| Disallow use of host networking and ports      |  [policy_validate_host_network_port.yaml](policy_validate_host_network_port.yaml)             |        best_practices            |
| Disallow use of host filesystem                |  [policy_validate_host_path.yaml](policy_validate_host_path.yaml)                                                                                            |
| Disallow hostPID and hostIPC                   |  [policy_validate_hostpid_hosipc.yaml](policy_validate_hostpid_hosipc.yaml)                                                      |      best_practices            |
| Require read only root filesystem              | [policy_validate_not_readonly_rootfilesystem.yaml](policy_validate_not_readonly_rootfilesystem.yaml)                     |      best_practices            |
| Disallow node ports                            | [policy_validate_disallow_node_port.yaml](policy_validate_disallow_node_port.yaml)                           |      best_practices            |
| Allow trusted registries                       | [policy_validate_whitelist_image_registries.yaml](policy_validate_whitelist_image_registries.yaml)                                                                               |     best_practices            |
| Require resource requests and limits           | [policy_validate_pod_resources.yaml](policy_validate_pod_resources.yaml)                                           |      best_practices            |
| Require pod liveness and readiness probes      | [policy_validate_pod_probes.yaml](policy_validate_pod_probes.yaml)                                            |     best_practices            |
| Require an image tag                           | [policy_validate_image_tag_notspecified_deny.yaml](policy_validate_image_tag_notspecified_deny.yaml)                                                         |   best_practices            |
| Disallow latest tag and pull IfNotPresent      | [policy_validate_image_latest_ifnotpresent_deny.yaml](policy_validate_image_latest_ifnotpresent_deny.yaml)                                                   |
| Require a namespace (disallow default)         | [policy_validate_default_namespace.yaml](policy_validate_default_namespace.yaml)                                                                     |     best_practices            |
| Prevent mounting of default service account    | [policy_validate_disallow_default_serviceaccount.yaml](policy_validate_disallow_default_serviceaccount.yaml)                                                                      |
| Require a default network policy               | [policy_validate_default_network_policy.yaml](policy_validate_default_network_policy.yaml)                                                                      |   best_practices            |
| Require namespace quotas and limit ranges      | [policy_validate_namespace_quota.yaml](policy_validate_namespace_quota.yaml)                                                                      |     best_practices            |
| Allow an FSGroup that owns the pod's volumes      | [policy_validate_fsgroup.yaml](policy_validate_fsgroup.yaml)                                                                      |
| Require SELinux level of the container      | [policy_validate_selinux_context.yaml](policy_validate_selinux_context.yaml)                                                                      |
| Allow default Proc Mount type      | [policy_validate_default_proc_mount.yaml](policy_validate_default_proc_mount.yaml)                                                                      |
| Allow certain capability to be added      | [policy_validate_container_capabilities.yaml](policy_validate_container_capabilities.yaml)                                                                      |
| Allow local tcp/udp port range      | [policy_validate_sysctl_configs.yaml](policy_validate_sysctl_configs.yaml)                                                                      |
| Allowed volume plugins      | [policy_validate_volume_whitelist.yaml](policy_validate_volume_whitelist.yaml)                                                                      |