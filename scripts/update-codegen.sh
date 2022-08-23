#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

case "$(uname -s)" in
    Linux*)     linkutil=readlink;;
    Darwin*)    linkutil=greadlink;;
    *)          machine="UNKNOWN:${unameOut}"
esac

# get nirmata root
NIRMATA_DIR=$(dirname ${BASH_SOURCE})/..
NIRMATA_ROOT=$(${linkutil} -f ${NIRMATA_DIR})

# instructions to build project https://github.com/kyverno/kyverno/wiki/Building

# get relative path to code generation script
CODEGEN_PKG="${GOPATH}/src/k8s.io/code-generator"

# get relative path of nirmata
NIRMATA_PKG=${NIRMATA_ROOT#"${GOPATH}/src/"}

# perform code generation
${CODEGEN_PKG}/generate-groups.sh \
    "client,informer,lister" \
    ${NIRMATA_PKG}/pkg/client \
    ${NIRMATA_PKG}/api \
    "kyverno:v1 kyverno:v1beta1 kyverno:v1alpha2 policyreport:v1alpha2"
