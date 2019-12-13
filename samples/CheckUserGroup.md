# Check userID, groupIP & fsgroup

All processes inside the pod can be made to run with specific user and groupID by setting `runAsUser` and `runAsGroup` respectively. `fsGroup` can be specified to make sure any file created in the volume with have the specified groupID. These options can be used to validate the IDs used for user and group.

## Policy YAML 

[policy_validate_user_group_fsgroup_id.yaml](more/policy_validate_user_group_fsgroup_id.yaml)

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: validate-userid-groupid-fsgroup
spec:
  rules:
  - name: validate-userid
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "User ID should be 1000"
      pattern:
        spec:
          securityContext:
            runAsUser: '1000'
  - name: validate-groupid
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Group ID should be 3000"
      pattern:
        spec:
          securityContext:
            runAsGroup: '3000'
  - name: validate-fsgroup
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "fsgroup should be 2000"
      pattern:
        spec:
          securityContext:
            fsGroup: '2000'
# Alls processes inside the pod can be made to run with specific user and groupID by setting runAsUser and runAsGroup respectively.
# fsGroup can be specified to make sure any file created in the volume with have the specified groupID.
# The above parameters can also be used in a validate policy to restrict user & group IDs.
````

