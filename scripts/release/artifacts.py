#!/usr/bin/env python3
"""Read same-run diagnostic artifacts, including successful jobs from earlier attempts."""

import io
import json
import os
from pathlib import Path
import subprocess
import sys
import zipfile


def available():
    pages = json.loads(subprocess.check_output([
        "gh", "api", "--paginate", "--slurp",
        f"repos/{os.environ['GITHUB_REPOSITORY']}/actions/runs/{os.environ['GITHUB_RUN_ID']}/artifacts",
    ], text=True))
    return [artifact for page in pages for artifact in page["artifacts"] if not artifact["expired"]]


def download(artifact, directory):
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    data = subprocess.check_output(["gh", "api", artifact["archive_download_url"]])
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        for member in archive.infolist():
            if not (directory / member.filename).resolve().is_relative_to(directory.resolve()):
                raise ValueError("Invalid diagnostic artifact path")
        archive.extractall(directory)


def restore_k6():
    artifacts = available()
    for scenario in ("simple", "kyverno", "kyverno-pss"):
        candidates = [artifact for artifact in artifacts
                      if artifact["name"].startswith(f"k6-reports-{scenario}-")]
        # "kyverno-" also prefixes "kyverno-pss-".
        if scenario == "kyverno":
            candidates = [artifact for artifact in candidates
                          if not artifact["name"].startswith("k6-reports-kyverno-pss-")]
        if not candidates:
            raise ValueError(f"Missing K6 reports for {scenario}")
        latest = max(candidates, key=lambda artifact: artifact["id"])
        download(latest, Path("summaries") / scenario)


if __name__ == "__main__":
    if sys.argv[1:] != ["k6"]:
        raise ValueError("Expected 'k6'")
    restore_k6()
