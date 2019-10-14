# Best Practice Policies

Best practice policies are designed to be applied to your Kubernetes clusters with minimal changes. To import these policies [install Kyverno](../documentation/installation.md) and import the resources as follows:

````bash
kubectl create -f https://github.com/nirmata/kyverno/raw/master/samples/best_practices/
````

More information on each best-practice policy is provided below:

## Run as non-root user

By default, processes in a container run as a root user (uid 0). To prevent potential compromise of container hosts, specify a least privileged user ID when building the container image and require that application containers run as non root users i.e. set `runAsNonRoot` to `true`.

***Policy YAML***: [deny_runasrootuser.yaml](best_practices/deny_runasrootuser.yaml) 

**Additional Information**
* [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)


## Disallow automount of Service Account credentials

Kubernetes automounts default service account credentials in each pod. To restrict access, opt out of automounting credentials by setting `automountServiceAccountToken` to `false`.

***Policy YAML***: [disallow_automountingapicred.yaml](best_practices/disallow_automountingapicred.yaml) 


## Disallow use of default namespace

Namespaces are a way to segment and isolate cluster resources across multiple users. When multiple users or teams are sharing a single cluster, it is recommended to isolate different workloads and restrict use of the default namespace.

***Policy YAML***: [disallow_default_namespace.yaml](best_practices/disallow_default_namespace.yaml) 


## Disallow use of host filesystem

The volume of type `hostpath` binds pods to a specific host, and data persisted in the volume is dependent on the life of the node. In a shared cluster, it is recommeded that applications are independent of hosts.

***Policy YAML***: [disallow_host_filesystem.yaml](best_practices/disallow_host_filesystem.yaml) 


## Disallow `hostNetwork` and `hostPort`

Using `hostPort` and `hostNetwork` allows pods to share the host network stack, allowing potential snooping of network traffic from an application pod. 

***Policy YAML***: [disallow_host_network_hostport.yaml](best_practices/disallow_host_network_hostport.yaml)


## Disallow `hostPID` and `hostIPC`

Sharing the host's PID namespace allows visibility of process on the host, potentially exposing process information. 
Sharing the host's IPC namespace allows the container process to communicate with processes on the host. To avoid pod container from having visibility to host process space, validate that `hostPID` and `hostIPC` are set to `false`.

***Policy YAML***: [disallow_hostpid_hostipc.yaml](best_practices/disallow_hostpid_hostipc.yaml)


## Restrict service type `NodePort`  

A Kubernetes service of type NodePort uses a host port to receive traffic from any source. A `NetworkPolicy` resource cannot be used to control traffic to host ports. Although `NodePort` services can be useful, their use must be limited to services with additional upstream security checks. 

***Policy YAML***: [disallow_node_port.yaml](best_practices/disallow_node_port.yaml)


## Disable privileged containers

Privileged containers are defined as any container where the container uid 0 is mapped to the host’s uid 0. A process within privileged containers can get unrestricted host access. With `securityContext.allowPrivilegeEscalation` enabled a process can gain privileges from its parent.
To disallow privileged containers and the escalation of privileges it is recommended to run pod containers with `securityContext.priveleged` as `false` and `allowPrivilegeEscalation` as `false`.

***Policy YAML***: [disallow_priviledged_priviligedescalation.yaml](best_practices/disallow_priviledged_priviligedescalation.yaml)

## Default deny all ingress traffic

By default, Kubernetes allows all ingress and egress traffic to and from pods within a cluster. A "default" `NetworkPolicy` resource for a namespace should be used to deny all ingress traffic to the pods in that namespace. Additional `NetworkPolicy` resources can then be configured to allow desired traffic to application pods.

***Policy YAML***: [require_default_network_policy.yaml](best_practices/require_default_network_policy.yaml)


## Disallow latest image tag

The `:latest` tag is mutable and can lead to unexpected errors if the image changes. A best practice is to use an immutable tag that maps to a specific version of an application pod.

***Policy YAML***: [require_image_tag_not_latest.yaml](best_practices/require_image_tag_not_latest.yaml)

## Configure namespace limits and quotas

To limit the number of objects, as well as the total amount of compute that may be consumed by an application, it is important to create resource limits and quotas for each namespace.

***Policy YAML***: [require_namespace_quota.yaml](best_practices/require_namespace_quota.yaml) 

**Additional Information**
* [Resource Quota](https://kubernetes.io/docs/concepts/policy/resource-quotas/)


## Require pod resource requests and limits

As application workloads share cluster resources, it is important to limit resources requested and consumed by each pod. It is recommended to require `resources.requests` and `resources.limits` per pod. If a namespace level request or limit is specified, defaults will automatically be applied to each pod based on the `LimitRange` configuration. 

***Policy YAML***: [require_pod_requests_limits.yaml](best_practices/require_pod_requests_limits.yaml)


## Require `livenessProbe` and `readinessProbe`

For each pod, a `livenessProbe` is carried out by the kubelet to determine when to restart a container. A `readinessProbe` is used by services and deployments to determine if the pod is ready to recieve network traffic.
Both liveness and readiness probes need to be configured to manage the pod lifecycle during restarts and upgrades.

***Policy YAML***: [require_probes.yaml](best_practices/require_probes.yaml)


## Read-only root filesystem

A read-only root file system helps to enforce an immutable infrastructure strategy; the container only needs to write on the mounted volume that persists the state. An immutable root filesystem can also prevent malicious binaries from writing to the host system.

***Policy YAML***: [require_readonly_rootfilesystem.yaml](best_practices/require_readonly_rootfilesystem.yaml)


## Disallow unknown image registries

Images from unknown registries may not be scanned and secured. Requiring use of known registries helps reduce threat exposure. You can customize this policy to allow image registries that you trust.

***Policy YAML***: [trusted_image_registries.yaml](best_practices/trusted_image_registries.yaml) 


# More Policies

The policies listed here provide additional best practices that should be considered for production use. These policies may require workload specific configutration. 

## Assign Linux capabilities inside Pod

Linux divides the privileges traditionally associated with superuser into distinct units, known as capabilities, which can be independently enabled or disabled by listing them in `securityContext.capabilites`. 

***Policy YAML***: [policy_validate_container_capabilities.yaml](more/policy_validate_container_capabilities.yaml)

**Additional Information**
* [List of linux capabilities](https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h)


## Check userID, groupIP & fsgroup used inside a Pod
All processes inside the pod can be made to run with specific user and groupID by setting `runAsUser` and `runAsGroup` respectively. `fsGroup` can be specified to make sure any file created in the volume with have the specified groupID. These options can be used to validate the IDs used for user and group.

***Policy YAML***: [policy_validate_container_capabilities.yaml](more/policy_validate_user_group_fsgroup_id.yaml)


## Configure kernel parameters inside pod
The Sysctl interface allows to modify kernel parameters at runtime and in the pod can be specified under `securityContext.sysctls`. If kernel parameters in the pod are to be modified, should be handled cautiously, and policy with rules restricting these options will be helpful. We can control minimum and maximum port that a network connection can use as its source(local) port by checking net.ipv4.ip_local_port_range

***Policy YAML***: [policy_validate_container_capabilities.yaml](more/policy_validate_user_group_fsgroup_id.yaml)

**Additional Information**
* [List of supported namespaced sysctl interfaces](https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/) 
