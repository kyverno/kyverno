# Disallow automount of Service Account credentials

Kubernetes automounts default service account credentials in each pod. To restrict access, opt out of automounting credentials by setting `automountServiceAccountToken` to `false`.

## Policy YAML 

[disallow_automountingapicred.yaml](best_practices/disallow_automountingapicred.yaml) 

````yaml
apiVersion : kyverno.io/v1alpha1
kind: ClusterPolicy
metadata:
  name: validate-disallow-automoutingapicred
spec:
  rules:
  - name: disallow-automoutingapicred
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Deny automounting API credentials"
      pattern:
        spec:
          =(serviceAccountName): "*"
          automountServiceAccountToken: false
````



