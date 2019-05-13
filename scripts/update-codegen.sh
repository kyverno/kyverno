#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

# get nirmata root
NIRMATA_DIR=$(dirname ${BASH_SOURCE})/..
NIRMATA_ROOT=$(greadlink -f ${NIRMATA_DIR})

# get relative path to code generation script
CODEGEN_PKG=${NIRMATA_DIR}/vendor/k8s.io/code-generator

# get relative path of nirmata
NIRMATA_PKG=${NIRMATA_ROOT#"${GOPATH}/src/"}

# perform code generation
${CODEGEN_PKG}/generate-groups.sh \
    "deepcopy,client,informer,lister" \
    ${NIRMATA_PKG}/pkg/client \
    ${NIRMATA_PKG}/pkg/apis \
    policy:v1alpha1
