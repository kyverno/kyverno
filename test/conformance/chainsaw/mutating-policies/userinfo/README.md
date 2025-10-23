## Description

This test verifies that CEL mutating policies can access user information (`request.userInfo`) during policy evaluation. This addresses the fix for TODO comments in the CEL mutating policies engine where userInfo was not being properly passed to admission.NewAttributesRecord() calls.

## Background

Previously, CEL mutating policies had TODO comments where user information should be passed to admission evaluation:

```go
attr := admission.NewAttributesRecord(
    // ... other parameters ...
    nil,  // ❌ TODO: Missing userInfo parameter
)
```

This meant that CEL mutating policies couldn't access user context (username, UID, groups) during policy evaluation, limiting their functionality.

## Test Steps

1. Create a test namespace
2. Create a CEL mutating policy that:
   - Checks if `request.userInfo.username` exists and is not empty
   - Adds labels including the username to pods if the condition is met
3. Wait for the policy to become ready
4. Create a test pod
5. Verify that the pod was mutated with labels containing the user information

## Expected Behavior

The test pod should be mutated to include:
- `mutated-by: kyverno` label
- `user: <username>` label (where username comes from `request.userInfo.username`)

This confirms that user information is properly accessible in CEL expressions for mutating policies.
