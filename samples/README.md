# Best Practice Policies

Best practice policies are recommended policies that can be applied to yoru Kubernetes clusters with minimal changes. To import these policies [install Kyverno](../documentation/installation.md) and import the resources as follows:

````bash
kubectl create -f https://github.com/nirmata/kyverno/raw/master/samples/best_practices/
````

More information on each best-practice policy is provided below:


## Run as non-root user

By default, processes in a container run as a root user (uid 0). To prevent compromising the host, a best practice is to specify a least privileged user ID when building the container image, and require that application containers run as non root users. 

***Policy YAML***: [deny_runasrootuser.yaml](best_practices/deny_runasrootuser.yaml) 

**Additional Information**
* [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)


## Disallow automounte API credentials

One can access the API from inside a pod using automatically mounted service account credentials by default. To restrict access, opt out of automounting API credentials for any pod by setting `automountServiceAccountToken` to `false`.

***Policy YAML***: [disallow_automountingapicred.yaml](best_practices/disallow_automountingapicred.yaml) 


## Disallow use of default namespace

Namespaces are a way to divide cluster resources between multiple users. When multiple users or teams are sharing a single cluster, it is recommended to isolate different workloads and aviod using default namespace.

***Policy YAML***: [disallow_default_namespace.yaml](best_practices/disallow_default_namespace.yaml) 


## Disallow use of host filesystem

Using the volume of type hostpath can easily lose data when a node crashes. Disable use of hostpath prevent data loss. 

***Policy YAML***: [disallow_host_filesystem.yaml](best_practices/disallow_host_filesystem.yaml) 


## Disallow `hostNetwork` and `hostPort`

Using `hostPort` and `hostNetwork` limits the number of nodes the pod can be scheduled on, as the pod is bound to the host thats its mapped to.
To avoid this limitation, use a validate rule to make sure these attributes are set to null and false.

***Policy YAML***: [disallow_host_network_hostport.yaml](best_practices/disallow_host_network_hostport.yaml)

## Disallow `hostPID` and `hostIPC`

Sharing the host's PID namespace allows vibility of process on the host, potentially exposing porcess information. 
Sharing the host's IPC namespace allows container process to communicate with processes on the host. 
To avoid pod container from having visilbility to host process space, we can check `hostPID` and `hostIPC` are set as `false`.

***Policy YAML***: [disallow_hostpid_hostipc.yaml](best_practices/disallow_hostpid_hostipc.yaml)

## Disallow node port

Node port ranged service is advertised to the public and can be scanned and probed from others exposing all nodes.
NetworkPolicy resources can currently only control NodePorts by allowing or disallowing all traffic on them. Unless required it is recommend to disable use to service type `NodePort`.

***Policy YAML***: [disallow_node_port.yaml](best_practices/disallow_node_port.yaml)

## Disable privileged containers

A process within priveleged containers get almost the same priveleges that are available to processes outside a container providing almost unrestricited host access. With `securityContext.allowPrivilegeEscalation` enabled the process can gain ore priveleges that its parent.
To restrcit the priveleges it is recommend to run pod containers with `securityContext.priveleged` as `false` and 
`allowPrivilegeEscalation` as `false`

***Policy YAML***: [disallow_priviledged_priviligedescalation.yaml](best_practices/disallow_priviledged_priviligedescalation.yaml)

## Default deny all ingress traffic

When no policies exist in a namespace, Kubernetes allows all ingress and egress traffic to and from pods in that namespace. A "default" isolation policy for a namespace denys any ingress traffic to the pods in that namespace, this ensures that even pods that aren’t selected by any other NetworkPolicy will still be isolated.

***Policy YAML***: [require_default_network_policy.yaml](best_practices/require_default_network_policy.yaml)

## Disallow latest image tag

Using the `:latest` tag when deploying containers in production makes it harder to track which version of the image is running and more difficult to roll back properly. Specifying a none latest image tag prevents a lot of errors from occurring when versions are mismatched.

***Policy YAML***: [require_image_tag_not_latest.yaml](best_practices/require_image_tag_not_latest.yaml)


## Default namesapce quotas

In order to limit the quantity of objects, as well as the total amount of compute resources that may be consumed by an application, it is essential to create one resource quota for each namespace by cluster administrator.

**Additional Information**
* [Resource Quota](https://kubernetes.io/docs/concepts/policy/resource-quotas/)

***Policy YAML***: [require_namespace_quota.yaml](best_practices/require_namespace_quota.yaml) 


## Require pod resource requests and limits

As workloads share the host cluster, it is essential to administer and limit resources requested and consumed by the pod. It is a good practice to always specify `resources.requests` and `resources.limits` per pod.

***Policy YAML***: [require_pod_requests_limits.yaml](best_practices/require_pod_requests_limits.yaml)

## Default health probe

Setting the health probe ensures an application is highly-avaiable and resilient. Health checks are a simple way to let the system know if an application is broken, and it helps the application quickly recover from failure.

***Policy YAML***: [require_probes.yaml](best_practices/require_probes.yaml)


## Read-only root filesystem

A read-only root file system helps to enforce an immutable infrastrucutre strategy, the container only need to write on mounted volume that persist the state. An immutable root filesystem can also prevent malicious binaries from writing to the host system.

***Policy YAML***: [require_readonly_rootfilesystem.yaml](best_practices/require_readonly_rootfilesystem.yaml)


## Trusted image registries

Images from unrecognized registry can introduce complexity to maintain the application. By specifying trusted registries help reducing such complexity. Follow instructoin [here](https://github.com/nirmata/kyverno/blob/master/documentation/writing-policies-validate.md#operators) to add allowed registries using `OR` operator.

***Policy YAML***: [trusted_image_registries.yaml](best_practices/trusted_image_registries.yaml) 


# Additional Policies
Additional policies list some policies that can also assist in maintaing kubernetes clusters.

## Assign Linux capabilities inside Pod
Linux divides the privileges traditionally, associated with superuser into distinct units, known as capabilities, which can be independently enabled or disabled by listing them in `securityContext.capabilites`. 


***Policy YAML***: [policy_validate_container_capabilities.yaml](best_practices/policy_validate_container_capabilities.yaml)

**Additional Information**
* [List of linux capabilities](https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h)

## Check userID, groupIP & fsgroup used inside a Pod
All processes inside the pod can be made to run with specific user and groupID by setting runAsUser and runAsGroup respectively. fsGroup can be specified to make sure any file created in the volume with have the specified groupID. These options can be used validate the IDs used for user and group.

***Policy YAML***: [policy_validate_container_capabilities.yaml](best_practices/policy_validate_user_group_fsgroup_id.yaml)

## Configure kernel parameters inside pod
Sysctl interface allows to modify kernel parameters at runtime and in the pod can be specified under `securityContext.sysctls`. If kernel parameters in the pod are to be modified should be handled cautiosly, and a policy with rules restricting these options will be helpful. We can control minimum and maximum port that a network connection can use as its source(local) port by checking net.ipv4.ip_local_port_range

***Policy YAML***: [policy_validate_container_capabilities.yaml](best_practices/policy_validate_user_group_fsgroup_id.yaml)

**Additional Information**
* [List of supported namespaced sysctl interfaces](https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/) 
