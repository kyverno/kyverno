#!/usr/bin/env python3
"""Idempotent stable-only Krew submission, using hashes of the staged archives."""

import base64
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time

import yaml

from release import check_seal, output, sha256


def manifest(metadata):
    text = Path(".krew.yaml").read_text().replace("{{ .TagName }}", metadata["tag"])
    pattern = r'(?m)^(\s*)\{\{addURIAndSha "([^"]+)" [^}]+\}\}$'

    def replace(match):
        indent, uri = match.groups()
        package = Path(".release/assets") / uri.rsplit("/", 1)[1]
        return f"{indent}uri: {uri}\n{indent}sha256: {sha256(package)}"

    text = re.sub(pattern, replace, text)
    if "{{" in text:
        raise ValueError("Unsupported Krew template: refusing to submit unresolved content")
    return yaml.safe_load(text)


def read_plugin(repo, ref):
    result = subprocess.run(
        ["gh", "api", f"repos/{repo}/contents/plugins/kyverno.yaml?ref={ref}"],
        capture_output=True, text=True,
    )
    if result.returncode != 0:
        if "(HTTP 404)" in result.stderr:
            return None
        raise ValueError(result.stderr)
    return yaml.safe_load(base64.b64decode(json.loads(result.stdout)["content"]))


def existing(metadata, expected):
    current = read_plugin("kubernetes-sigs/krew-index", "HEAD")
    if current and current["spec"]["version"] == metadata["tag"]:
        if current["spec"] != expected["spec"]:
            raise ValueError("Krew index has conflicting content for this version")
        return True
    # Avoid downgrading the stable plugin on a supported-branch patch release.
    if current:
        version = current["spec"]["version"].removeprefix("v")
        if re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", version) and tuple(
            map(int, version.split("."))
        ) > tuple(map(int, metadata["tag"][1:].split("."))):
            return True
    owner, repo = os.environ["GH_REPO"].split("/")
    branch = f"{owner}-kyverno-{repo}-{metadata['tag']}"
    prs = json.loads(output(
        "gh", "api", "--method", "GET",
        "repos/kubernetes-sigs/krew-index/pulls",
        "-f", f"head=krew-release-bot:{branch}", "-f", "state=open",
    ))
    for pr in prs:
        plugin = read_plugin("krew-release-bot/krew-index", pr["head"]["sha"])
        if plugin and plugin["spec"] == expected["spec"]:
            print(f"Verified Krew PR: {pr['html_url']}")
            return True
        raise ValueError(f"Existing Krew PR conflicts: {pr['html_url']}")
    return False


def main():
    check_seal(".release")
    metadata = json.loads(Path(".release/plan.json").read_text())
    if metadata["prerelease"]:
        if sys.argv[1] == "prepare":
            with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
                stream.write("submit=false\n")
        print("Prerelease: Krew must not change.")
        return
    expected = manifest(metadata)
    found = existing(metadata, expected)
    if sys.argv[1] == "prepare":
        path = Path(".release-publication/krew.yaml")
        path.parent.mkdir(exist_ok=True)
        path.write_text(yaml.safe_dump(expected, sort_keys=False))
        with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
            stream.write(f"submit={str(not found).lower()}\n")
    elif not found:
        for _ in range(12):
            time.sleep(5)
            if existing(metadata, expected):
                return
        raise ValueError("Krew submission was not found or did not persist; release stays draft")


if __name__ == "__main__":
    main()
