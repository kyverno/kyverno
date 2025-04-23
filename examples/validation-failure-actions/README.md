# Validation Failure Actions Examples

This directory contains example policies demonstrating different validation failure actions in Kyverno.

## Pod Security Best Practices

The [pod-security-policy.yaml](pod-security-policy.yaml) example demonstrates a policy that uses `DeferEnforce` to provide comprehensive feedback on all violations at once. This policy enforces several best practices for Pod security:

1. Requiring resource limits for all containers
2. Requiring a security context with runAsNonRoot: true
3. Disallowing privileged containers
4. Requiring specific labels (app, team, environment)

With `DeferEnforce`, if a Pod violates multiple rules, the user will receive feedback about all violations in a single rejection, allowing them to fix all issues at once.

## Mixed Failure Actions

The [mixed-failure-actions.yaml](mixed-failure-actions.yaml) example demonstrates how to mix different validation failure actions within a single policy:

1. `Enforce` for critical security checks (privileged containers)
2. `DeferEnforce` for important requirements (resource limits, runAsNonRoot)
3. `Audit` for recommended practices (labels)

This approach balances security (immediate rejection for critical issues) with user experience (comprehensive feedback for other requirements).

## Testing the Examples

You can apply these policies to your cluster:

```bash
kubectl apply -f pod-security-policy.yaml
kubectl apply -f mixed-failure-actions.yaml
```

Then try to create a Pod that violates multiple rules:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
EOF
```

With the `DeferEnforce` policy, you'll receive feedback about all violations at once.
