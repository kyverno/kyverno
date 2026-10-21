#!/usr/bin/env bash
# Bootstrap the cluster node: Docker, kubectl, kind, helm, kwok, Prometheus.
# Run as root on the Akamai Dedicated instance.
#
# Usage (from your local machine):
#   ssh root@<cluster-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-cluster.sh
#
# Override Kyverno Helm chart version:
#   KYVERNO_VERSION=3.9.0 ssh root@<cluster-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-cluster.sh

set -euo pipefail

KIND_VERSION="v0.27.0"
KUBECTL_VERSION="v1.32.3"
HELM_VERSION="v3.17.3"
KWOK_VERSION="v0.7.0"
KIND_IMAGE="kindest/node:v1.32.3"
KYVERNO_VERSION="${KYVERNO_VERSION:-3.8.1}"   # app v1.18.1; override via env for newer releases

echo "==> Installing system packages ..."
apt-get update -qq
apt-get install -y -qq curl ca-certificates git jq apt-transport-https gnupg2

echo "==> Installing Docker ..."
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker

echo "==> Installing kubectl $KUBECTL_VERSION ..."
curl -fsSLo /usr/local/bin/kubectl \
  "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
chmod +x /usr/local/bin/kubectl

echo "==> Installing kind $KIND_VERSION ..."
curl -fsSLo /usr/local/bin/kind \
  "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-amd64"
chmod +x /usr/local/bin/kind

echo "==> Installing helm $HELM_VERSION ..."
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | \
  DESIRED_VERSION="$HELM_VERSION" bash

echo "==> Creating KinD cluster ..."
cat <<EOF | kind create cluster --name kind --image "$KIND_IMAGE" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker
- role: worker
EOF

echo "==> Installing Prometheus stack ..."
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update
helm upgrade --install --wait --timeout 15m \
  --namespace monitoring --create-namespace \
  kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --set prometheus.prometheusSpec.enableRemoteWriteReceiver=true \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set kubeEtcd.service.targetPort=2382

echo "==> Installing KWOK $KWOK_VERSION ..."
kubectl apply -f "https://github.com/kubernetes-sigs/kwok/releases/download/${KWOK_VERSION}/kwok.yaml"
kubectl apply -f "https://github.com/kubernetes-sigs/kwok/releases/download/${KWOK_VERSION}/stage-fast.yaml"
kubectl apply -f "https://github.com/kubernetes-sigs/kwok/releases/download/${KWOK_VERSION}/metrics-usage.yaml"

echo "==> Creating KWOK node ..."
kubectl create -f - <<EOF
apiVersion: v1
kind: Node
metadata:
  annotations:
    node.alpha.kubernetes.io/ttl: "0"
    kwok.x-k8s.io/node: fake
  labels:
    beta.kubernetes.io/arch: amd64
    beta.kubernetes.io/os: linux
    kubernetes.io/arch: amd64
    kubernetes.io/os: linux
    kubernetes.io/role: agent
    node-role.kubernetes.io/agent: ""
    type: kwok
  generateName: kwok-node-
spec:
  taints:
  - effect: NoSchedule
    key: kwok.x-k8s.io/node
    value: fake
status:
  allocatable:
    cpu: "32"
    memory: 256Gi
    pods: "1000"
  capacity:
    cpu: "32"
    memory: 256Gi
    pods: "1000"
  nodeInfo:
    architecture: amd64
    bootID: ""
    containerRuntimeVersion: ""
    kernelVersion: ""
    kubeProxyVersion: fake
    kubeletVersion: fake
    machineID: ""
    operatingSystem: linux
    osImage: ""
    systemUUID: ""
  phase: Running
EOF

echo "==> Creating test namespace ..."
kubectl create ns testns 2>/dev/null || true

echo ""
echo "==> Cluster setup complete."
echo "    kubeconfig: /root/.kube/config"
echo "    To expose Prometheus on loader: port-forward or use NodePort"
echo ""
echo "    Next: copy kubeconfig to loader node and edit server IP to VLAN address (10.0.0.10)"
