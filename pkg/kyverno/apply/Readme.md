Apply Command Possibilities 

| S.No | policy        | resource         | cluster        | namespace      | interpretation                                   |
| ---- |:-------------:| :---------------:| :-------------:| :-------------:| :-----------------------------------------------:| 
| 1.   | policy.yaml   | -r resource.yaml |                |                | apply policy from file to the resource from file |
| 2.   | policy.yaml   | -r resourceName  | --cluster      |                | apply policy from file to the resource in cluster|
| 3.   | policy.yaml   |                  | --cluster      |                | apply policy from file to all the resources in cluster|
| 4.   | policy.yaml   | -r resourceName  | --cluster      | -n=namespace   | apply policy from file to the resource in cluster in mentioned namespace |
| 5.   | policy.yaml   |                  | --cluster      | -n=namespace   | apply policy from file to all the resources in cluster in mentioned namespace |
| 6.   | policyName    | -r resource.yaml | --cluster      |                | invalid condition as policy cannot be in cluster when applying to resource outside cluster |
| 7.   | policyName    | -r resource.yaml | --cluster      | -n=namespace   | invalid condition as policy cannot be in cluster when applying to resource outside cluster |
| 8.   | policyName    | -r resourceName  | --cluster      |                | apply policy from cluster to the resource in cluster |
| 9.   | policyName    |                  | --cluster      |                | apply polify from cluster to all the resouces in cluster |
| 10.  | policyName    | -r resourceName  | --cluster      | -n=namespace   | apply policy from cluster to the resource in cluster in mentioned namespace |
| 11.  | policyName    |                  | --cluster      | -n=namespace   | apply polify from cluster to all the resouces in cluster in mentioned namespace |
| 12.  |               | -r resource.yaml | --cluster      |                | invalid condition as policy cannot be in cluster when applying to resource outside cluster |
| 13.  |               | -r resource.yaml | --cluster      | -n=namespace   | invalid condition as policy cannot be in cluster when applying to resource outside cluster |
| 14.  |               | -r resourceName  | --cluster      |                | applying all policies from the cluster to resouce in cluster |
| 15.  |               | -r resourceName  | --cluster      | -n=namespace   | applying all policies from the cluster to resouce in cluster in mentioned namespace |
| 16.  |               |                  | --cluster      |                | applying all policies from the cluster to all resouces in cluster |
| 16.  |               |                  | --cluster      |                | applying all policies from the cluster to all resouces in cluster in mentioned namespace |




