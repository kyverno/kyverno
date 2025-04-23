# Validation Failure Actions

Kyverno supports different validation failure actions that determine how policy violations are handled. This document explains the available options and how to use them effectively.

## Audit

When `failureAction` is set to `Audit`, policy violations are reported but do not block the request. This is useful for monitoring compliance without enforcing it.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Audit
  rules:
  - name: check-team-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
```

## Enforce

When `failureAction` is set to `Enforce`, policy violations block the request. The request is rejected at the first rule that fails.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-team-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
```

## DeferEnforce

When `failureAction` is set to `DeferEnforce`, policy violations block the request, but only after evaluating all rules. This provides comprehensive feedback about all rule violations at once, avoiding the "whack-a-mole" problem where requests fail one rule at a time.

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-labels
spec:
  validationFailureAction: DeferEnforce
  rules:
  - name: check-team-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
  - name: check-app-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "The label 'app' is required"
      pattern:
        metadata:
          labels:
            app: "?*"
```

With `DeferEnforce`, if a Pod is missing both the 'team' and 'app' labels, the request will be rejected with messages for both rule violations, allowing the user to fix all issues at once before resubmitting.

### Example Scenario

Consider a scenario where you have a policy with multiple validation rules for Pods:

1. Require a 'team' label
2. Require an 'app' label
3. Require resource limits
4. Require security context settings

With the traditional `Enforce` action, if a Pod is missing all of these requirements, the user would experience:

1. Submit Pod → Rejected for missing 'team' label
2. Add 'team' label, resubmit → Rejected for missing 'app' label
3. Add 'app' label, resubmit → Rejected for missing resource limits
4. Add resource limits, resubmit → Rejected for missing security context
5. Add security context, resubmit → Finally accepted

With `DeferEnforce`, the user would experience:

1. Submit Pod → Rejected with all four validation failures listed
2. Fix all issues, resubmit → Accepted

This significantly improves the user experience and reduces the time needed to comply with policies.

## Rule-Level Failure Actions

Failure actions can also be specified at the rule level, which overrides the policy-level setting:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: mixed-failure-actions
spec:
  validationFailureAction: Audit
  rules:
  - name: check-team-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      failureAction: DeferEnforce
      message: "The label 'team' is required"
      pattern:
        metadata:
          labels:
            team: "?*"
  - name: check-app-label
    match:
      resources:
        kinds:
        - Pod
    validate:
      failureAction: Enforce
      message: "The label 'app' is required"
      pattern:
        metadata:
          labels:
            app: "?*"
```

In this example:
- The policy default is `Audit` (report only)
- The 'check-team-label' rule uses `DeferEnforce` (block after evaluating all rules)
- The 'check-app-label' rule uses `Enforce` (block immediately)

## Comparison of Validation Failure Actions

| Feature | Audit | Enforce | DeferEnforce |
|---------|-------|---------|-------------|
| Blocks requests | No | Yes | Yes |
| Reports all violations at once | Yes | No | Yes |
| Suitable for | Monitoring, testing policies | Production enforcement | Production enforcement with better UX |
| User experience | No interruption | May require multiple submissions | Single rejection with complete feedback |

## Best Practices

### When to Use Each Action

- **Audit**: Use during policy development and testing, or for policies that are informational only.
- **Enforce**: Use when immediate rejection is critical, such as for security policies where you want to prevent any further processing of non-compliant resources.
- **DeferEnforce**: Use for most validation policies in production environments where you want to enforce compliance but provide a better user experience.

### Mixing Actions

You can mix different failure actions within a single policy by setting them at the rule level:

- Set critical security rules to `Enforce` for immediate rejection
- Set less critical rules to `DeferEnforce` to provide comprehensive feedback
- Set informational rules to `Audit` to provide guidance without blocking

### Migration Strategy

When introducing new policies, consider this progression:

1. Start with `Audit` to monitor compliance without disrupting workflows
2. Move to `DeferEnforce` once users are familiar with the requirements
3. Use `Enforce` only for critical security policies where immediate rejection is necessary
