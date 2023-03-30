#!/usr/bin/env bash

set -e

# CREATE POLICY-REPORTER APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: policy-reporter
  namespace: argocd
spec:
  destination:
    namespace: kyverno
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: policy-reporter
    repoURL: https://kyverno.github.io/policy-reporter
    targetRevision: 2.13.4
    helm:
      values: |
        ui:
          enabled: true
          ingress:
            annotations:
              nginx.ingress.kubernetes.io/rewrite-target: \$1\$2
              nginx.ingress.kubernetes.io/configuration-snippet: |
                rewrite ^(/policy-reporter)$ \$1/ redirect;
            enabled: true
            hosts:
              - host: ~
                paths:
                  - path: /policy-reporter(/|$)(.*)
                    pathType: Prefix
        kyvernoPlugin:
          enabled: true
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
EOF
