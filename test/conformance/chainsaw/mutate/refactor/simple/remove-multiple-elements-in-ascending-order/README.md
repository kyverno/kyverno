## Description

This test removes multiple elements from an array iterating in ascending order.

## Expected Behavior

Removing in ascending order is usually not giving the expected result as removing one element will modify the index on the following elements.
Hence the path to remove following elements are going to point to the wrong index, removing should be done in descending order.
In this case, the we expect volumes at index 0 and 1 to be removed but as we remove volume at index 0 first, removing the volume at index 1 actually removes the volume at index 2 in the original array.

## Reference Issue(s)

5661