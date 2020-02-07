<small>*[documentation](/README.md#documentation) / Policy Violations*</small>

# Policy Violations

Policy Violations are created to:
1. Report resources that do not comply with validation rules with `validationFailureAction` set to `audit`.
2. Report existing resources (i.e. resources created before the policy was created) that do not comply with validation or mutation rules.

Policy Violation objects are created in the resource namespace. Policy Violation resources are automatically removed when the resource is updated to comply with the policy rule, or when the policy rule is deleted. 

You can view all existing policy violations as shown below:

````
λ kubectl get polv --all-namespaces
NAMESPACE   NAME                        POLICY                RESOURCEKIND   RESOURCENAME                  AGE
default     disallow-root-user-56j4t    disallow-root-user    Deployment     nginx-deployment              5m7s
default     validation-example2-7snmh   validation-example2   Deployment     nginx-deployment              5m7s
docker      disallow-root-user-2kl4m    disallow-root-user    Pod            compose-api-dbbf7c5db-kpnvk   43m
docker      disallow-root-user-hfxzn    disallow-root-user    Pod            compose-7b7c5cbbcc-xj8f6      43m
docker      disallow-root-user-s5rjp    disallow-root-user    Deployment     compose                       43m
docker      disallow-root-user-w58kp    disallow-root-user    Deployment     compose-api                   43m
docker      validation-example2-dgj9j   validation-example2   Deployment     compose                       5m28s
docker      validation-example2-gzfdf   validation-example2   Deployment     compose-api                   5m27s
````

# Cluster Policy Violations

Cluster Policy Violations are like Policy Violations but created for cluster-wide resources.
