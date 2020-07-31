#!/bin/sh
set -e

pwd=$(pwd)
hash=$(git describe --dirty --always --tags)
#
## Install Kind
curl -Lo $pwd/kind https://kind.sigs.k8s.io/dl/v0.8.1/kind-linux-amd64
chmod a+x $pwd/kind

## Create Kind Cluster
$pwd/kind create cluster
$pwd/kind load docker-image nirmata/kyverno:$hash
$pwd/kind load docker-image nirmata/kyvernopre:$hash

pwd=$(pwd)
cd $pwd/definitions
echo "Installing kustomize"
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
chmod a+x $pwd/definitions/kustomize
echo "Kustomize image edit"
$pwd/definitions/kustomize edit set image nirmata/kyverno:$hash
$pwd/definitions/kustomize edit set image nirmata/kyvernopre:$hash
$pwd/definitions/kustomize build $pwd/definitions/ > $pwd/definitions/install.yaml

