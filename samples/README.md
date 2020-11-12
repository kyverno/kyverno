# Sample Policies

Sample policies are designed to be applied to your Kubernetes clusters with minimal changes.

The policies are mostly validation rules in `audit` mode i.e. your existing workloads will not be impacted, but will be audited for policy complaince.

## Best Practice Policies

These policies are highly recommended.

1. [Disallow root user](DisallowRootUser.md)
1. [Disallow privileged containers](DisallowPrivilegedContainers.md)
1. [Disallow new capabilities](DisallowNewCapabilities.md)
1. [Disallow kernel parameter changes](DisallowSysctls.md)
1. [Disallow use of bind mounts (`hostPath` volumes)](DisallowBindMounts.md)
1. [Disallow docker socket bind mount](DisallowDockerSockMount.md)
1. [Disallow `hostNetwork` and `hostPort`](DisallowHostNetworkPort.md)
1. [Disallow `hostPID` and `hostIPC`](DisallowHostPIDIPC.md)
1. [Disallow use of default namespace](DisallowDefaultNamespace.md)
1. [Disallow latest image tag](DisallowLatestTag.md)
1. [Disallow Helm Tiller](DisallowHelmTiller.md)
1. [Require read-only root filesystem](RequireReadOnlyRootFS.md)
1. [Require pod resource requests and limits](RequirePodRequestsLimits.md)
1. [Require pod `livenessProbe` and `readinessProbe`](RequirePodProbes.md)
1. [Add default network policy](AddDefaultNetworkPolicy.md)
1. [Add namespace quotas](AddNamespaceQuotas.md)
1. [Add `safe-to-evict` for pods with `emptyDir` and `hostPath` volumes](AddSafeToEvict.md)

## Additional Policies

These policies provide additional best practices and are worthy of close consideration. These policies may require specific changes for your workloads and environments.

1. [Restrict image registries](RestrictImageRegistries.md)
1. [Restrict `NodePort` services](RestrictNodePort.md)
1. [Restrict auto-mount of service account credentials](RestrictAutomountSAToken.md)
1. [Restrict ingress classes](RestrictIngressClasses.md)
1. [Restrict User Group](CheckUserGroup.md)
1. [Require pods are labeled](RequireLabels.md)
1. [Require pods have certain labels](RequireCertainLabels.md)
1. [Require Deployments have multiple replicas](RequireDeploymentsHaveReplicas.md)

## Applying the sample policies

To apply these policies to your cluster, install Kyverno and import the policies as follows:

### Install Kyverno**

````sh
kubectl create -f https://raw.githubusercontent.com/kyverno/kyverno/main/definitions/release/install.yaml
````

<small>[(installation docs)](../documentation/installation.md)</small>

### Apply Kyverno Policies**

To start applying policies to your cluster, first clone the repo:

````bash
git clone https://github.com/kyverno/kyverno.git
cd kyverno
````

Import best practices from [here](best_pratices):

````bash
kubectl create -f samples/best_practices
````

Import additional policies from [here](more):

````bash
kubectl create -f samples/more/
````
