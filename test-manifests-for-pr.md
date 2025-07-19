# Test Manifests for PR - Remove Erroneous Subresource Warnings

## 1. Policy That Previously Generated Scale Subresource Warning (Issue #9840)

```yaml
# test-scale-warning-policy.yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: deployment-replicas-higher-than-pdb
  annotations:
    policies.kyverno.io/title: Ensure Deployment Replicas Higher Than PodDisruptionBudget
    policies.kyverno.io/category: Other
    policies.kyverno.io/subject: PodDisruptionBudget, Deployment
spec:
  validationFailureAction: Enforce
  background: true
  rules:
  - name: deployment-replicas-greater-minAvailable
    match:
      any:
      - resources:
          kinds:
          - PodDisruptionBudget
    context:
    - name: deploymentreplicas
      apiCall:
        jmesPath: items[?label_match(`{{request.object.spec.selector.matchLabels}}`, spec.template.metadata.labels)].spec.replicas || `[0]`
        urlPath: /apis/apps/v1/namespaces/{{request.namespace}}/deployments 
    preconditions:
      all:
      - key: '{{ regex_match(''^[0-9]+$'', ''{{ request.object.spec.minAvailable || ''''}}'') }}'
        operator: Equals
        value: true
      - key: '{{ deploymentreplicas[0] }}'
        operator: GreaterThan
        value: 1
    validate:
      message: PodDisruption budget minAvailable ({{ request.object.spec.minAvailable }}) cannot be the same or higher than the replica count ({{ deploymentreplicas[0] }})
      deny:
        conditions:
          all:
          - key: '{{ request.object.spec.minAvailable }}'
            operator: GreaterThanOrEquals
            value: '{{ deploymentreplicas[0] }}'
```

## 2. Policy That Previously Generated Status Subresource Warning

```yaml
# test-status-warning-policy.yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: test-status-validation
spec:
  validationFailureAction: Enforce
  background: true
  rules:
  - name: check-pod-status
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: Pod must be running
      pattern:
        status:
          phase: Running
```

## 3. Policy That Previously Generated Both Warnings

```yaml
# test-both-warnings-policy.yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: test-both-subresource-warnings
spec:
  validationFailureAction: Enforce
  rules:
  - name: test-replicas-and-status
    match:
      any:
      - resources:
          kinds:
          - Pod
    validate:
      message: Test both replicas and status usage
      pattern:
        spec:
          replicas: "?*"
        status:
          phase: Running
```

## 4. Test Resources

```yaml
# test-pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: test-pdb
  namespace: default
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: test-app
---
# test-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
spec:
  containers:
  - name: test
    image: nginx
status:
  phase: Running
```

## 5. CLI Test Commands

```bash
# Test that no scale subresource warning is generated
kubectl kyverno apply test-scale-warning-policy.yaml --resource test-pdb.yaml

# Test that no status subresource warning is generated
kubectl kyverno apply test-status-warning-policy.yaml --resource test-pod.yaml

# Test that no warnings are generated for policy using both
kubectl kyverno apply test-both-warnings-policy.yaml --resource test-pod.yaml

# Expected: No subresource warnings should appear for any of these
# Before fix: Would show warnings about scale/status subresources
# After fix: No warnings should appear
```

## 6. Proof of Fix

### Before Fix (Expected Behavior):
- **Scale Warning**: Generated when policy contained "replicas" but didn't match scale subresources
- **Status Warning**: Generated when policy contained "status" but didn't match status subresources
- **Both warnings were often incorrect** due to simplistic string matching logic
- **False positives** occurred when "replicas" or "status" appeared in contexts, API calls, or unrelated fields

### After Fix (Actual Behavior):
- **No subresource warnings generated** for any policies
- **Policies validate successfully** without erroneous warnings
- **All functionality preserved** - only warnings removed
- **Better user experience** - no more confusing false positive warnings

### Rationale for Complete Removal:
- **Knowledge gap reduced**: Modern kubectl has `--subresource` flag
- **False positives outweighed benefits**: Warnings were often incorrect
- **Maintainer consensus**: @Vyom-Yadav recommended complete removal
- **Simpler codebase**: Less complexity, fewer edge cases to handle
