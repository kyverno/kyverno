# ArgoCD lab

This lab sets up the following components:
- A [kind](https://kind.sigs.k8s.io) cluster
- [nginx-ingress](https://github.com/kubernetes/ingress-nginx)
- [ArgoCD](https://argo-cd.readthedocs.io)
- ArgoCD application to deploy [kyverno](https://kyverno.io)
- ArgoCD application to deploy [kyverno-policies](https://artifacthub.io/packages/helm/kyverno/kyverno-policies)
- ArgoCD application to deploy [policy-reporter](https://kyverno.github.io/policy-reporter)

## Install

Run the command below to install the lab:

```console
./kind-argo.sh
```

## Accessing ArgoCD

ArgoCD should be available at http://localhost/argocd.

Login credentials:
- User name: `admin`
- Password: `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d`

## Accessing policy-reporter

policy-reporter should be available at http://localhost/policy-reporter.
