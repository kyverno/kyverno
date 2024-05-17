## Description

This test mainly verifies that the operator AllNotIn does not work properly.

## Expected Behavior

1. The clusterpolicy is created correctly.
2. Failed to create resources in test-validate namespace because the deployment lacks of label.
3. Successfully created deployment in default because 'def*' is within the value of AllNotIn.

## Reference Issue(s)

5617
