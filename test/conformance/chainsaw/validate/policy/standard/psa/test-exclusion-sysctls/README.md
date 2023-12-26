## Description

This test ensures the PSS checks with the new advanced support on exclusions are applied to the resources successfully.

## Expected Behavior

Two pods (`good-pod` & `excluded-pod`) should be created as it follows the baseline:latest `Sysctls` PSS check and one pod (`bad-pod`) should not be created as it violate the baseline:latest `Sysctls` PSS check.
