# Best Practice Policies

Best practice policies are recommended policies that can be applied to yoru Kubernetes clusters with minimal changes. To import these policies [install Kyverno](../documentation/installation.md) and import the resources as follows:

````bash
kubectl create -f https://github.com/nirmata/kyverno/raw/master/samples/best_practices/
````

More information on each best-practice policy is provided below:

## Run as non-root user

By default, processes in a container run as a root user (uid 0). To prevent compromising the host, a best practice is to specify a least privileged user ID when building the container image, and require that application containers run as non root users. 

***Policy YAML***: [deny_runasrootuser.yaml](best_practices/deny_runasrootuser.yaml) 

**Aditional Information**
* [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)


## hostNetwork and hostPort not allowed

Using `hostPort` and `hostNetwork` limits the number of nodes the pod can be scheduled on, as the pod is bound to the host thats its mapped to.
To avoid this limitation, use a validate rule to make sure these attributes are set to null and false.

***Policy YAML*** [disallow_host_network_hostport.yaml](best_practices/disallow_host_network_hostport.yaml)


## Read-only root filesystem

A read-only root file system helps to enforce an immutable infrastrucutre strategy, the container only need to write on mounted volume that persist the state. An immutable root filesystem can also prevent malicious binaries from writing to the host system.

***Policy YAML*** [require_readonly_rootfilesystem.yaml](best_practices/require_readonly_rootfilesystem.yaml)


## Disallow hostPID and hostIPC
Sharing the host's PID namespace allows vibility of process on the host, potentially exposing porcess information. 
Sharing the host's IPC namespace allows container process to communicate with processes on the host. 
To avoid pod container from having visilbility to host process space, we can check `hostPID` and `hostIPC` are set as `false`.

***Policy YAML***[disallow_hostpid_hostipc.yaml](best_practices/disallow_hostpid_hostipc.yaml)


## Disallow node port
Node port ranged service is advertised to the public and can be scanned and probed from others exposing all nodes.
NetworkPolicy resources can currently only control NodePorts by allowing or disallowing all traffic on them. Unless required it is recommend to disable use to service type `NodePort`.

***Policy YAML***[disallow_node_port.yaml](best_practices/disallow_node_port.yaml)

## Disable privileged containers
A process within priveleged containers get almost the same priveleges that are available to processes outside a container providing almost unrestricited host access. With `securityContext.allowPrivilegeEscalation` enabled the process can gain ore priveleges that its parent.
To restrcit the priveleges it is recommend to run pod containers with `securityContext.priveleged` as `false` and 
`allowPrivilegeEscalation` as `false`

***Policy YAML***[disallow_priviledged_priviligedescalation.yaml](best_practices/disallow_priviledged_priviligedescalation.yaml)
# Additional Policies

| Description                                       	| Policy                                                                                                                                                                                           	| Details                                                                                                                                                                                                                                                                                                                                     	|
|---------------------------------------------------	|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|
| Check userID, groupIP & fsgroup used inside a Pod 	| [Restrict the range of ids used inside a Pod](additional/policy_validate_user_group_fsgroup_id.yaml)                                                                                             	| All processes inside the pod can be made to run with specific user and groupID by setting runAsUser and runAsGroup respectively. fsGroup can be specified to make sure any file created in the volume with have the specified groupID.                                                                                                      	|
| Assign Linux capabilities inside Pod              	| [Verify capabilities add in a Pod](additional/policy_validate_container_capabilities.yaml)                                                                                                       	| Linux divides the privileges traditionally, associated with superuser into distinct units, known as capabilities, which can be independently enabled and disabled by specifying them in capabilities section of securityContext. [List of linux capabilities](https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h 	|
| Configure kernel parameters                       	| [The minimum and maximum port a network connection can use as its source(local) port can be validating by checking net.ipv4.ip_local_port_range](additional/policy_validate_sysctl_configs.yaml) 	| Sysctl interface allows to modify kernel parameters at runtime and can be specified in the sysctls section of securityContext. [list of supported namespaced sysctl interfaces](https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/)                                                                                        	|