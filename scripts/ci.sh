#!/bin/sh
set -e

pwd=$(pwd)
hash=sha-$(git rev-parse --short HEAD)
echo $hash
cd $pwd/definitions
echo "Installing kustomize"
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash
chmod a+x $pwd/definitions/kustomize
echo "Kustomize image edit"
$pwd/definitions/kustomize edit set image kyverno=nirmata/kyverno:$hash
$pwd/definitions/kustomize edit set image kyvernopre=nirmata/kyvernopre:$hash
$pwd/definitions/kustomize build . > $pwd/definitions/install.yaml