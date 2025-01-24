## Description

This test makes sure that Kyverno CLI apply works as expected when an exception matches a pod with a namespace selector in case of cluster mode. (i.e. `--cluster` flag is set)

## Steps

1.  - Create a namespace `ns-1`
1.  - Label the namespace `ns-1` with `kyverno.tess.io/mutateresource=false`
1.  - Create a pod `test-pod` in namespace `ns-1`
1.  - Create a policy that requires pod to run as non-root user.
1.  - Create an exception that matches any pod whose ns selector is `kyverno.tess.io/mutateresource=false`
1.  - Use `kyverno apply` command to apply the policy and the exception in a cluster mode. It is expected to have a `skip` as a result.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/10260
