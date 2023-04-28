## Description

This test has two foreach statements in the same rule.
The first one adds elements in an array and the second one removes elements from the array.

## Expected Behavior

As mutations happen at the rule boundary, the second foreach doesn't see the elements added in the first foreach.
Therefore those elements will not be removed by the second foreach.

## Reference Issue(s)

5661