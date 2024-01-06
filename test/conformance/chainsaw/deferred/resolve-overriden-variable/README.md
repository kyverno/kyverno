## Description

This test checks for handling of variable dependencies with the same name:
- the same name is used twice in the rule context
- the same name is also used in a foreach context

## Expected Behavior

The configmap should create fine and contain `one: one` in the data.
