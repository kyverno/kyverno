# Sample Policies

Sample policies are designed to be applied to your Kubernetes clusters with minimal changes. To apply these policies to your cluster, install Kyverno and import the policies as follows:

**Install Kyverno**

````sh
kubectl create -f https://github.com/nirmata/kyverno/raw/master/definitions/install.yaml
````
<small>[(installation docs)](../documentation/installation.md)</small>

**Apply Kyverno Policies**

To start applying policies to your cluster, first clone the repo:

````bash
git clone https://github.com/nirmata/kyverno.git
cd kyverno
````

Import best_practices from [here](best_pratices):

````bash
kubectl create -f samples/best_practices
````

Import addition policies from [here](more):

````bash
kubectl create -f samples/more/
````

The policies are mostly validation rules in `audit` mode i.e. your existing workloads will not be impacted, but will be audited for policy complaince.

## Best Practice Policies

These policies are highly recommended.

1. [Disallow root user](DisallowRootUser.md)
2. [Disable privileged containers and disallow privilege escalation](DisablePrivilegedContainers.md)
3. [Disallow new capabilities](DisallowNewCapabilities.md)
4. [Require read-only root filesystem](RequireReadOnlyFS.md)
5. [Disallow use of bind mounts (`hostPath` volumes)](DisallowHostFS.md)
6. [Disallow docker socket bind mount](DisallowDockerSockMount.md)
7. [Disallow `hostNetwork` and `hostPort`](DisallowHostNetworkPort.md)
8. [Disallow `hostPID` and `hostIPC`](DisallowHostPIDIPC.md)
9. [Disallow unknown image registries](DisallowUnknownRegistries.md)
10. [Disallow latest image tag](DisallowLatestTag.md)
11. [Disallow use of default namespace](DisallowDefaultNamespace.md)
12. [Require namespace limits and quotas](RequireNSLimitsQuotas.md)
13. [Require pod resource requests and limits](RequirePodRequestsLimits.md)
14. [Require pod `livenessProbe` and `readinessProbe`](RequirePodProbes.md)
15. [Default deny all ingress traffic](DefaultDenyAllIngress.md)
16. [Disallow Helm Tiller](DisallowHelmTiller.md)
17. [Add `safe-to-evict` for pods with `emptyDir` and `hostPath` volumes](AddSafeToEvict.md)

## Additional Policies

The policies provide additional best practices and are worthy of close consideration. These policies may require workload specific changes. 

18. [Limit use of `NodePort` services](LimitNodePort.md)
19. [Limit automount of Service Account credentials](DisallowAutomountSACredentials.md)
20. [Configure Linux Capabilities](AssignLinuxCapabilities.md)
21. [Limit Kernel parameter access](ConfigureKernelParmeters.md)
22. [Restrict ingress class](KnownIngressClass.md)
