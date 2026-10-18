#!/usr/bin/env python3
"""Publish, verify and finally undraft exactly the retained binary and manifest assets."""

import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from release import (
    check_seal, existing_release, require_draft, run, same_or_absent, sha256, sign,
    upload_asset, verify_signature, verify_tag, workflow_identity,
)


def asset_bytes(tag, name):
    release = existing_release(tag)
    asset = next((item for item in release["assets"] if item["name"] == name), None)
    if asset is None:
        return None
    return subprocess.check_output([
        "gh", "api", "-H", "Accept: application/octet-stream",
        asset["apiUrl"],
    ])


def blob_identity(metadata):
    return workflow_identity(metadata, ("release", "prepare-release", "release-publish"))


def verify_blob(path, bundle, metadata):
    run("cosign", "verify-blob", "--bundle", bundle, path,
        "--certificate-identity-regexp", blob_identity(metadata),
        "--certificate-github-workflow-sha", metadata["sha"],
        "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com")


def all_assets():
    return [
        *sorted(Path(".release/assets").iterdir()),
        *sorted(Path(".release/packages").glob("*.tgz")),
        *sorted(Path(".release/sboms").glob("*-bom.cdx.json")),
        Path(".release/install.yaml"),
        *sorted(Path("config/crds").rglob("*.yaml")),
        Path(".release/plan.json"), Path(".release/SHA256SUMS"),
    ]


def manifest_digest(metadata):
    index = json.loads(Path(".release/manifests/index.json").read_text())
    descriptor = next(item for item in index["manifests"]
                      if item.get("annotations", {}).get("org.opencontainers.image.ref.name") == metadata["tag"])
    return descriptor["digest"]


def remote_manifest(reference):
    result = subprocess.run(
        ["oras", "manifest", "fetch", "--descriptor", reference],
        capture_output=True, text=True,
    )
    if result.returncode == 0:
        return json.loads(result.stdout)["digest"]
    if "not found" in result.stderr.lower() or "manifest_unknown" in result.stderr.lower():
        return None
    raise ValueError(f"Cannot inspect remote manifest: {result.stderr}")


def publish(metadata):
    directory = Path(".release-publication")
    directory.mkdir(exist_ok=True)
    for path in all_assets():
        upload_asset(metadata["tag"], path)
        bundle = directory / f"{path.name}.sigstore.json"
        existing = asset_bytes(metadata["tag"], bundle.name)
        if existing is None:
            run("cosign", "sign-blob", "--yes", "--bundle", bundle, path)
        else:
            bundle.write_bytes(existing)
        verify_blob(path, bundle, metadata)
        upload_asset(metadata["tag"], bundle)
    reference = f"ghcr.io/{metadata['owner']}/manifests/kyverno:{metadata['tag']}"
    digest = manifest_digest(metadata)
    if not same_or_absent(remote_manifest(reference), digest, reference):
        run("oras", "cp", "--from-oci-layout",
            f".release/manifests:{metadata['tag']}", reference)
    if remote_manifest(reference) != digest:
        raise ValueError("Remote install manifest digest does not match staged artifact")
    sign(reference.rsplit(":", 1)[0] + "@" + digest, "manifest", metadata)


def verify(metadata):
    directory = Path(".release-verification")
    directory.mkdir(exist_ok=True)
    for path in all_assets():
        data = asset_bytes(metadata["tag"], path.name)
        if data is None or hashlib.sha256(data).hexdigest() != sha256(path):
            raise ValueError(f"Missing or conflicting remote asset: {path.name}")
        bundle = directory / f"{path.name}.sigstore.json"
        data = asset_bytes(metadata["tag"], bundle.name)
        if data is None:
            raise ValueError(f"Missing signature bundle: {bundle.name}")
        bundle.write_bytes(data)
        verify_blob(path, bundle, metadata)
    reference = f"ghcr.io/{metadata['owner']}/manifests/kyverno:{metadata['tag']}"
    digest = manifest_digest(metadata)
    if remote_manifest(reference) != digest:
        raise ValueError("Remote manifest OCI digest mismatch")
    verify_signature(reference.rsplit(":", 1)[0] + "@" + digest, metadata)
    # A digest verifies the entire OCI object, including the Flux-compatible install layer.
    run("oras", "pull", reference, "--output", directory / "manifest")
    same_or_absent(sha256(directory / "manifest/manifest.tar.gz"),
                   sha256(".release/manifest.tar.gz"), "manifest content")


def should_be_latest(metadata):
    if metadata["prerelease"]:
        return False
    pages = json.loads(subprocess.check_output([
        "gh", "api", "--paginate", "--slurp",
        f"repos/{os.environ['GH_REPO']}/releases?per_page=100",
    ], text=True))
    candidate = tuple(map(int, metadata["tag"][1:].split(".")))
    versions = [
        tuple(map(int, match.groups()))
        for page in pages for release in page
        if not release["draft"] and not release["prerelease"]
        for match in [re.fullmatch(r"v([0-9]+)\.([0-9]+)\.([0-9]+)", release["tag_name"])]
        if match
    ]
    return candidate >= max(versions, default=candidate)


def complete(metadata):
    release = existing_release(metadata["tag"])
    if release is None:
        raise ValueError("Draft release disappeared")
    notes = ""
    # Read the changelog without allowing Make to rebuild any artifacts.
    changelog = Path("CHANGELOG.md").read_text()
    heading = f"## {metadata['tag']}"
    if heading in changelog:
        notes = changelog[changelog.index(heading):].split("\n## ", 1)[0]
    notes += (
        f"\n\nAll mandatory release validation and remote verification completed for `{metadata['sha']}`.\n"
        f"Charts: {json.dumps(metadata['charts'], sort_keys=True)}.\n"
        f"Full matrix and diagnostics: https://github.com/{os.environ['GH_REPO']}/actions/runs/"
        f"{os.environ['GITHUB_RUN_ID']}.\n"
    )
    path = Path(".release-publication/release-notes.txt")
    path.parent.mkdir(exist_ok=True)
    path.write_text(notes)
    latest = should_be_latest(metadata)
    if release["isDraft"]:
        verify_tag(metadata)
        current = existing_release(metadata["tag"])
        if current is None or not current["isDraft"]:
            raise ValueError("Release is no longer a draft; refusing to mutate public content")
        verify_tag(metadata)
        run("gh", "release", "edit", metadata["tag"], "--draft=false",
            f"--prerelease={str(metadata['prerelease']).lower()}",
            f"--latest={str(latest).lower()}", "--notes-file", path)
    final = existing_release(metadata["tag"])
    if final["isDraft"] or final["isPrerelease"] != metadata["prerelease"]:
        raise ValueError("Release finalization did not persist")


def main():
    check_seal(".release")
    metadata = json.loads(Path(".release/plan.json").read_text())
    verify_tag(metadata)
    if sys.argv[1] == "publish":
        require_draft(metadata)
    {"publish": publish, "verify": verify, "complete": complete}[sys.argv[1]](metadata)


if __name__ == "__main__":
    main()
