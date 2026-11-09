# AGENTS.md — cmd/cli/kubectl-kyverno/processor

This package implements the core policy execution engine (`PolicyProcessor`) for the Kyverno CLI (`apply` and `test` commands), executing admission evaluations and CEL policy rules against target resources.

## CEL Namespace Resolution (`GetNamespaceResolver`)

CEL policies (ValidatingPolicies, MutatingPolicies, GeneratingPolicies) resolve target namespaces via `celengine.NamespaceResolver` (`type NamespaceResolver = func(string) *corev1.Namespace`).

### Resolution Precedence and Fallback

1. **Local Variables First**: If in-memory variables (`variables.Variables`) define the requested namespace, the local namespace is returned immediately.
2. **Live Cluster Fallback**: When cluster mode is enabled (`cluster == true`) and a cluster client (`dclient.Interface`) is present, unresolved namespaces fall back to querying the cluster (via typed `kubeClient.CoreV1().Namespaces().Get` or dynamic `GetResource`).
3. **Session Caching**:
   - Confirmed namespace hits are cached in memory to avoid repeated cluster API queries.
   - Confirmed missing namespaces (`apierrors.IsNotFound`) are cached as `nil` (negative caching) to prevent redundant lookups for non-existent namespaces.
   - Transient API failures (`err != nil && !apierrors.IsNotFound(err)`) are **not** cached, allowing subsequent evaluations to retry.
4. **Timeouts**: Live cluster calls are bounded by a context timeout (10 seconds) to prevent command hangs if the API server is unreachable.
5. **Processor Memoization**: `PolicyProcessor.getNamespaceResolver()` memoizes the created resolver on the struct field `PolicyProcessor.NamespaceResolver` so subsequent evaluations reuse the exact same resolver instance and cache.
