# AuthorizingPolicy CLI Test Examples

This directory contains CLI test fixtures for AuthorizingPolicy authorization decisions.

## Files

- `policy.yaml` - Example AuthorizingPolicy with allow/deny rules
- `requests.yaml` - Example SubjectAccessReview requests for testing
- `results.md` - Expected authorization decisions

## Test Cases

### Test 1: Allow Developer Read Access
- User: alice
- Groups: developers, team-backend
- Action: get pods
- Expected: **Allow** (matches allow-developers-read-pods rule)

### Test 2: Deny Delete Access
- User: eve
- Groups: external-users
- Action: delete pods
- Expected: **Deny** (matches deny-delete-pods rule)

### Test 3: System Admin Access
- User: system:admin
- Groups: system:masters
- Action: get non-resource path
- Expected: **No Opinion** (non-resource request, no matching rules)

## CLI Testing

To test these fixtures with the Kyverno CLI:

```bash
# Test policy validation
kubectl-kyverno apply -f policy.yaml

# Test authorization decisions
kubectl-kyverno test --policy policy.yaml --resource requests.yaml
```

## Integration Testing

For full integration testing with a Kyverno cluster:

1. Create the AuthorizingPolicy in your cluster
2. Wait for webhook to be registered
3. Send authorization requests to the webhook endpoint:
   ```bash
   curl -X POST https://kyverno-service/authz/subjectaccessreview \
     -H "Content-Type: application/json" \
     -d @<(kubectl get sar test-sar-allow -o json | kubectl apply -f -)
   ```
4. Verify the authorization decision

## Expected Responses

### Allow Response
```json
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReviewStatus",
  "allowed": true,
  "reason": "Matched allow-developers-read-pods rule"
}
```

### Deny Response
```json
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReviewStatus",
  "allowed": false,
  "denied": true,
  "reason": "Matched deny-delete-pods rule"
}
```

### No Opinion Response
```json
{
  "apiVersion": "authorization.k8s.io/v1",
  "kind": "SubjectAccessReviewStatus",
  "allowed": false,
  "reason": "No matching AuthorizingPolicy rules"
}
```
