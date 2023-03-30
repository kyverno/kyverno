#!/usr/bin/env bash

set -e

# DEPLOY ARGOCD

helm upgrade --install --wait --timeout 15m --atomic --namespace argocd --create-namespace \
  --repo https://argoproj.github.io/argo-helm argocd argo-cd --values - <<EOF
dex:
  enabled: false
redis:
  enabled: true
redis-ha:
  enabled: false
repoServer:
  serviceAccount:
    create: true
server:
  config:
    resource.exclusions: |
      - apiGroups:
          - kyverno.io
        kinds:
          - AdmissionReport
          - BackgroundScanReport
          - ClusterAdmissionReport
          - ClusterBackgroundScanReport
        clusters:
          - '*'
    resource.compareoptions: |
      ignoreAggregatedRoles: true
      ignoreResourceStatusField: all
    url: http://localhost/argocd
    application.instanceLabelKey: argocd.argoproj.io/instance
  extraArgs:
    - --insecure
    - --rootpath
    - /argocd
  ingress:
    annotations:
      kubernetes.io/ingress.class: nginx
      cert-manager.io/cluster-issuer: ca-issuer
    enabled: true
    paths:
      - /argocd
EOF
