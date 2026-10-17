#!/usr/bin/env python3
"""Build retained release candidates. This program cannot publish a release."""

import gzip
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import zipfile

from release import (
    IMAGES, IMAGE_VALUES, chart_image_references, output, prepare, read, run, seal,
    sha256, verify_packages, write_json,
)


def archive_cli(binary, name, windows):
    destination = Path(".release/assets") / name
    if windows:
        with zipfile.ZipFile(destination, "w", zipfile.ZIP_DEFLATED) as archive:
            archive.write(binary, "kyverno.exe")
            archive.write("LICENSE", "LICENSE")
    else:
        with destination.open("wb") as stream:
            with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0) as compressed:
                with tarfile.open(fileobj=compressed, mode="w") as archive:
                    archive.add(binary, arcname="kyverno")
                    archive.add("LICENSE", arcname="LICENSE")


def stage_manifest(metadata):
    with Path(".release/manifest.tar.gz").open("wb") as stream:
        with gzip.GzipFile(filename="", mode="wb", fileobj=stream, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode="w") as archive:
                archive.add(".release/install.yaml", arcname="install.yaml")
    created = output("git", "show", "-s", "--format=%cI", metadata["sha"])
    write_json(".release/manifest-config.json", {
        "created": created, "architecture": "", "os": "",
        "rootfs": {"type": "layers", "diff_ids": []},
    })
    run("oras", "push", "--oci-layout", f"manifests:{metadata['tag']}",
        "--image-spec", "v1.0",
        "--config", "manifest-config.json:application/vnd.cncf.flux.config.v1+json",
        "--annotation", f"org.opencontainers.image.created={created}",
        "--annotation", f"org.opencontainers.image.source=https://github.com/{os.environ['GITHUB_REPOSITORY']}",
        "--annotation", f"org.opencontainers.image.revision={metadata['tag']}/{metadata['sha']}",
        "manifest.tar.gz:application/vnd.cncf.flux.content.v1.tar+gzip", cwd=".release")


def main():
    metadata = json.loads(Path(".release/plan.json").read_text())
    if output("git", "rev-parse", "HEAD") != metadata["sha"]:
        raise ValueError("Checkout does not match the requested release SHA")
    metadata["policy_library_sha"] = output(
        "git", "ls-remote", "https://github.com/kyverno/policies.git", "HEAD"
    ).split()[0]
    metadata["chart_images"] = chart_image_references(read("charts/kyverno/values.yaml"), metadata)
    Path(".release/assets").mkdir(parents=True, exist_ok=True)
    Path(".release/sboms").mkdir(parents=True, exist_ok=True)
    Path(".release/bin").mkdir(parents=True, exist_ok=True)
    # Preserve the existing GoReleaser platform/architecture support and archive names.
    config = read(".goreleaser.yml")
    build = config["builds"][0]
    for goos in build["goos"]:
        for arch in build["goarch"]:
            if any(all({"goos": goos, "goarch": arch}.get(key) == value
                       for key, value in rule.items()) for rule in build.get("ignore", [])):
                continue
            binary = Path(f".release/bin/{goos}-{arch}/kyverno")
            binary.parent.mkdir(parents=True, exist_ok=True)
            run("go", "build", "-trimpath", "-ldflags",
                f"-s -w -X github.com/kyverno/kyverno/pkg/version.BuildVersion={metadata['tag']}",
                "-o", binary, "./cmd/cli/kubectl-kyverno",
                env={**os.environ, "GOOS": goos, "GOARCH": arch, "CGO_ENABLED": "0"})
            suffix = "zip" if goos == "windows" else "tar.gz"
            archive_arch = "x86_64" if arch == "amd64" else arch
            archive_cli(binary, f"kyverno-cli_{metadata['tag']}_{goos}_{archive_arch}.{suffix}",
                        goos == "windows")
    shutil.copyfile(".release/bin/linux-amd64/kyverno", "kubectl-kyverno")
    Path("kubectl-kyverno").chmod(0o755)
    run("git", "archive", "--format=tar.gz",
        f"--prefix=kyverno-{metadata['tag']}/",
        "-o", f".release/assets/kyverno-{metadata['tag']}-source.tar.gz", metadata["sha"])
    # ko creates OCI layouts without a registry write; all original base-image platforms remain.
    for name, main_path in IMAGES.items():
        layout = Path(f".release/images/{name}")
        run("ko", "build", f"./{main_path}", "--push=false", "--bare",
            "--platform=all", f"--oci-layout-path={layout}",
            "--image-annotation", f"org.opencontainers.image.revision={metadata['sha']}",
            "--image-annotation", f"org.opencontainers.image.source=https://github.com/{os.environ['GITHUB_REPOSITORY']}",
            env={**os.environ, "KO_DOCKER_REPO": f"ghcr.io/{metadata['owner']}/{name}",
                 "LD_FLAGS": f"-s -w -X github.com/kyverno/kyverno/pkg/version.BuildVersion={metadata['tag']}"})
        # Tests receive the exact linux/amd64 member of the staged multiarch index, not a rebuild.
        local = f"ko.local/github.com/kyverno/kyverno/{main_path}:{metadata['sha']}"
        archive = Path(f".release/docker/{name}.tar")
        archive.parent.mkdir(parents=True, exist_ok=True)
        run("skopeo", "copy", "--override-os", "linux", "--override-arch", "amd64",
            f"oci:{layout}", f"docker-archive:{archive}:{local}")
        run("docker", "load", "--input", archive)
        run("docker", "tag", local, metadata["images"][name])
        for keys, reference in metadata["chart_images"].items():
            if IMAGE_VALUES[keys] == name and reference != metadata["images"][name]:
                run("docker", "tag", local, reference)
    local_images = [
        f"ko.local/github.com/kyverno/kyverno/{path}:{metadata['sha']}"
        for path in IMAGES.values()
    ]
    version = output("docker", "run", "--rm", metadata["images"]["kyverno-cli"], "version")
    if not re.search(rf"(?<![\w.-]){re.escape(metadata['tag'])}(?![\w.-])", version):
        raise ValueError(f"Staged CLI image has the wrong application version: {version}")
    run("docker", "save", "--output", "kyverno.tar", *local_images,
        *sorted(set(metadata["images"].values()) | set(metadata["chart_images"].values())))
    shutil.rmtree(".release/docker")
    shutil.copyfile("kyverno.tar", ".release/kyverno.tar")
    shutil.copyfile("kubectl-kyverno", ".release/kubectl-kyverno")
    metadata["image_digests"] = {}
    for name in IMAGES:
        raw = subprocess.check_output(["skopeo", "inspect", "--raw", f"oci:.release/images/{name}"])
        metadata["image_digests"][name] = "sha256:" + hashlib.sha256(raw).hexdigest()
    prepare(metadata)
    verify_packages(metadata, ".release/packages")
    # Keep the Flux-compatible content layer and its OCI envelope immutable too.
    stage_manifest(metadata)
    # Publish the package bytes tested here; never re-run packaging in a publisher.
    with tarfile.open(".release/workspace.tar", "w") as archive:
        archive.add("charts", arcname="charts")
        archive.add("config/crds", arcname="config/crds")
        archive.add(".release/plan.json", arcname=".release/plan.json")
        archive.add(".release/packages", arcname=".release/packages")
        archive.add(".release/dependencies", arcname=".release/dependencies")
        archive.add(".release/install.yaml", arcname=".release/install.yaml")
    # Candidate-wide checksums also cover every CLI, OCI layer, manifest and dependency package.
    Path(".release/assets/checksums.txt").write_text(
        "".join(f"{sha256(path)}  {path.name}\n"
                for path in sorted(Path(".release/assets").iterdir()))
    )
    seal(".release")


if __name__ == "__main__":
    main()
