# Disallow new capabilities

Linux allows defining fine-grained permissions using
capabilities. With Kubernetes, it is possible to add capabilities that escalate the
level of kernel access and allow other potentially dangerous behaviors. This policy 
enforces that containers cannot add new capabilities. Other policies can be used to set 
default capabilities. 

## Policy YAML

[disallow_new_capabilities.yaml](best_practices/disallow_new_capabilities.yaml)

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-new-capabilities
  annotations:
    pod-policies.kyverno.io/autogen-controllers: none
spec:
  validationFailureAction: audit
  rules:
  - name: validate-add-capabilities
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "New capabilities cannot be added"
      pattern:
        spec:
          containers:
            - name: "*"
              =(securityContext):
                =(capabilities):
                  X(add): null

````
