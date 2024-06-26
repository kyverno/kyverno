## Description

This test validates the use of variables in validate.cel subrule.

This test creates the following:
1. Two namespaces: `production-ns` and `staging-ns`
2. A policy that enforces that all containers of a deployment has the image repo match the environment label of its namespace. Except for "exempt" deployments, or any containers that do not belong to the "example.com" organization For example, if the namespace has a label of {"environment": "staging"}, all container images must be either staging.example.com/* or do not contain "example.com" at all, unless the deployment has {"exempt": "true"} label.
3. Six deployments.

## Expected Behavior

The following deployments is blocked:
1. `deployment-fail-01`: It intended to be created in namespace `production-ns` but its container image is `staging.example.com/nginx` which violates the validation rule.
2. `deployment-fail-02`: It intended to be created in namespace `staging-ns` but its container image is `example.com/nginx` which violates the validation rule.
3. `deployment-fail-03`: It intended to be created in namespace `staging-ns` and it has a label of `exempt: "false"` but its container image is `example.com/nginx` which violates the validation rule.

The following deployments is created:
1. `deployment-pass-01`, It is created in namespace `production-ns` and its container image is `prod.example.com/nginx`.
2. `deployment-pass-02`, It is created in namespace `staging-ns` and its container image is `staging.example.com/nginx`.
3. `deployment-pass-03`, It is created in namespace `staging-ns` and its container image is `example.com/nginx` but it has a label of `exempt: "true"` so it passes the validation rule.
