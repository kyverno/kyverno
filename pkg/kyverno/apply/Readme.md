Apply Command - Valid Parameter Combinations and their interpretation!

| S.No | policy        | resource         | cluster   | namespace      | interpretation                                                                           |
| ---- |:-------------:| :---------------:| :--------:| :-------------:| :----------------------------------------------------------------------------------------| 
| 1.   | policy.yaml   | -r resource.yaml | false     |                | apply policy from file to the resource from file                                         |
| 2.   | policy.yaml   | -r resourceName  | true      |                | apply policy from file to the resource in cluster                                        |
| 3.   | policy.yaml   |                  | true      |                | apply policy from file to all the resources in cluster                                   |
| 4.   | policy.yaml   | -r resourceName  | true      | -n=namespace   | apply policy from file to the resource in cluster in mentioned namespace                 |
| 5.   | policy.yaml   |                  | true      | -n=namespace   | apply policy from file to all the resources in cluster in mentioned namespace            |