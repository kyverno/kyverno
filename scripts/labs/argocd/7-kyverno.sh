#!/usr/bin/env bash

set -e

# CREATE KYVERNO APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kyverno
  namespace: argocd
spec:
  destination:
    namespace: kyverno
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: kyverno
    repoURL: https://kyverno.github.io/kyverno
    targetRevision: 2.6.2
    helm:
      values: |
        serviceMonitor:
          enabled: true
        initContainer:
          extraArgs:
            - --loggingFormat=json
        extraArgs:
          - --loggingFormat=json
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
EOF
