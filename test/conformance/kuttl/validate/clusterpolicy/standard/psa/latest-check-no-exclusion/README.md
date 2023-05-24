## Description

This test ensures the PSS checks with the latest version, without exclusions, are applied to the resources successfully.

## Expected Behavior

The two pods should not be created as it violate the baseline:latest `seccomp` PSS check.

## Reference Issue(s)
https://github.com/kyverno/kyverno/issues/7260