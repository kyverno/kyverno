#!/usr/bin/env bash
# Bootstrap the loader node: k6 with xk6-kubernetes extension.
# Run as root on the Akamai Dedicated instance.
#
# Usage (from your local machine):
#   ssh root@<loader-ip> 'bash -s' < docs/perf-testing/provision/akamai-setup-loader.sh

set -euo pipefail

GO_VERSION="1.23.4"
XK6_K8S_VERSION="v0.9.0"

echo "==> Installing system packages ..."
apt-get update -qq
apt-get install -y -qq curl ca-certificates git jq

echo "==> Installing Go $GO_VERSION ..."
curl -fsSLo /tmp/go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"
tar -C /usr/local -xzf /tmp/go.tar.gz
rm /tmp/go.tar.gz
ln -sf /usr/local/go/bin/go /usr/local/bin/go

echo "==> Installing kubectl ..."
curl -fsSLo /usr/local/bin/kubectl \
  "https://dl.k8s.io/release/v1.32.3/bin/linux/amd64/kubectl"
chmod +x /usr/local/bin/kubectl
mkdir -p /root/.kube

echo "==> Building k6 with xk6-kubernetes $XK6_K8S_VERSION ..."
export GOPATH="/root/go"
export PATH="$PATH:/usr/local/go/bin:$GOPATH/bin"
go install go.k6.io/xk6/cmd/xk6@latest
"$GOPATH/bin/xk6" build \
  --with "github.com/grafana/xk6-kubernetes@${XK6_K8S_VERSION}" \
  --output /usr/local/bin/k6

echo ""
echo "==> Loader setup complete."
echo "    k6 binary: /usr/local/bin/k6"
echo ""
echo "    Expected next steps:"
echo "      1. Copy kubeconfig from cluster node to /root/.kube/config"
echo "         (edit server to use VLAN IP 10.0.0.10:6443 if needed)"
echo "      2. Clone/copy the kyverno repo and cd into it"
echo "      3. Run: k6 run docs/perf-testing/v1.18.1/scripts/k6/vpol-script.js --vus 10 --iterations 100"
