## Description

This test makes sure that Kyverno CLI apply works as expected when an exception matches a pod with a namespace selector in case of cluster mode. (i.e. `--cluster` flag is set)

## Steps

1.  - Create a namespace `ns-1`
1.  - Label the namespace `ns-1` with `kyverno.tess.io/mutateresource=false`
1.  - Create a pod `test-pod` in namespace `ns-1`
1.  - Create a policy that requires pod to run as non-root user.
1.  - Create an exception that matches any pod whose ns selector is `kyverno.tess.io/mutateresource=false`
1.  - Use `kyverno apply` command to apply the policy and the exception in a cluster mode. It is expected to have a `skip` as a result.

Policy and exception are `policies.kyverno.io` (ValidatingPolicy / CEL PolicyException) rather than
the deprecated `kyverno.io/v1` kinds, which `kyverno apply`/`kyverno test` hard-block since #17553.
In `--cluster` mode the CLI's CEL namespace resolver only reads from `--values-file`
(cmd/cli/kubectl-kyverno/variables/variables.go), it does not fetch the live namespace the way the
legacy engine path does, so `values.yaml` supplies `ns-1`'s label for the exception's
`namespaceObject` matchCondition to see.

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/10260
