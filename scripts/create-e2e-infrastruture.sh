#!/bin/sh
set -e

pwd=$(pwd)
hash=$(git describe --match "[0-9].[0-9]-dev*")
#
## Install Kind
curl -Lo "$pwd"/kind https://kind.sigs.k8s.io/dl/v0.11.0/kind-linux-amd64
chmod a+x "$pwd"/kind

## Create Kind Cluster
if [ -z "${KIND_IMAGE}" ]; then
    "$pwd"/kind create cluster
else
    "$pwd"/kind create cluster --image="${KIND_IMAGE}"
fi

"$pwd"/kind load docker-image ghcr.io/kyverno/kyverno:"$hash"
"$pwd"/kind load docker-image ghcr.io/kyverno/kyvernopre:"$hash"

pwd=$(pwd)
cd "$pwd"/config
echo "Installing kustomize"
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
kustomize edit set image ghcr.io/kyverno/kyverno:"$hash"
kustomize edit set image ghcr.io/kyverno/kyvernopre:"$hash"
kustomize build "$pwd"/config/ -o "$pwd"/config/install.yaml
