# ArgoCD lab

This lab sets up the following components:
- A kind cluster
- nginx-ingress
- ArgoCD
- ArgoCD application to deploy kyverno
- ArgoCD application to deploy kyverno-policies

## Accessing ArgoCD

ArgoCD should be available at http://localhost/argocd.

Login credentials:
- User name: `admin`
- Password: `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d`
