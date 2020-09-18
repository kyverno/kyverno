---
title: Disallow Root User
slug: /disallow-root-user
description: 
weight: 2
---

# Run as non-root user

By default, all processes in a container run as the root user (uid 0). To prevent potential compromise of container hosts, specify a non-root and least privileged user ID when building the container image and require that application containers run as non root users i.e. set `runAsNonRoot` to `true`.

## Additional Information

* [Pod Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)

## Policy YAML 

````yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-root-user
  annotations:
    policies.kyverno.io/category: Security
    policies.kyverno.io/description: By default, processes in a container run as a 
      root user (uid 0). To prevent potential compromise of container hosts, specify a 
      least privileged user ID when building the container image and require that 
      application containers run as non root users.
spec:
  validationFailureAction: audit
  rules:
  - name: validate-runAsNonRoot
    match:
      resources:
        kinds:
        - Pod
    validate:
      message: "Running as root is not allowed. Set runAsNonRoot to true, or use runAsUser"
      anyPattern:
      - spec:
          securityContext:
            runAsNonRoot: true
      - spec:
          securityContext:
            runAsUser: ">0"
      - spec:
          containers:
          - securityContext:
              runAsNonRoot: true
      - spec:
          containers:
          - securityContext:
              runAsUser: ">0"
````

## Install Policy 
```
kubectl apply -f https://raw.githubusercontent.com/nirmata/kyverno/master/samples/best_practices/disallow_root_user.yaml
```

## Test Policy

Create a pod with root user permission

```
kubectl apply -f https://raw.githubusercontent.com/nirmata/kyverno/master/test/resources/deny_runasrootuser.yaml
```