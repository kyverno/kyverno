# ArgoCD lab

This lab sets up the following components:
- A [kind](https://kind.sigs.k8s.io) cluster
- [ingress-nginx](https://github.com/kubernetes/ingress-nginx)
- [ArgoCD](https://argo-cd.readthedocs.io)
- ArgoCD application to deploy [kyverno](https://kyverno.io)
- ArgoCD application to deploy [kyverno-policies](https://artifacthub.io/packages/helm/kyverno/kyverno-policies)
- ArgoCD application to deploy [policy-reporter](https://kyverno.github.io/policy-reporter)
- ArgoCD application to deploy [metrics-server](https://github.com/kubernetes-sigs/metrics-server)
- ArgoCD application to deploy [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)

> **Note**: Unfortunately kube-prometheus-stack fails to sync the first time it is deployed hence we need to make it pass by hand.

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

## Accessing prometheus

prometheus should be available at http://localhost/prometheus.

## Accessing alertmanager

alertmanager should be available at http://localhost/alertmanager.

## Accessing grafana

grafana should be available at http://localhost/grafana.

Login credentials:
- User name: `admin`
- Password: `admin`
