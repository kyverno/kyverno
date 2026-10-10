#!/usr/bin/env bash
set -euo pipefail

# ====== Edit these ======
IMAGE="dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com/dahu/nginx:1.14.1-8.6"
# ========================

echo "[1/4] Checking cosign"
command -v cosign >/dev/null 2>&1 || { echo "Please install the cosign CLI first: https://docs.sigstore.dev/cosign/system_config/installation/"; exit 1; }

echo "[2/4] Generating a cosign key pair (skipped if one already exists)"
if [ ! -f cosign.key ] || [ ! -f cosign.pub ]; then
  cosign generate-key-pair
else
  echo "cosign.key / cosign.pub already exist, skipping"
fi

echo "[3/4] Logging in to ACR and pushing the signature"
# This assumes you have already logged in to the registry with docker login.
# If not, run this first:
# docker login --username=<your-acr-username> dahu-registry-vpc.cn-hangzhou.cr.aliyuncs.com
cosign sign --key cosign.key "${IMAGE}"

echo "[4/4] Verifying the signature (optional)"
cosign verify --key cosign.pub "${IMAGE}"

echo ""
echo "Signing complete. Public key file: cosign.pub"
echo "Paste the contents of cosign.pub into the attestors.cosign.key.data field of verify-acr-image-policy.yaml"
