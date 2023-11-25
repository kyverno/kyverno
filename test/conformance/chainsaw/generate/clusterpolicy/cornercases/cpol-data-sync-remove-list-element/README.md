## Description

This test checks the removal of an array element is synced to the downstream resource correctly.

## Expected Behavior

When the `Egress` is removed from the data generate policy, this change should be synced to the downstream generated resource. The test passes if the `Egress` is removed from the networkpolicy `cpol-data-sync-remove-list-element-ns/default-netpol`, otherwise fails.

## Reference Issue(s)

n/a