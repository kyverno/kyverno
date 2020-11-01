Apply Command - Valid Parameter Combinations and their interpretation!

| S.No | policy        | resource         | cluster   | namespace      | interpretation                                                                           |
| ---- |:-------------:| :---------------:| :--------:| :-------------:| :----------------------------------------------------------------------------------------| 
| 1.   | policy.yaml   | -r resource.yaml | false     |                | apply policy from file to the resource from file                                         |
| 2.   | policy.yaml   | -r resourceName  | true      |                | apply policy from file to the resource in cluster                                        |
| 3.   | policy.yaml   |                  | true      |                | apply policy from file to all the resources in cluster                                   |
| 4.   | policy.yaml   | -r resourceName  | true      | -n=namespace   | apply policy from file to the resource in cluster in mentioned namespace                 |
| 5.   | policy.yaml   |                  | true      | -n=namespace   | apply policy from file to all the resources in cluster in mentioned namespace            |
| 6.   | policyName    | -r resourceName  | true      |                | apply policy from cluster to the resource in cluster                                     |
| 7.   | policyName    |                  | true      |                | apply polify from cluster to all the resouces in cluster                                 |
| 8.   | policyName    | -r resourceName  | true      | -n=namespace   | apply policy from cluster to the resource in cluster in mentioned namespace              |
| 9.   | policyName    |                  | true      | -n=namespace   | apply polify from cluster to all the resouces in cluster in mentioned namespace          |
| 10.  |               | -r resourceName  | true      |                | applying all policies from the cluster to resouce in cluster                             |
| 11.  |               | -r resourceName  | true      | -n=namespace   | applying all policies from the cluster to resouce in cluster in mentioned namespace      |
| 12.  |               |                  | true      |                | applying all policies from the cluster to all resouces in cluster                        |
| 13.  |               |                  | true      |                | applying all policies from the cluster to all resouces in cluster in mentioned namespace |




