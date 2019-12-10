# Sample Policies

Sample policies are designed to be applied to your Kubernetes clusters with minimal changes. 

The policies are mostly validation rules in `audit` mode i.e. your existing workloads will not be impacted, but will be audited for policy complaince.

## Best Practice Policies

These policies are highly recommended.

1. [Disallow root user](DisallowRootUser.md)
2. [Disallow privileged containers](DisallowPrivilegedContainers.md)
3. [Disallow new capabilities](DisallowNewCapabilities.md)
4. [Disallow kernel parameter changes](DisallowSysctls.md)
5. [Disallow use of bind mounts (`hostPath` volumes)](DisallowBindMounts.md)
6. [Disallow docker socket bind mount](DisallowDockerSockMount.md)
7. [Disallow `hostNetwork` and `hostPort`](DisallowHostNetworkPort.md)
8. [Disallow `hostPID` and `hostIPC`](DisallowHostPIDIPC.md)
9. [Disallow use of default namespace](DisallowDefaultNamespace.md)
10. [Disallow latest image tag](DisallowLatestTag.md)
11. [Disallow Helm Tiller](DisallowHelmTiller.md)
12. [Require read-only root filesystem](RequireReadOnlyRootFS.md)
13. [Require pod resource requests and limits](RequirePodRequestsLimits.md)
14. [Require pod `livenessProbe` and `readinessProbe`](RequirePodProbes.md)
15. [Add default network policy](AddDefaultNetworkPolicy.md)
16. [Add namespace quotas](AddNamespaceQuotas.md)
17. [Add `safe-to-evict` for pods with `emptyDir` and `hostPath` volumes](AddSafeToEvict.md)

## Additional Policies

These policies provide additional best practices and are worthy of close consideration. These policies may require specific changes for your workloads and environments. 

17. [Restrict image registries](RestrictImageRegistries.md)
18. [Restrict `NodePort` services](RestrictNodePort.md)
19. [Restrict auto-mount of service account credentials](RestrictAutomountSAToken.md)
20. [Restrict ingress classes](RestrictIngressClasses.md)
21. [Restrict User Group](CheckUserGroup.md)

## Applying the sample policies

To apply these policies to your cluster, install Kyverno and import the policies as follows:

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

