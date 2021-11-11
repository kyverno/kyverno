#!/usr/bin/env bash

# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

GOPATH_BIN="$(go env GOPATH)/bin/"
MINIMUM_KUSTOMIZE_VERSION=3.1.0
goarch="$(go env GOARCH)"
goos="$(go env GOOS)"

# Ensure the kustomize tool exists and is a viable version, or installs it
verify_kustomize_version() {

  # If kustomize is not available on the path, get it
  if ! [ -x "$(command -v kustomize)" ]; then
    if [ "$goos" == "linux" ] || [ "$goos" == "darwin" ]; then
      echo 'kustomize not found, installing'
      if ! [ -d "${GOPATH_BIN}" ]; then
        mkdir -p "${GOPATH_BIN}"
      fi
      curl -sLo "${GOPATH_BIN}/kustomize" "https://github.com/kubernetes-sigs/kustomize/releases/download/v${MINIMUM_KUSTOMIZE_VERSION}/kustomize_${MINIMUM_KUSTOMIZE_VERSION}_${goos}_${goarch}"
      chmod +x "${GOPATH_BIN}/kustomize"
    else
      echo "Missing required binary in path: kustomize"
      return 2
    fi
  fi

  local kustomize_version
  kustomize_version=$(kustomize version)
  if [[ "${MINIMUM_KUSTOMIZE_VERSION}" != $(echo -e "${MINIMUM_KUSTOMIZE_VERSION}\n${kustomize_version}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) ]]; then
    cat <<EOF
Detected kustomize version: ${kustomize_version}.
Requires ${MINIMUM_KUSTOMIZE_VERSION} or greater.
Please install ${MINIMUM_KUSTOMIZE_VERSION} or later.
EOF
    return 2
  fi
}

verify_kustomize_version
