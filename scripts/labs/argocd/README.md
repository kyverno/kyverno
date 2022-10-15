# ArgoCD lab

This lab sets up the following components:
- A kind cluster
- nginx-ingress
- ArgoCD
- ArgoCD application to deploy kyverno
- ArgoCD application to deploy kyverno-policies
- ArgoCD application to deploy policy-reporter

## Install

Run the command below to install the lab:

```shell
./kind-argo.sh
```

## Accessing ArgoCD

ArgoCD should be available at http://localhost/argocd.

Login credentials:
- User name: `admin`
- Password: `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d`

## Accessing policy-reporter

policy-reporter should be available at http://localhost/policy-reporter.
