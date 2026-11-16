#!/usr/bin/env bash
# Creates a kind cluster with kube-apiserver conditional authorization enabled
# and Kyverno configured as a webhook authorizer.
#
# Run from the repository root. Prerequisites:
#   - kind node image built from Kubernetes v1.38.0-beta.0 (a pre-release):
#       kind build node-image --type release v1.38.0-beta.0 --image kindest/node:v1.38.0-beta.0
#   - Kyverno images built for the current commit: make image-build-all
#
# Environment:
#   KIND_NAME   cluster name (default authz-138)
#   KIND_IMAGE  node image (default kindest/node:v1.38.0-beta.0)
#   AUTHZ_DIR   host directory mounted into the control-plane node
#               (default: a new temporary directory)
#   KIND        kind binary (default: kind on PATH). The node image must be
#               built and loaded by the same kind version: kind v0.31.0 (the
#               Makefile default) cannot load images into a node built by v0.33.0.
set -euo pipefail

KIND_NAME=${KIND_NAME:-authz-138}
KIND_IMAGE=${KIND_IMAGE:-kindest/node:v1.38.0-beta.0}
AUTHZ_DIR=${AUTHZ_DIR:-$(mktemp -d)}
SETUP_DIR=$(cd "$(dirname "$0")" && pwd)
NODE=${KIND_NAME}-control-plane
KUBECTL=(kubectl --context "kind-${KIND_NAME}")
KIND=${KIND:-$(command -v kind)}

echo "AUTHZ_DIR=${AUTHZ_DIR}"
mkdir -p "${AUTHZ_DIR}"
cp "${SETUP_DIR}/authorization-config-bootstrap.yaml" "${AUTHZ_DIR}/authorization-config.yaml"
sed "s#AUTHZ_DIR#${AUTHZ_DIR}#" "${SETUP_DIR}/kind-config.yaml" > "${AUTHZ_DIR}/kind-config.yaml"

if ! "${KIND}" get clusters | grep -qx "${KIND_NAME}"; then
  "${KIND}" create cluster --name "${KIND_NAME}" --image "${KIND_IMAGE}" --config "${AUTHZ_DIR}/kind-config.yaml" --wait 120s
fi

# retry COMMAND...: retries transient network errors (registry pulls, chart downloads).
retry() {
  for i in 1 2 3 4 5 6; do
    if "$@"; then return 0; fi
    echo "attempt ${i} failed: $*" >&2
    sleep 10
  done
  return 1
}

# The kind-load-* targets rebuild the images with ko, which pulls base images.
# Load them one at a time so a retry does not repeat finished loads.
for image in kyverno-init kyverno cleanup-controller reports-controller background-controller readiness-checker cli; do
  retry make "kind-load-${image}" KIND="${KIND}" KIND_NAME="${KIND_NAME}"
done
# The kyverno-api chart does not ship the AuthorizingPolicy CRD yet.
"${KUBECTL[@]}" apply --server-side -f config/crds/policies.kyverno.io/policies.kyverno.io_authorizingpolicies.yaml
kubectl config use-context "kind-${KIND_NAME}"
retry make kind-install-kyverno KIND="${KIND}" KIND_NAME="${KIND_NAME}" \
  EXPLICIT_INSTALL_SETTINGS="--set admissionController.container.extraArgs.enableAuthorizingPolicies=true"
"${KUBECTL[@]}" -n kyverno rollout status deployment kyverno-admission-controller --timeout=300s

# kube-apiserver runs on the host network, so it cannot resolve cluster DNS
# names. Use the service ClusterIP (kube-proxy routes it on the node) and
# verify the serving certificate against the service DNS name.
SVC_IP=$("${KUBECTL[@]}" -n kyverno get service kyverno-svc -o jsonpath='{.spec.clusterIP}')
"${KUBECTL[@]}" -n kyverno get secret kyverno-svc.kyverno.svc.kyverno-tls-ca \
  -o jsonpath='{.data.tls\.crt}' | base64 -d > "${AUTHZ_DIR}/kyverno-ca.crt"

cat > "${AUTHZ_DIR}/kyverno-authz-kubeconfig.yaml" <<EOF
apiVersion: v1
kind: Config
clusters:
  - name: kyverno-sar
    cluster:
      server: https://${SVC_IP}:443/authz/subjectaccessreview
      certificate-authority: /etc/kubernetes/authz/kyverno-ca.crt
      tls-server-name: kyverno-svc.kyverno.svc
  - name: kyverno-conditions
    cluster:
      server: https://${SVC_IP}:443/authz/conditionsreview
      certificate-authority: /etc/kubernetes/authz/kyverno-ca.crt
      tls-server-name: kyverno-svc.kyverno.svc
  # Test-only: sends conditions reviews to the SubjectAccessReview endpoint (HTTP 400).
  - name: kyverno-conditions-wrong-endpoint
    cluster:
      server: https://${SVC_IP}:443/authz/subjectaccessreview
      certificate-authority: /etc/kubernetes/authz/kyverno-ca.crt
      tls-server-name: kyverno-svc.kyverno.svc
  # Test-only: nothing listens on this port (connection refused).
  - name: kyverno-conditions-down
    cluster:
      server: https://127.0.0.1:1/authz/conditionsreview
      certificate-authority: /etc/kubernetes/authz/kyverno-ca.crt
      tls-server-name: kyverno-svc.kyverno.svc
users:
  - name: kube-apiserver
    user: {}
contexts:
  - name: kyverno-sar
    context: {cluster: kyverno-sar, user: kube-apiserver}
  - name: kyverno-conditions
    context: {cluster: kyverno-conditions, user: kube-apiserver}
  - name: kyverno-conditions-wrong-endpoint
    context: {cluster: kyverno-conditions-wrong-endpoint, user: kube-apiserver}
  - name: kyverno-conditions-down
    context: {cluster: kyverno-conditions-down, user: kube-apiserver}
current-context: kyverno-sar
EOF

# Switch kube-apiserver to the Kyverno authorizer chain (reloaded without a restart).
cp "${SETUP_DIR}/authorization-config.yaml" "${AUTHZ_DIR}/authorization-config.yaml.new"
mv "${AUTHZ_DIR}/authorization-config.yaml.new" "${AUTHZ_DIR}/authorization-config.yaml"

# Wait until kube-apiserver asks Kyverno: a probe policy grants a metadata-only read.
cat <<EOF | "${KUBECTL[@]}" apply -f -
apiVersion: policies.kyverno.io/v1alpha1
kind: AuthorizingPolicy
metadata:
  name: setup-probe
spec:
  matchSubjects:
    - matchExpressions:
        - {key: user, operator: In, values: ["chainsaw-setup-probe"]}
  matchResources:
    resourceRules:
      - {apiGroups: [""], apiVersions: ["*"], resources: ["configmaps"], verbs: ["list"], namespaces: ["default"]}
  rules:
    - {name: allow, effect: Allow, expression: "true"}
EOF
for i in $(seq 1 60); do
  if "${KUBECTL[@]}" -n default get configmaps --as chainsaw-setup-probe >/dev/null 2>&1; then
    echo "kube-apiserver uses the Kyverno authorizer (after ${i} attempts)"
    "${KUBECTL[@]}" delete authorizingpolicy setup-probe
    exit 0
  fi
  sleep 5
done
echo "kube-apiserver did not start using the Kyverno authorizer" >&2
docker exec "${NODE}" sh -c 'crictl logs $(crictl ps --name kube-apiserver -q) 2>&1 | grep -i authoriz | tail -20' >&2 || true
exit 1
