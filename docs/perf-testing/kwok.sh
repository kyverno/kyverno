#!/bin/bash

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# Variables preparation
KWOK_WORK_DIR=$(mktemp -d)
KWOK_REPO=kubernetes-sigs/kwok
KWOK_LATEST_RELEASE=$(curl "https://api.github.com/repos/${KWOK_REPO}/releases/latest" | jq -r '.tag_name')

# Render kustomization yaml
cat <<EOF > "${KWOK_WORK_DIR}/kustomization.yaml"
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: registry.k8s.io/kwok/kwok
    newTag: "${KWOK_LATEST_RELEASE}"
resources:
  - "https://github.com/${KWOK_REPO}/kustomize/kwok?ref=${KWOK_LATEST_RELEASE}"
EOF

kubectl kustomize "${KWOK_WORK_DIR}" > "${KWOK_WORK_DIR}/kwok.yaml"

# create `kwok` deployment 
kubectl apply -f "${KWOK_WORK_DIR}/kwok.yaml"
