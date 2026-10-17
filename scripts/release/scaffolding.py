#!/usr/bin/env python3
"""Prepare the pinned Sigstore topology on every supported Kubernetes version."""

import argparse
import hashlib
from pathlib import Path
import subprocess
import urllib.request


COMMIT = "9cbe5a1e1b01b29a875ddb7cdc77027d20a0a2d9"
CHECKSUM = "e1333aa6446cb2023667d3a574ceb314c903b598c2705732e24f33a96943e1d2"
KIND_VERSION = "v0.33.0"
NODE_IMAGES = {
    "v1.33.7": "sha256:d26ef333bdb2cbe9862a0f7c3803ecc7b4303d8cea8e814b481b09949d353040",
    "v1.34.3": "sha256:08497ee19eace7b4b5348db5c6a1591d7752b164530a36f855cb0f2bdcbadd48",
    "v1.35.1": "sha256:05d7bcdefbda08b4e038f644c4df690cdac3fba8b06f8289f30e10026720a1ab",
    "v1.36.1": "sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5",
}
MARKER = "case ${K8S_VERSION} in\n"


def prepare_script(source, version):
    if version not in NODE_IMAGES:
        raise ValueError(f"Missing pinned Sigstore test node image: {version}")
    if hashlib.sha256(source).hexdigest() != CHECKSUM:
        raise ValueError("Sigstore setup script differs from its committed checksum")
    script = source.decode()
    if script.count(MARKER) != 1:
        raise ValueError("Unexpected Sigstore setup script version map")
    script = script.replace('KIND_VERSION="v0.31.0"', f'KIND_VERSION="{KIND_VERSION}"', 1)
    # Upstream's fixed version map stops at 1.35. Keep its topology setup but
    # explicitly select the same immutable node as the full release matrix.
    entry = (
        f'  {version})\n'
        f'    K8S_VERSION="{version[1:]}"\n'
        '    KNATIVE_VERSION="1.19.0"\n'
        f'    KIND_IMAGE="kindest/node:{version}@{NODE_IMAGES[version]}"\n'
        '    ;;\n'
    )
    return script.replace(MARKER, MARKER + entry, 1)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("version", choices=NODE_IMAGES)
    parser.add_argument("--prepare-only", action="store_true")
    args = parser.parse_args()
    url = f"https://raw.githubusercontent.com/sigstore/scaffolding/{COMMIT}/hack/setup-kind.sh"
    with urllib.request.urlopen(url, timeout=60) as response:
        script = prepare_script(response.read(), args.version)
    path = Path(".release-scaffolding/setup-kind.sh")
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(script)
    if not args.prepare_only:
        subprocess.run([
            "bash", str(path), "--registry-url", "registry.local:5000",
            "--cluster-suffix", "cluster.local", "--k8s-version", args.version,
        ], check=True)


if __name__ == "__main__":
    main()
