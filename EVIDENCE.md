# Issue #16131 - Policies get mixed

## Root Cause Analysis

**Problem**: Multiple `NamespacedValidatingPolicy` resources with different `objectSelector` or `namespaceSelector` constraints were being incorrectly consolidated into a single `ValidatingWebhookConfiguration`. This caused the webhook selectors to reflect only the last policy processed, breaking the matching logic for all other policies.

**Location**: `pkg/controllers/webhook/validating.go:173-300` (function `buildWebhookRules`)

**Root Cause**: 
- Basic policies (those without fine-grained matching like timeouts, exact match policy, or match conditions) were consolidated into two shared webhooks (`webhookIgnore` and `webhookFail`)
- For each policy, selector assignments **overwrote** previous values instead of handling each policy separately
- Lines 199-214 in original code repeatedly assigned selectors to the shared webhooks
- Rules were appended across policies, but selectors reflected only the last policy

## Solution Design

**Pattern Applied**: Individual Webhook Pattern (similar to fine-grained policies)

**OOP Principles Used**:
- **Encapsulation**: Each webhook now encapsulates its own rules and selectors
- **Separation of Concerns**: Each policy gets its own webhook, eliminating selector collision
- **Consistency**: Matches the pattern already used for fine-grained policies

**Key Changes**:
1. Create individual webhooks for each basic policy (instead of consolidating into two shared webhooks)
2. Each webhook gets its own `NamespaceSelector` and `ObjectSelector` from its policy
3. Each webhook gets a unique name using `generateName()` function with policy name
4. Each webhook gets its own client config path with the policy name

## Implementation Details

### Before (Broken)
```go
// Single webhookIgnore and webhookFail used for ALL basic policies
webhookIgnore := admissionregistrationv1.ValidatingWebhook{...}
webhookFail := admissionregistrationv1.ValidatingWebhook{...}

for _, policy := range basic {
  // Selectors OVERWRITTEN on each iteration
  webhookIgnore.NamespaceSelector = mergeLabelSelectors(...)
  webhookIgnore.ObjectSelector = mergeLabelSelectors(...)
  webhookFail.NamespaceSelector = mergeLabelSelectors(...)
  webhookFail.ObjectSelector = mergeLabelSelectors(...)
  
  // Rules APPENDED
  webhookIgnore.Rules = append(webhookIgnore.Rules, webhookRules...)
}
```

### After (Fixed)
```go
// Separate lists for ignore and fail webhooks
var basicIgnoreList, basicFailList []admissionregistrationv1.ValidatingWebhook

for _, policy := range basic {
  // Create fresh webhook for EACH policy
  webhook := admissionregistrationv1.ValidatingWebhook{...}
  
  // Selectors set ONCE per policy
  webhook.NamespaceSelector = mergeLabelSelectors(...)
  webhook.ObjectSelector = mergeLabelSelectors(...)
  webhook.Rules = webhookRules
  
  // Unique name per policy
  webhook.Name = generateName(name+"-ignore", p)
  
  // Add to appropriate list
  basicIgnoreList = append(basicIgnoreList, webhook)
}
```

## Test Coverage

Added comprehensive test case: `TestBuildWebhookRules_ValidatingPolicy/Multiple_Policies_with_Different_Selectors_(Issue_#16131)`

**Test Scenario**:
- Two `ValidatingPolicy` resources:
  1. `deny-app-deletion-by-label` - No object selector
  2. `deny-intermediate-app-deletion` - Has `objectSelector: definer.myorg.de/type=intermediate`
- Both target `argoproj.io/v1alpha1` Applications on DELETE

**Expected Behavior** (now validated):
- Two separate webhooks generated
- First webhook has no object selector
- Second webhook has the specific object selector
- Each webhook has unique name and client config path

## Testing Results

```
=== RUN   TestBuildWebhookRules_ValidatingPolicy
=== RUN   TestBuildWebhookRules_ValidatingPolicy/Single_Ignore_Policy
=== RUN   TestBuildWebhookRules_ValidatingPolicy/Single_Fail_Policy
=== RUN   TestBuildWebhookRules_ValidatingPolicy/Fine-Grained_Ignore_Policy
=== RUN   TestBuildWebhookRules_ValidatingPolicy/Fine-Grained_Fail_Policy
=== RUN   TestBuildWebhookRules_ValidatingPolicy/Multiple_Policies_with_Different_Selectors_(Issue_#16131)
--- PASS: TestBuildWebhookRules_ValidatingPolicy (0.01s)

=== RUN   TestBuildWebhookRules_ImageValidatingPolicy
=== RUN   TestBuildWebhookRules_ImageValidatingPolicy/Autogen_Single_Ignore_Policy
=== RUN   TestBuildWebhookRules_ImageValidatingPolicy/Autogen_Fine-grained_Ignore_Policy
--- PASS: TestBuildWebhookRules_ImageValidatingPolicy (0.00s)

✓ All webhook tests pass
✓ New test case validates the fix
```

## Before/After Behavior

### Before (Issue)
User creates two NamespacedValidatingPolicy:
1. `deny-app-deletion-by-label` - Matches all argo apps, denies deletion if labeled
2. `deny-intermediate-app-deletion` - Matches only intermediate-labeled argo apps, denies deletion

Result:
- **Single webhook generated** with mixed rules
- Webhook's `objectSelector` reflects ONLY the last policy
- First policy's rules are present but won't match correctly due to wrong selector
- Application that should be protected is not protected

### After (Fixed)
Result:
- **Two separate webhooks generated**
- First webhook: no object selector, applies to all apps (policy 1)
- Second webhook: object selector for intermediate, applies to intermediate apps (policy 2)
- Each policy works correctly with its own selector
- Application protection works as intended

## Code Quality

✓ Minimal changes - only necessary logic restructured
✓ No breaking changes to public APIs
✓ Maintains backward compatibility
✓ Follows existing patterns (matches fine-grained policy approach)
✓ Self-documenting code with clear variable names
✓ All existing tests pass after update
✓ New test covers the issue

## Files Modified

1. `pkg/controllers/webhook/validating.go` - Fixed webhook consolidation logic
2. `pkg/controllers/webhook/validating_test.go` - Added test case and updated existing test expectations
