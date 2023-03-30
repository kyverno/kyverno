#!/usr/bin/env bash

set -e

# CREATE LOKI APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: loki
  namespace: argocd
spec:
  destination:
    namespace: monitoring
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: loki-stack
    repoURL: https://grafana.github.io/helm-charts
    targetRevision: 2.8.4
    helm:
      values: |
        loki:
          isDefault: false
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF
