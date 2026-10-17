#!/usr/bin/env python3
"""Prepare and verify a release without mutating a public tag or rebuilding artifacts."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request

import yaml


IMAGES = {
    "kyverno": "cmd/kyverno",
    "kyvernopre": "cmd/kyverno-init",
    "background-controller": "cmd/background-controller",
    "cleanup-controller": "cmd/cleanup-controller",
    "reports-controller": "cmd/reports-controller",
    "kyverno-cli": "cmd/cli/kubectl-kyverno",
    "readiness-checker": "cmd/readiness-checker",
}
IMAGE_VALUES = {
    "admissionController.container.image": "kyverno",
    "admissionController.initContainer.image": "kyvernopre",
    "backgroundController.image": "background-controller",
    "cleanupController.image": "cleanup-controller",
    "reportsController.image": "reports-controller",
    "webhooksCleanup.image": "readiness-checker",
    "test.image": "readiness-checker",
    "crds.migration.image": "kyverno-cli",
    "upgrade.legacyPolicyCheck.image": "kyverno-cli",
}
TAG = re.compile(
    r"^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
    r"(?P<pre>-(?:[0-9A-Za-z-]+)(?:\.[0-9A-Za-z-]+)*)?$"
)


def run(*args, **kwargs):
    print("+", " ".join(str(arg) for arg in args), flush=True)
    return subprocess.run(
        [str(arg) for arg in args], check=True, text=True, **kwargs
    )


def output(*args):
    return subprocess.check_output([str(arg) for arg in args], text=True).strip()


def read(path):
    return yaml.safe_load(Path(path).read_text())


def write_json(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")


def sha256(path):
    digest = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def plan(tag, sha, mapping, owner="kyverno"):
    match = TAG.fullmatch(tag)
    if not match or not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("Expected a vMAJOR.MINOR.PATCH[-prerelease] tag and full commit SHA")
    pre = match["pre"] or ""
    for part in pre.removeprefix("-").split("."):
        if part.isdigit() and len(part) > 1 and part.startswith("0"):
            raise ValueError("Numeric prerelease identifiers must not have leading zeros")
    line = ".".join(match.group(1, 2))
    versions = mapping.get("lines", {}).get(line)
    if not versions:
        raise ValueError(f"No release mapping for application line {line}")
    charts = {}
    for name in ("kyverno", "kyverno-policies"):
        minor = versions.get(name, "")
        if not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", minor):
            raise ValueError(f"Invalid {name} minor mapping: {minor!r}")
        charts[name] = f"{minor}.{match.group(3)}{pre}"
    if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", owner):
        raise ValueError("Invalid registry owner")
    return {
        "schema": 1,
        "tag": tag,
        "sha": sha,
        "prerelease": bool(pre),
        "charts": charts,
        "owner": owner,
        "locked_dependencies": mapping.get("dependencies", {}),
        "images": {name: f"ghcr.io/{owner}/{name}:{tag}" for name in IMAGES},
    }


def validate_event(metadata, event, ref, sha, rehearsal=False):
    if sha != metadata["sha"]:
        raise ValueError("Release SHA must equal the exact event SHA")
    if rehearsal:
        if event != "pull_request":
            raise ValueError("A rehearsal is allowed only on an unprivileged PR event")
        return
    if event == "push" and ref == f"refs/tags/{metadata['tag']}":
        return
    if event == "workflow_dispatch" and re.fullmatch(r"refs/heads/(main|release-[0-9]+\.[0-9]+)", ref):
        if ref.startswith("refs/heads/release-"):
            line = ref.removeprefix("refs/heads/release-")
            if not metadata["tag"].startswith(f"v{line}."):
                raise ValueError("Application tag line does not match the dispatch release branch")
        return
    raise ValueError("Only a matching tag push or an exact-SHA main/release branch dispatch may release")


def set_scalar(path, keys, value):
    """Replace one YAML scalar without losing helm-docs value descriptions."""
    text = Path(path).read_text()
    node = yaml.compose(text)
    for key in keys.split("."):
        node = next(child for name, child in node.value if name.value == key)
    if not isinstance(node, yaml.ScalarNode):
        raise ValueError(f"{keys} is not a scalar in {path}")
    Path(path).write_text(
        text[: node.start_mark.index]
        + json.dumps(value)
        + text[node.end_mark.index :]
    )


def external_dependencies(chart):
    return [
        {key: dependency.get(key, "") for key in ("name", "repository", "version")}
        for dependency in chart.get("dependencies", [])
        if dependency.get("repository")
    ]


def chart_image_references(values, metadata):
    references = {}
    for keys, name in IMAGE_VALUES.items():
        value = values
        for key in keys.split("."):
            value = value[key]
        registry = value["registry"] or values.get("global", {}).get("image", {}).get("registry") or value.get("defaultRegistry")
        repository = f"{metadata['owner']}/{name}"
        references[keys] = f"{registry + '/' if registry else ''}{repository}:{metadata['tag']}"
    return references


def prepare(metadata):
    charts = Path("charts")
    parent = charts / "kyverno"
    original_external = external_dependencies(read(parent / "Chart.lock"))
    if original_external != external_dependencies(read(parent / "Chart.yaml")):
        raise ValueError("External chart dependencies differ from Chart.lock")
    expected_names = {f"{item['name']}-{item['version']}.tgz" for item in original_external}
    if set(metadata["locked_dependencies"]) != expected_names:
        raise ValueError("External dependency checksum pins do not match Chart.lock")
    # Resolve the original lock before changing the bundled chart versions.
    add_repositories()
    run("helm", "dependency", "build", parent)
    retained = Path(".release/dependencies")
    retained.mkdir(parents=True, exist_ok=True)
    dependency_checksums = {}
    for dependency in original_external:
        name = f"{dependency['name']}-{dependency['version']}.tgz"
        source = parent / "charts" / name
        dependency_checksums[name] = sha256(source)
        if dependency_checksums[name] != metadata["locked_dependencies"][name]:
            raise ValueError(f"External dependency does not match its committed checksum pin: {name}")
        shutil.copyfile(source, retained / name)
    chart_paths = [
        (charts / name, version) for name, version in metadata["charts"].items()
    ]
    for name in ("crds", "grafana"):
        chart_paths.append((parent / "charts" / name, metadata["charts"]["kyverno"]))
    compatibility = {}
    for path, version in chart_paths:
        chart = read(path / "Chart.yaml")
        compatibility[str(path)] = chart.get("kubeVersion")
        set_scalar(path / "Chart.yaml", "version", version)
        if "appVersion" in chart:
            set_scalar(path / "Chart.yaml", "appVersion", metadata["tag"])
        if "artifacthub.io/prerelease" in chart.get("annotations", {}):
            # Annotation names contain dots, so use the YAML document here.
            chart = read(path / "Chart.yaml")
            chart["annotations"]["artifacthub.io/prerelease"] = str(
                metadata["prerelease"]
            ).lower()
            (path / "Chart.yaml").write_text(yaml.safe_dump(chart, sort_keys=False))
    chart = read(parent / "Chart.yaml")
    for dependency in chart["dependencies"]:
        if not dependency.get("repository"):
            dependency["version"] = metadata["charts"]["kyverno"]
    (parent / "Chart.yaml").write_text(yaml.safe_dump(chart, sort_keys=False))
    for key, image in IMAGE_VALUES.items():
        set_scalar(parent / "values.yaml", f"{key}.tag", metadata["tag"])
        set_scalar(parent / "values.yaml", f"{key}.repository", f"{metadata['owner']}/{image}")
    metadata["chart_images"] = chart_image_references(read(parent / "values.yaml"), metadata)
    # Only local versions changed. Every external constraint is an exact lock version.
    run("helm", "dependency", "update", "--skip-refresh", parent)
    if external_dependencies(read(parent / "Chart.lock")) != original_external:
        raise ValueError("Release preparation changed locked external dependencies")
    for name, expected in dependency_checksums.items():
        if sha256(parent / "charts" / name) != expected:
            raise ValueError(f"Release preparation changed external dependency bytes: {name}")
    for path, _ in chart_paths:
        if read(path / "Chart.yaml").get("kubeVersion") != compatibility[str(path)]:
            raise ValueError(f"Changed Kubernetes compatibility constraint in {path}")
    run("make", "codegen-helm-docs")
    packages = Path(".release/packages")
    packages.mkdir(parents=True, exist_ok=True)
    for name in metadata["charts"]:
        run("helm", "lint", "--strict", charts / name)
        run("helm", "package", charts / name, "--destination", packages)
    rendered = output(
        "helm", "template", "kyverno", packages / f"kyverno-{metadata['charts']['kyverno']}.tgz",
        "--namespace", "kyverno", "--kube-version", "1.36.1", "--skip-tests",
        "--set", "global.templating.enabled=true",
        "--set", f"global.templating.version={metadata['tag']}",
    )
    verify_images(rendered, metadata)
    Path(".release/install.yaml").write_text(rendered + "\n")
    metadata["compatibility"] = compatibility
    metadata["external_dependencies"] = original_external
    metadata["dependency_checksums"] = dependency_checksums
    metadata["lock_digest"] = read(parent / "Chart.lock")["digest"]
    write_json(".release/plan.json", metadata)


def add_repositories():
    helm = Path(".release/helm").resolve()
    helm.mkdir(parents=True, exist_ok=True)
    os.environ["HELM_REPOSITORY_CONFIG"] = str(helm / "repositories.yaml")
    os.environ["HELM_REPOSITORY_CACHE"] = str(helm / "cache")
    for name, url in (
        ("openreports", "https://openreports.github.io/reports-api"),
        ("reports-server", "https://kyverno.github.io/reports-server/"),
        ("kyverno-api", "https://kyverno.github.io/api"),
    ):
        run("helm", "repo", "add", name, url, "--force-update")


def verify_images(rendered, metadata):
    found = set()
    expected = {name: {reference} for name, reference in metadata["images"].items()}
    for keys, reference in metadata.get("chart_images", {}).items():
        expected[IMAGE_VALUES[keys]].add(reference)

    def visit(value):
        if isinstance(value, dict):
            for key, child in value.items():
                if key == "image" and isinstance(child, str):
                    for name, references in expected.items():
                        if child.rsplit(":", 1)[0] in {ref.rsplit(":", 1)[0] for ref in references}:
                            if child not in references:
                                raise ValueError(f"Stale image reference: {child}; expected {sorted(references)}")
                            found.add(name)
                visit(child)
        elif isinstance(value, list):
            for child in value:
                visit(child)

    for document in yaml.safe_load_all(rendered):
        visit(document)
    # CLI migration and cleanup hooks can be conditional. Validate their values separately.
    required = {"kyverno", "kyvernopre", "background-controller",
                "cleanup-controller", "reports-controller"}
    if not required <= found:
        raise ValueError(f"Missing rendered controller images: {sorted(required - found)}")
    return found


def dependency_files(archive, prefix):
    return {
        member.name.removeprefix(prefix): hashlib.sha256(archive.extractfile(member).read()).hexdigest()
        for member in archive.getmembers() if member.isfile() and member.name.startswith(prefix)
    }


def verify_packages(metadata, directory):
    for name, version in metadata["charts"].items():
        package = Path(directory) / f"{name}-{version}.tgz"
        with tarfile.open(package) as archive:
            chart = yaml.safe_load(archive.extractfile(f"{name}/Chart.yaml"))
            if chart["version"] != version or chart["appVersion"] != metadata["tag"]:
                raise ValueError(f"Stale chart metadata in {package}")
            expected = metadata.get("compatibility", {}).get(f"charts/{name}")
            if expected and chart.get("kubeVersion") != expected:
                raise ValueError(f"Changed kubeVersion in {package}")
            if name == "kyverno":
                values = yaml.safe_load(archive.extractfile("kyverno/values.yaml"))
                for keys, image in IMAGE_VALUES.items():
                    value = values
                    for key in keys.split("."):
                        value = value[key]
                    registry = value["registry"] or values.get("global", {}).get("image", {}).get("registry") or value.get("defaultRegistry")
                    ref = f"{registry + '/' if registry else ''}{value['repository']}:{value['tag']}"
                    expected = metadata.get("chart_images", {}).get(keys, metadata["images"][image])
                    if ref != expected:
                        raise ValueError(f"Stale image in {keys}: {ref}")
                for subchart in ("crds", "grafana"):
                    bundled = yaml.safe_load(
                        archive.extractfile(f"kyverno/charts/{subchart}/Chart.yaml")
                    )
                    if bundled["version"] != version:
                        raise ValueError(f"Stale {subchart} version")
                    if "appVersion" in bundled and bundled["appVersion"] != metadata["tag"]:
                        raise ValueError(f"Stale {subchart} appVersion")
                    key = f"charts/kyverno/charts/{subchart}"
                    if key in metadata.get("compatibility", {}) and bundled.get("kubeVersion") != metadata["compatibility"][key]:
                        raise ValueError(f"Changed {subchart} kubeVersion")
                for dependency in chart["dependencies"]:
                    if not dependency.get("repository") and dependency["version"] != version:
                        raise ValueError(f"Stale dependency reference: {dependency['name']}")
                if "external_dependencies" in metadata:
                    if metadata["dependency_checksums"] != metadata["locked_dependencies"]:
                        raise ValueError("Changed retained dependency checksum pins")
                    locked = yaml.safe_load(archive.extractfile("kyverno/Chart.lock"))
                    if locked["digest"] != metadata["lock_digest"]:
                        raise ValueError("Changed packaged dependency lock digest")
                    if external_dependencies(chart) != metadata["external_dependencies"] or external_dependencies(locked) != metadata["external_dependencies"]:
                        raise ValueError("Packaged external dependencies differ from release lock")
                    for dependency in metadata["external_dependencies"]:
                        external = yaml.safe_load(archive.extractfile(
                            f"kyverno/charts/{dependency['name']}/Chart.yaml"
                        ))
                        if external["version"] != dependency["version"]:
                            raise ValueError(f"Stale external dependency: {dependency['name']}")
                        name = f"{dependency['name']}-{dependency['version']}.tgz"
                        retained = Path(".release/dependencies") / name
                        if sha256(retained) != metadata["dependency_checksums"][name]:
                            raise ValueError(f"Changed retained dependency archive: {name}")
                        with tarfile.open(retained) as original:
                            expected = dependency_files(original, f"{dependency['name']}/")
                        if not expected or dependency_files(
                            archive, f"kyverno/charts/{dependency['name']}/"
                        ) != expected:
                            raise ValueError(f"Changed packaged dependency content: {name}")


def seal(root):
    root = Path(root)
    checksums = {
        str(path.relative_to(root)): sha256(path)
        for path in sorted(root.rglob("*"))
        if path.is_file() and path != root / "SHA256SUMS"
    }
    (root / "SHA256SUMS").write_text(
        "".join(f"{digest}  {name}\n" for name, digest in checksums.items())
    )


def check_seal(root):
    root = Path(root)
    expected = {}
    for line in (root / "SHA256SUMS").read_text().splitlines():
        digest, name = line.split("  ", 1)
        path = root / name
        if not path.resolve().is_relative_to(root.resolve()) or path.is_symlink():
            raise ValueError(f"Invalid checksum path: {name}")
        expected[name] = digest
        if sha256(path) != digest:
            raise ValueError(f"Checksum mismatch: {name}")
    actual = {
        str(path.relative_to(root)) for path in root.rglob("*")
        if path.is_file() and path != root / "SHA256SUMS"
    }
    if actual != set(expected):
        raise ValueError("Candidate contains unsealed or missing artifacts")


def same_or_absent(existing_digest, candidate_digest, name):
    if existing_digest is not None and existing_digest != candidate_digest:
        raise ValueError(f"Refusing to overwrite conflicting published content: {name}")
    return existing_digest is not None


def gh(*args):
    return output("gh", *args)


def existing_release(tag):
    result = subprocess.run(
        ["gh", "release", "view", tag, "--json", "isDraft,assets,isPrerelease,tagName"],
        capture_output=True, text=True,
    )
    if result.returncode == 0:
        return json.loads(result.stdout)
    if "release not found" not in result.stderr.lower():
        raise ValueError(f"Cannot check existing release: {result.stderr}")
    return None


def verify_tag(metadata):
    reference = json.loads(gh(
        "api", f"repos/{os.environ['GH_REPO']}/git/ref/tags/{metadata['tag']}"
    ))["object"]
    while reference["type"] == "tag":
        reference = json.loads(gh(
            "api", f"repos/{os.environ['GH_REPO']}/git/tags/{reference['sha']}"
        ))["object"]
    if reference["type"] != "commit" or reference["sha"] != metadata["sha"]:
        raise ValueError("Public tag no longer points at the validated SHA; never move or delete it")


def require_draft(metadata):
    release = existing_release(metadata["tag"])
    if release is None or not release["isDraft"]:
        raise ValueError("Release is not a draft; refusing to mutate or re-sign public content")
    return release


def ensure_draft(metadata):
    release = existing_release(metadata["tag"])
    if release is None:
        run("gh", "release", "create", metadata["tag"], "--verify-tag", "--draft",
            "--title", metadata["tag"], "--notes",
            f"Release candidate for {metadata['sha']}. Publication and remote verification in progress.",
            *(["--prerelease"] if metadata["prerelease"] else []))
    elif release["isPrerelease"] != metadata["prerelease"]:
        raise ValueError("Existing release prerelease status does not match the tag")
    elif not release["isDraft"]:
        raise ValueError("Existing release is public; refusing to mutate or re-sign it")
    return release


def upload_asset(tag, path):
    path = Path(path)
    release = existing_release(tag)
    asset = next((item for item in release["assets"] if item["name"] == path.name), None)
    if asset:
        # Authenticated API downloads work for draft releases as well.
        data = subprocess.check_output(
            ["gh", "api", "-H", "Accept: application/octet-stream",
             asset["apiUrl"]]
        )
        same_or_absent(hashlib.sha256(data).hexdigest(), sha256(path), path.name)
    else:
        if not release["isDraft"]:
            raise ValueError(f"Published release is missing {path.name}; refusing to mutate it")
        run("gh", "release", "upload", tag, path)


def inspect_digest(reference):
    result = subprocess.run(
        ["skopeo", "inspect", "--raw", f"docker://{reference}"],
        capture_output=True,
    )
    if result.returncode == 0:
        return "sha256:" + hashlib.sha256(result.stdout).hexdigest()
    error = result.stderr.decode()
    if "manifest unknown" in error.lower() or "name unknown" in error.lower():
        return None
    raise ValueError(f"Cannot inspect {reference}: {error}")


def publish_images(metadata):
    digests = {}
    for name in IMAGES:
        source = f"oci:.release/images/{name}"
        raw = subprocess.check_output(["skopeo", "inspect", "--raw", source])
        digest = "sha256:" + hashlib.sha256(raw).hexdigest()
        reference = metadata["images"][name]
        if not same_or_absent(inspect_digest(reference), digest, reference):
            run("skopeo", "copy", "--all", "--preserve-digests", source,
                f"docker://{reference}")
        if inspect_digest(reference) != digest:
            raise ValueError(f"Published digest mismatch for {reference}")
        subject = reference.rsplit(":", 1)[0] + "@" + digest
        sign(subject, "image", metadata)
        sbom = Path(f".release/sboms/{name}-bom.cdx.json")
        run("cosign", "attach", "sbom", "--type", "cyclonedx", "--sbom", sbom, subject,
            env={**os.environ, "COSIGN_REPOSITORY": f"ghcr.io/{metadata['owner']}/sbom"})
        digests[name] = digest
        upload_asset(metadata["tag"], sbom)
    write_json(".release-publication/images.json", digests)
    with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
        for name, digest in digests.items():
            stream.write(f"{name}={digest}\n")


def sign(subject, kind, metadata):
    run("cosign", "sign", "--yes",
        "-a", f"repo={os.environ['GH_REPO']}",
        "-a", f"workflow={os.environ['GITHUB_WORKFLOW']}",
        "-a", f"ref={metadata['sha']}", "-a", f"kind={kind}", subject,
        env={**os.environ, "COSIGN_REPOSITORY": f"ghcr.io/{metadata['owner']}/signatures"})


def verify_signature(subject, metadata):
    run("cosign", "verify", subject,
        "--certificate-identity-regexp",
        rf"^https://github\.com/{re.escape(os.environ['GH_REPO'])}/\.github/workflows/"
        r"(release|prepare-release|release-publish|helm-release)\.yaml@refs/"
        r"(tags/v.*|heads/(main|release-[^/]+))$",
        "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
        "--certificate-github-workflow-sha", metadata["sha"],
        "-a", f"ref={metadata['sha']}",
        env={**os.environ, "COSIGN_REPOSITORY": f"ghcr.io/{metadata['owner']}/signatures"})


def verify_provenance(subject):
    run("cosign", "verify-attestation", "--type", "slsaprovenance",
        "--certificate-identity",
        "https://github.com/slsa-framework/slsa-github-generator/"
        ".github/workflows/generator_container_slsa3.yml@refs/tags/v2.1.0",
        "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
        subject)


def verify_index_entry(name, entry, metadata):
    package = Path(f".release/packages/{name}-{metadata['charts'][name]}.tgz")
    with tarfile.open(package) as archive:
        chart = yaml.safe_load(archive.extractfile(f"{name}/Chart.yaml"))
    actual = {key: value for key, value in entry.items() if key not in ("created", "digest", "urls")}
    if actual != chart or entry.get("digest") != sha256(package):
        raise ValueError(f"Conflicting Helm index metadata or digest: {name}")
    urls = entry.get("urls", [])
    expected = f"https://kyverno.github.io/kyverno/{package.name}"
    if not urls or any(not isinstance(url, str) or urllib.parse.urljoin(
        "https://kyverno.github.io/kyverno/index.yaml", url
    ) != expected for url in urls):
        raise ValueError(f"Conflicting Helm index package URLs: {name}")
    return expected


def publish_charts(metadata):
    verify_packages(metadata, ".release/packages")
    pages = Path(".release-pages")
    run("git", "clone", "--single-branch", "--branch", "gh-pages",
        f"https://github.com/{os.environ['GH_REPO']}.git", pages)
    run("git", "-C", pages, "config", "credential.helper", "!gh auth git-credential")
    # The caller holds the repository-wide gh-pages concurrency lock.
    for name, version in metadata["charts"].items():
        package = Path(f".release/packages/{name}-{version}.tgz")
        destination = pages / package.name
        same_or_absent(sha256(destination) if destination.exists() else None,
                       sha256(package), destination)
        shutil.copyfile(package, destination)
        oci_tag = version.replace("+", "_")
        reference = f"ghcr.io/{metadata['owner']}/charts/{name}:{oci_tag}"
        existing = inspect_digest(reference)
        if existing:
            remote = Path(".release-remote-charts")
            remote.mkdir(exist_ok=True)
            run("helm", "pull", f"oci://ghcr.io/{metadata['owner']}/charts/{name}",
                "--version", version, "--destination", remote)
            same_or_absent(sha256(remote / package.name), sha256(package), reference)
        else:
            run("helm", "push", package, f"oci://ghcr.io/{metadata['owner']}/charts")
        digest = inspect_digest(reference)
        if not digest:
            raise ValueError(f"Missing chart after publication: {reference}")
        sign(reference.rsplit(":", 1)[0] + "@" + digest, "helm-chart", metadata)
        upload_asset(metadata["tag"], package)
    # Do not rewrite the index timestamp on a successful interrupted retry.
    index = read(pages / "index.yaml")
    missing = False
    for name, version in metadata["charts"].items():
        entry = next((item for item in index["entries"].get(name, [])
                      if item["version"] == version), None)
        if entry:
            verify_index_entry(name, entry, metadata)
        else:
            missing = True
    if missing:
        run("helm", "repo", "index", pages, "--url", "https://kyverno.github.io/kyverno/",
            "--merge", pages / "index.yaml")
    run("git", "-C", pages, "add", "index.yaml",
        *[f"{name}-{version}.tgz" for name, version in metadata["charts"].items()])
    if subprocess.run(["git", "-C", str(pages), "diff", "--cached", "--quiet"]).returncode:
        run("git", "-C", pages, "-c", "user.name=github-actions[bot]",
            "-c", "user.email=41898282+github-actions[bot]@users.noreply.github.com",
            "commit", "-m", f"Publish charts for {metadata['tag']}")
        # Authentication uses checkout's credential helper, never a token in a URL.
        run("git", "-C", pages, "push", "origin", "HEAD:gh-pages")


def verify_remote_charts(metadata):
    verify_packages(metadata, ".release/packages")
    # Pages/CDN propagation is asynchronous. Retry reads only, never rebuild/re-publish.
    last_error = None
    for attempt in range(12):
        try:
            with urllib.request.urlopen(
                "https://kyverno.github.io/kyverno/index.yaml", timeout=30
            ) as response:
                index = yaml.safe_load(response)
            for name, version in metadata["charts"].items():
                package = Path(f".release/packages/{name}-{version}.tgz")
                entry = next(item for item in index["entries"][name] if item["version"] == version)
                url = verify_index_entry(name, entry, metadata)
                with urllib.request.urlopen(url, timeout=30) as response:
                    digest = hashlib.sha256(response.read()).hexdigest()
                same_or_absent(digest, sha256(package), url)
                remote = Path(".release-verification")
                remote.mkdir(exist_ok=True)
                run("helm", "pull", f"oci://ghcr.io/{metadata['owner']}/charts/{name}",
                    "--version", version, "--destination", remote)
                same_or_absent(sha256(remote / package.name), sha256(package), name)
                ref = f"ghcr.io/{metadata['owner']}/charts/{name}:{version}"
                verify_signature(ref.rsplit(":", 1)[0] + "@" + inspect_digest(ref), metadata)
            last_error = None
            break
        except (urllib.error.URLError, KeyError, StopIteration, ValueError, TimeoutError,
                yaml.YAMLError, subprocess.CalledProcessError) as error:
            last_error = error
            print(f"Remote verification attempt {attempt + 1}: {error}", file=sys.stderr)
            time.sleep(20)
    if last_error:
        raise ValueError(f"Remote chart verification failed: {last_error}")


def verify_remote(metadata):
    verify_remote_charts(metadata)
    verify_images(output(
        "helm", "template", "kyverno",
        f".release-verification/kyverno-{metadata['charts']['kyverno']}.tgz",
        "--kube-version", "1.36.1", "--skip-tests",
    ), metadata)
    for name in IMAGES:
        source = f"oci:.release/images/{name}"
        raw = subprocess.check_output(["skopeo", "inspect", "--raw", source])
        expected = "sha256:" + hashlib.sha256(raw).hexdigest()
        if inspect_digest(metadata["images"][name]) != expected:
            raise ValueError(f"Remote image digest mismatch: {name}")
        for keys, reference in metadata.get("chart_images", {}).items():
            if IMAGE_VALUES[keys] == name and reference != metadata["images"][name]:
                for attempt in range(12):
                    if inspect_digest(reference) == expected:
                        break
                    print(f"Waiting for chart registry reference {reference} (attempt {attempt + 1})")
                    time.sleep(20)
                else:
                    raise ValueError(f"Chart registry mirror digest mismatch: {reference}")
        subject = metadata["images"][name].rsplit(":", 1)[0] + "@" + expected
        verify_signature(subject, metadata)
        verify_provenance(subject)
        downloaded = subprocess.check_output(
            ["cosign", "download", "sbom", subject],
            env={**os.environ, "COSIGN_REPOSITORY": f"ghcr.io/{metadata['owner']}/sbom"})
        if hashlib.sha256(downloaded.strip()).hexdigest() != hashlib.sha256(
            Path(f".release/sboms/{name}-bom.cdx.json").read_bytes().strip()
        ).hexdigest():
            raise ValueError(f"Remote SBOM mismatch: {name}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=(
        "plan", "event", "prepare", "verify-packages", "seal", "check-seal", "draft",
        "publish-images", "publish-charts", "verify-remote", "upload-assets",
    ))
    parser.add_argument("--tag", default=os.environ.get("RELEASE_TAG"))
    parser.add_argument("--sha", default=os.environ.get("RELEASE_SHA"))
    parser.add_argument("--owner", default="kyverno")
    args = parser.parse_args()
    if args.command in ("plan", "event", "prepare"):
        metadata = plan(args.tag, args.sha, read("config/releases.yaml"), args.owner)
        if args.command == "event":
            validate_event(metadata, os.environ["GITHUB_EVENT_NAME"], os.environ["GITHUB_REF"],
                           os.environ["GITHUB_SHA"], os.environ.get("RELEASE_REHEARSAL") == "true")
            if os.environ["GITHUB_EVENT_NAME"] == "push":
                verify_tag(metadata)
        write_json(".release/plan.json", metadata)
        if args.command == "prepare":
            prepare(metadata)
        else:
            print(json.dumps(metadata, indent=2))
        return
    if args.command == "seal":
        seal(".release")
        return
    check_seal(".release")
    metadata = json.loads(Path(".release/plan.json").read_text())
    if args.tag and args.tag != metadata["tag"] or args.sha and args.sha != metadata["sha"]:
        raise ValueError("Retained candidate does not match the requested tag/SHA")
    if args.command == "check-seal":
        return
    if args.command in ("draft", "publish-images", "publish-charts", "verify-remote", "upload-assets"):
        verify_tag(metadata)
    if args.command in ("publish-images", "publish-charts", "upload-assets"):
        require_draft(metadata)
    if args.command == "verify-packages":
        verify_packages(metadata, ".release/packages")
    elif args.command == "draft":
        ensure_draft(metadata)
    elif args.command == "publish-images":
        publish_images(metadata)
    elif args.command == "publish-charts":
        publish_charts(metadata)
    elif args.command == "verify-remote":
        verify_remote(metadata)
    elif args.command == "upload-assets":
        for path in sorted(Path(".release/assets").glob("*")):
            upload_asset(metadata["tag"], path)
        upload_asset(metadata["tag"], ".release/install.yaml")
        for path in sorted(Path("config/crds").rglob("*.yaml")):
            upload_asset(metadata["tag"], path)
        upload_asset(metadata["tag"], ".release/plan.json")
        upload_asset(metadata["tag"], ".release/SHA256SUMS")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
