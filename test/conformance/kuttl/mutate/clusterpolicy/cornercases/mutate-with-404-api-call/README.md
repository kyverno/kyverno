## Description

This test checks that the mutate policy does not fail because of 404 in API Call when failure policy is set to `Ignore`.

## Expected Behavior

The failure policy in the policy is set to Ignore and the API Call refers to a non existent URL. Mutation should not happen and error should not be thrown. 

## Reference Issue(s)

https://github.com/kyverno/kyverno/issues/8936
