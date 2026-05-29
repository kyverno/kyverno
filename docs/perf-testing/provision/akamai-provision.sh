#!/usr/bin/env bash
# Provision two Akamai Dedicated instances for Kyverno performance testing.
#
# Requires: linode-cli authenticated (linode-cli configure), jq, ssh-keygen.
#
# Usage:
#   export LINODE_SSH_KEY="$(cat ~/.ssh/id_ed25519.pub)"
#   ./scripts/provision/akamai-provision.sh
#
# To destroy after testing:
#   ./scripts/provision/akamai-provision.sh teardown

set -euo pipefail

REGION="${REGION:-us-east}"
IMAGE="linode/ubuntu22.04"
CLUSTER_TYPE="g6-dedicated-32"   # 32 vCPU / 64 GB
LOADER_TYPE="g6-dedicated-16"    # 16 vCPU / 32 GB
CLUSTER_LABEL="kyverno-perf-cluster"
LOADER_LABEL="kyverno-perf-loader"
VLAN_LABEL="kyverno-perf-vlan"
CLUSTER_VLAN_IP="10.0.0.10/24"
LOADER_VLAN_IP="10.0.0.11/24"
IDS_FILE="/tmp/kyverno-perf-instance-ids"

if [[ -z "${LINODE_SSH_KEY:-}" ]]; then
  echo "ERROR: set LINODE_SSH_KEY to your public key string" >&2
  exit 1
fi

teardown() {
  if [[ ! -f "$IDS_FILE" ]]; then
    echo "No instance IDs file found at $IDS_FILE" >&2
    exit 1
  fi
  # shellcheck disable=SC1090
  source "$IDS_FILE"
  echo "Deleting cluster instance $CLUSTER_ID ..."
  linode-cli linodes delete "$CLUSTER_ID"
  echo "Deleting loader instance $LOADER_ID ..."
  linode-cli linodes delete "$LOADER_ID"
  rm -f "$IDS_FILE"
  echo "Done."
}

if [[ "${1:-}" == "teardown" ]]; then
  teardown
  exit 0
fi

ROOT_PASS="$(openssl rand -base64 24)Aa1!"

linode_create() {
  local label="$1" type="$2" vlan_ip="$3"
  linode-cli --no-defaults linodes create \
    --type "$type" \
    --region "$REGION" \
    --image "$IMAGE" \
    --label "$label" \
    --root_pass "$ROOT_PASS" \
    --authorized_keys "$LINODE_SSH_KEY" \
    --interfaces '[{"purpose":"public"},{"purpose":"vlan","label":"'"$VLAN_LABEL"'","ipam_address":"'"$vlan_ip"'"}]' \
    --json 2>/dev/null | jq '.[0]'
}

echo "==> Creating cluster instance ($CLUSTER_TYPE) in region $REGION ..."
CLUSTER_JSON=$(linode_create "$CLUSTER_LABEL" "$CLUSTER_TYPE" "$CLUSTER_VLAN_IP")
CLUSTER_ID=$(echo "$CLUSTER_JSON" | jq -r '.id')
CLUSTER_IP=$(echo "$CLUSTER_JSON" | jq -r '.ipv4[0]')
echo "    id=$CLUSTER_ID  public_ip=$CLUSTER_IP"

echo "==> Creating loader instance ($LOADER_TYPE) in region $REGION ..."
LOADER_JSON=$(linode_create "$LOADER_LABEL" "$LOADER_TYPE" "$LOADER_VLAN_IP")

LOADER_ID=$(echo "$LOADER_JSON" | jq -r '.id')
LOADER_IP=$(echo "$LOADER_JSON"  | jq -r '.ipv4[0]')
echo "    id=$LOADER_ID  public_ip=$LOADER_IP"

# Persist IDs for teardown
cat > "$IDS_FILE" <<EOF
CLUSTER_ID=$CLUSTER_ID
LOADER_ID=$LOADER_ID
EOF

echo ""
echo "==> Waiting for instances to become running ..."
for ID in "$CLUSTER_ID" "$LOADER_ID"; do
  for i in $(seq 1 30); do
    STATUS=$(linode-cli --no-defaults linodes view "$ID" --json 2>/dev/null | jq -r '.[0].status')
    [[ "$STATUS" == "running" ]] && break
    echo "    $ID status=$STATUS (attempt $i/30) ..."
    sleep 10
  done
done

echo ""
echo "==> Instances ready."
echo ""
echo "Next steps:"
echo "  1. Bootstrap the cluster node:"
echo "     ssh root@$CLUSTER_IP 'bash -s' < scripts/provision/akamai-setup-cluster.sh"
echo ""
echo "  2. Bootstrap the loader node:"
echo "     ssh root@$LOADER_IP 'bash -s' < scripts/provision/akamai-setup-loader.sh"
echo ""
echo "  3. Copy kubeconfig from cluster to loader:"
echo "     scp root@$CLUSTER_IP:/root/.kube/config /tmp/kyverno-perf-kubeconfig"
echo "     # edit server: from 127.0.0.1 to $CLUSTER_VLAN_IP"
echo "     scp /tmp/kyverno-perf-kubeconfig root@$LOADER_IP:/root/.kube/config"
echo ""
echo "  Cluster public IP : $CLUSTER_IP  (VLAN: 10.0.0.10)"
echo "  Loader  public IP : $LOADER_IP   (VLAN: 10.0.0.11)"
echo ""
echo "  To teardown:  $0 teardown"
