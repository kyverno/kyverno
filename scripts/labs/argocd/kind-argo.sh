#!/usr/bin/env bash

set -e

# CONSTANTS

readonly KIND_IMAGE=kindest/node:v1.24.4
readonly NAME=argo

# DELETE CLUSTER

kind delete cluster --name $NAME || true

# CREATE CLUSTER

kind create cluster --name $NAME --image $KIND_IMAGE --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
kubeadmConfigPatches:
  - |-
    kind: ClusterConfiguration
    controllerManager:
      extraArgs:
        bind-address: 0.0.0.0
    etcd:
      local:
        extraArgs:
          listen-metrics-urls: http://0.0.0.0:2381
    scheduler:
      extraArgs:
        bind-address: 0.0.0.0
  - |-
    kind: KubeProxyConfiguration
    metricsBindAddress: 0.0.0.0
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |-
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
  - role: worker
  - role: worker
  - role: worker
EOF

# DEPLOY INGRESS-NGINX

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

sleep 15

kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

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

# CREATE KUBE-PROMETHEUS-STACK APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kube-prometheus-stack
  namespace: argocd
spec:
  destination:
    namespace: monitoring
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: kube-prometheus-stack
    repoURL: https://prometheus-community.github.io/helm-charts
    targetRevision: 41.7.3
    helm:
      values: |
        kubeEtcd:
          service:
            enabled: true
            targetPort: 2381
        defaultRules:
          create: true
        alertmanager:
          alertmanagerSpec:
            routePrefix: /alertmanager
            alertmanagerConfigSelector:
              matchLabels: {}
            alertmanagerConfigNamespaceSelector:
              matchLabels: {}
          ingress:
            enabled: true
            pathType: Prefix
        prometheus:
          prometheusSpec:
            externalUrl: /prometheus
            routePrefix: /prometheus
            ruleSelectorNilUsesHelmValues: false
            serviceMonitorSelectorNilUsesHelmValues: false
            podMonitorSelectorNilUsesHelmValues: false
            probeSelectorNilUsesHelmValues: false
          ingress:
            enabled: true
            pathType: Prefix
        grafana:
          enabled: true
          adminPassword: admin
          sidecar:
            enableUniqueFilenames: true
            dashboards:
              enabled: true
              searchNamespace: ALL
              provider:
                foldersFromFilesStructure: true
            datasources:
              enabled: true
              searchNamespace: ALL
          grafana.ini:
            server:
              root_url: "%(protocol)s://%(domain)s:%(http_port)s/grafana"
              serve_from_sub_path: true
          ingress:
            enabled: true
            path: /grafana
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
EOF

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
    targetRevision: 2.6.1
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

# CREATE KYVERNO-POLICIES APP

kubectl apply -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kyverno-policies
  namespace: argocd
spec:
  destination:
    namespace: kyverno
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: kyverno-policies
    repoURL: https://kyverno.github.io/kyverno
    targetRevision: 2.6.1
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
EOF

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

echo "---------------------------------------------------------------------------------"

ARGOCD_PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)

echo "ARGOCD is running and available at            http://localhost/argocd"
echo "- log in with admin / $ARGOCD_PASSWORD"
echo "POLICY-REPORTER is running and available at   http://localhost/policy-reporter"
echo "PROMETHEUS is running and available at        http://localhost/prometheus"
echo "ALERTMANAGER is running and available at      http://localhost/alertmanager"
echo "GRAFANA is running and available at           http://localhost/grafana"
echo "- log in with admin / admin"
