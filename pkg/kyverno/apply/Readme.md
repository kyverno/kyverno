Apply Command Possibilities 

| parameters         | interpretation           | 
| ------------- |:-------------:| 
| kyverno apply policy.yaml -r resource.yaml     | apply policy from file to the resource from file | 
| kyverno apply policy.yaml -r resource_name --cluster       | apply policy from file to the resource in cluster |
| kyverno apply policy.yaml -r resource_name1 resource_name2 --cluster       | apply policy from file to the resource1 and resource2 in cluster |
| kyverno apply policy.yaml --cluster       | apply policy from file to all the resources in cluster |
| kyverno apply policy_name -r resource_name --cluster       | apply policy from cluster to resource in cluster |
| kyverno apply --cluster       | apply all policies from cluster to all resources in cluster |
| kyverno apply policy.yaml --cluster       | apply policy from file to all the resources in cluster |
| kyverno apply policy_name -r resource.yaml --cluster       | invalid because whenever policy is inside the cluster, resource should be also in the cluster      |