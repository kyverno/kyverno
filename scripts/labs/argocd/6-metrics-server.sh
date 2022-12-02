#!/usr/bin/env bash

set -e

# CREATE METRICS-SERVER APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: metrics-server
  namespace: argocd
spec:
  destination:
    namespace: kube-system
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: metrics-server
    repoURL: https://charts.bitnami.com/bitnami
    targetRevision: 6.2.2
    helm:
      values: |
        extraArgs:
          - --kubelet-insecure-tls=true
        apiService:
          create: true
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
EOF
