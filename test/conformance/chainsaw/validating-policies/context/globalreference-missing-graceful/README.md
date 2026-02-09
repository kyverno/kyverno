## Description

This test verifies that ValidatingPolicies gracefully handle missing GlobalContextEntries instead of failing hard. 

When a ValidatingPolicy uses CEL expressions with `globalContext.Get()` and the referenced GlobalContextEntry doesn't exist or isn't synchronized yet, the policy should continue evaluation with null values rather than blocking admission completely.

## Expected Behavior

1. ValidatingPolicy with missing GlobalContextEntry should be created successfully
2. Pod creation should succeed even when GlobalContextEntry is missing (graceful degradation)
3. When GlobalContextEntry is later created, policies should work normally
4. No admission blocking should occur due to missing GlobalContextEntries

## Related Issues

- Fixes #13337: calls to globalContext.Get do not work when GlobalContextEntry is missing
- Addresses graceful handling of timing dependencies between GlobalContextEntries and ValidatingPolicies

## Test Coverage

- Missing GlobalContextEntry handling
- Null value evaluation in CEL expressions  
- Policy creation with missing dependencies
- Normal operation when dependencies are available 