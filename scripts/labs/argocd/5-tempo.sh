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

kubectl create namespace monitoring || true

kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    grafana_datasource: "1"
  name: tempo-datasource
  namespace: monitoring
data:
  tempo-datasource.yaml: |-
    apiVersion: 1
    datasources:
    - name: Tempo
      type: tempo
      access: proxy
      url: "http://tempo.monitoring:3100"
      version: 1
      isDefault: false
EOF
