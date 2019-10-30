# Sample Policies

Sample policies are designed to be applied to your Kubernetes clusters with minimal changes. To apply these policies to your cluster, install Kyverno and import the policies as follows:

**Install Kyverno**

````sh
kubectl create -f https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml
````
<small>[(installation docs)](../documentation/installation.md)</small>

**Apply Kyverno Policies**

````bash

kubectl create -f [samples/best_practices/](best_practices/)

kubectl create -f [samples/more/](more/)
````

The policies are mostly validation rules in `audit` mode i.e. your existing workloads will not be impacted, but will be audited for policy complaince.

## Best Practice Policies

These policies are highly recommended.

1. [Run as non-root user](RunAsNonRootUser.md)
2. [Disable privileged containers and disallow privilege escalation](DisablePrivilegedContainers.md)
3. [Require Read-only root filesystem](RequireReadOnlyFS.md)
4. [Disallow use of host filesystem](DisallowHostFS.md)
5. [Disallow `hostNetwork` and `hostPort`](DisallowHostNetworkPort.md)
6. [Disallow `hostPID` and `hostIPC`](DisallowHostPIDIPC.md)
7. [Disallow unknown image registries](DisallowUnknownRegistries.md)
8. [Disallow latest image tag](DisallowLatestTag.md)
9. [Disallow use of default namespace](DisallowDefaultNamespace.md)
10. [Require namespace limits and quotas](RequireNSLimitsQuotas.md)
11. [Require pod resource requests and limits](RequirePodRequestsLimits.md)
12. [Require pod `livenessProbe` and `readinessProbe`](RequirePodProbes.md)
13. [Default deny all ingress traffic](DefaultDenyAllIngress.md)


## Additional Policies

The policies provide additional best practices and are worthy of close consideration. These policies may require workload specific changes. 

14. [Limit use of `NodePort` services](LimitNodePort.md)
15. [Limit automount of Service Account credentials](DisallowAutomountSACredentials.md)
16. [Configure Linux Capabilities](AssignLinuxCapabilities.md)
17. [Limit Kernel parameter access](ConfigureKernelParmeters.md)



