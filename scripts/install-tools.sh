#!/bin/bash

set -euo pipefail

# Tool versions
COSIGN_VERSION=${COSIGN_VERSION:-"v2.4.1"}
CHAINSAW_VERSION=${CHAINSAW_VERSION:-"v0.2.12"}
KUBECTL_VERSION=${KUBECTL_VERSION:-"v1.33.1"}

# Tool installation directory
TOOLS_DIR=${TOOLS_DIR:-"${PWD}/.tools"}
mkdir -p "${TOOLS_DIR}"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "${ARCH}" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: ${ARCH}"
        exit 1
        ;;
esac

case "${OS}" in
    linux|darwin)
        ;;
    *)
        echo "Unsupported OS: ${OS}"
        exit 1
        ;;
esac

echo "Installing tools for ${OS}/${ARCH}..."

# Function to download with retries
download_with_retry() {
    local url="$1"
    local output="$2"
    local max_attempts=3
    
    for i in $(seq 1 $max_attempts); do
        echo "Attempt $i/$max_attempts: Downloading $url"
        if curl -fsSL --connect-timeout 30 --max-time 300 "$url" -o "$output"; then
            echo "Successfully downloaded $output"
            return 0
        else
            echo "Download failed, attempt $i/$max_attempts"
            sleep 5
        fi
    done
    
    echo "Failed to download $url after $max_attempts attempts"
    return 1
}

# Install cosign
install_cosign() {
    local cosign_binary="${TOOLS_DIR}/cosign"
    
    if [[ -f "$cosign_binary" ]]; then
        local current_version
        current_version=$("$cosign_binary" version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || echo "unknown")
        if [[ "$current_version" == "$COSIGN_VERSION" ]]; then
            echo "cosign $COSIGN_VERSION already installed"
            return 0
        fi
    fi
    
    echo "Installing cosign $COSIGN_VERSION..."
    local cosign_url="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}/cosign-${OS}-${ARCH}"
    
    download_with_retry "$cosign_url" "$cosign_binary"
    chmod +x "$cosign_binary"
    
    echo "cosign installed successfully"
    "$cosign_binary" version
}

# Install chainsaw
install_chainsaw() {
    local chainsaw_binary="${TOOLS_DIR}/chainsaw"
    
    if [[ -f "$chainsaw_binary" ]]; then
        local current_version
        current_version=$("$chainsaw_binary" version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 || echo "unknown")
        if [[ "$current_version" == "$CHAINSAW_VERSION" ]]; then
            echo "chainsaw $CHAINSAW_VERSION already installed"
            return 0
        fi
    fi
    
    echo "Installing chainsaw $CHAINSAW_VERSION..."
    
    # Download and extract chainsaw
    local chainsaw_archive="${TOOLS_DIR}/chainsaw.tar.gz"
    local chainsaw_url="https://github.com/kyverno/chainsaw/releases/download/${CHAINSAW_VERSION}/chainsaw_${OS}_${ARCH}.tar.gz"
    
    download_with_retry "$chainsaw_url" "$chainsaw_archive"
    
    # Extract chainsaw binary
    tar -xzf "$chainsaw_archive" -C "$TOOLS_DIR" chainsaw
    rm -f "$chainsaw_archive"
    chmod +x "$chainsaw_binary"
    
    echo "chainsaw installed successfully"
    "$chainsaw_binary" version
}

# Install kubectl
install_kubectl() {
    local kubectl_binary="${TOOLS_DIR}/kubectl"
    
    if [[ -f "$kubectl_binary" ]]; then
        local current_version
        current_version=$("$kubectl_binary" version --client -o yaml 2>/dev/null | grep -oE 'gitVersion: v[0-9]+\.[0-9]+\.[0-9]+' | sed 's/gitVersion: //' || echo "unknown")
        if [[ "$current_version" == "$KUBECTL_VERSION" ]]; then
            echo "kubectl $KUBECTL_VERSION already installed"
            return 0
        fi
    fi
    
    echo "Installing kubectl $KUBECTL_VERSION..."
    
    # Try multiple kubectl download sources
    local kubectl_urls=(
        "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${OS}/${ARCH}/kubectl"
        "https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/${OS}/${ARCH}/kubectl"
    )
    
    local downloaded=false
    for url in "${kubectl_urls[@]}"; do
        if download_with_retry "$url" "$kubectl_binary"; then
            downloaded=true
            break
        fi
    done
    
    if [[ "$downloaded" != "true" ]]; then
        echo "Failed to download kubectl from all sources"
        return 1
    fi
    
    chmod +x "$kubectl_binary"
    
    echo "kubectl installed successfully"
    "$kubectl_binary" version --client
}

# Check if kind cluster is running
check_kind_cluster() {
    local kubectl_binary="${TOOLS_DIR}/kubectl"
    
    if [[ ! -f "$kubectl_binary" ]]; then
        echo "kubectl not found, skipping cluster check"
        return 0
    fi
    
    # Check if kind cluster exists and is running
    if "$kubectl_binary" cluster-info --context kind-kind &>/dev/null; then
        echo "Kind cluster is running"
        return 0
    else
        echo "Warning: Kind cluster is not running or not accessible"
        echo "You may need to start the cluster with: kind create cluster"
        return 1
    fi
}

# Main installation
main() {
    local tools_to_install=("$@")
    
    # If no arguments, install all tools
    if [[ ${#tools_to_install[@]} -eq 0 ]]; then
        tools_to_install=("cosign" "chainsaw" "kubectl")
    fi
    
    echo "Starting tool installation for: ${tools_to_install[*]}..."
    
    for tool in "${tools_to_install[@]}"; do
        case "$tool" in
            cosign)
                install_cosign
                ;;
            chainsaw)
                install_chainsaw
                ;;
            kubectl)
                install_kubectl
                ;;
            *)
                echo "Unknown tool: $tool"
                echo "Supported tools: cosign, chainsaw, kubectl"
                exit 1
                ;;
        esac
    done
    
    echo ""
    echo "Requested tools installed successfully in ${TOOLS_DIR}/"
    echo "Add ${TOOLS_DIR} to your PATH to use these tools:"
    echo "export PATH=\"${TOOLS_DIR}:\$PATH\""
    
    # Optional cluster check
    if [[ "${CHECK_CLUSTER:-false}" == "true" ]]; then
        echo ""
        check_kind_cluster
    fi
}

main "$@" 