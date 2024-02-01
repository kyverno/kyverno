## Description

This test creates a policy with `matchConditions` and two pods, it then expects a background scan report to be created for the pod in the selected namespace `match-conditions-standard-ns` other than `default`.

## Steps

1.  - Create the testing namespace `match-conditions-standard-ns`
1.  - Create pods in `match-conditions-standard-ns` and `default` namespaces
1.  - Create a cluster policy
    - Assert the policy becomes ready
1.  - Assert a policy report is created for the pod in `match-conditions-standard-ns`
1.  - Assert a policy report is not created for the pod in `default`
