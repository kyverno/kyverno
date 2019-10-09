# Best Practice Policies

Best practice policies are recommended policies that can be applied to yoru Kubernetes clusters with minimal changes. To import these policies [install Kyverno](../documentation/installation.md) and import the resources as follows:

````bash
kubectl create -f https://github.com/nirmata/kyverno/raw/master/samples/best_practices/
````

More information on each best-practice policy is provided below:

## Run as non-root user

**Description**:  By default, processes in a container run as a root user (uid 0). To prevent compromising the host, a best practice is to specify a least privileged user ID when building the container image, and require that application containers run as non root users. 

**Policy YAML**: [deny_runasrootuser.yaml](best_practices/deny_runasrootuser.yaml) 

**Aditional Information**
* [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)


# Additional Policies

| Description                                       	| Policy                                                                                                                                                                                           	| Details                                                                                                                                                                                                                                                                                                                                     	|
|---------------------------------------------------	|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------	|
| Check userID, groupIP & fsgroup used inside a Pod 	| [Restrict the range of ids used inside a Pod](additional/policy_validate_user_group_fsgroup_id.yaml)                                                                                             	| All processes inside the pod can be made to run with specific user and groupID by setting runAsUser and runAsGroup respectively. fsGroup can be specified to make sure any file created in the volume with have the specified groupID.                                                                                                      	|
| Assign Linux capabilities inside Pod              	| [Verify capabilities add in a Pod](additional/policy_validate_container_capabilities.yaml)                                                                                                       	| Linux divides the privileges traditionally, associated with superuser into distinct units, known as capabilities, which can be independently enabled and disabled by specifying them in capabilities section of securityContext. [List of linux capabilities](https://github.com/torvalds/linux/blob/master/include/uapi/linux/capability.h 	|
| Configure kernel parameters                       	| [The minimum and maximum port a network connection can use as its source(local) port can be validating by checking net.ipv4.ip_local_port_range](additional/policy_validate_sysctl_configs.yaml) 	| Sysctl interface allows to modify kernel parameters at runtime and can be specified in the sysctls section of securityContext. [list of supported namespaced sysctl interfaces](https://kubernetes.io/docs/tasks/administer-cluster/sysctl-cluster/)                                                                                        	|