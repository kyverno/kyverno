#!/usr/bin/env bash

set -e

# CREATE TEMPO APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tempo
  namespace: argocd
spec:
  destination:
    namespace: monitoring
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: tempo
    repoURL: https://grafana.github.io/helm-charts
    targetRevision: 0.16.5
    helm:
      values: |
        tempo:
          searchEnabled: true
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF
