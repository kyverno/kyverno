#!/usr/bin/env python3
"""Restore same-run immutable candidates on retries; expired artifacts fail closed."""

import json
import os
from pathlib import Path
import subprocess

from release import check_seal, output


NAMES = ("release-candidate", "release-workspace", "kyverno.tar", "kubectl-kyverno")


def main():
    pages = json.loads(output(
        "gh", "api", "--paginate", "--slurp",
        f"repos/{os.environ['GITHUB_REPOSITORY']}/actions/runs/{os.environ['GITHUB_RUN_ID']}/artifacts",
    ))
    artifacts = {item["name"]: item for page in pages for item in page["artifacts"]}
    if os.environ.get("GITHUB_EVENT_NAME") == "workflow_dispatch" and "release-candidate" not in artifacts:
        refs = json.loads(output(
            "gh", "api",
            f"repos/{os.environ['GITHUB_REPOSITORY']}/git/matching-refs/tags/{os.environ['RELEASE_TAG']}",
        ))
        if any(ref["ref"] == f"refs/tags/{os.environ['RELEASE_TAG']}" for ref in refs):
            raise ValueError("Release tag exists without its original retained candidate; refusing to rebuild")
        pages = json.loads(output(
            "gh", "api", "--paginate", "--slurp",
            f"repos/{os.environ['GITHUB_REPOSITORY']}/actions/workflows/prepare-release.yaml/runs"
            "?event=workflow_dispatch&per_page=100",
        ))
        title = f"Prepare release {os.environ['RELEASE_TAG']} at {os.environ['RELEASE_SHA']}"
        if any(
            run["id"] != int(os.environ["GITHUB_RUN_ID"]) and run.get("display_title") == title
            for page in pages for run in page["workflow_runs"]
        ):
            raise ValueError("A previous dispatch for this tag/SHA exists; refusing to rebuild")
    for name in NAMES:
        if name in artifacts and artifacts[name]["expired"]:
            raise ValueError(f"{name} expired: do not rebuild a partially published release")
    if "release-candidate" not in artifacts and any(name in artifacts for name in NAMES[1:]):
        raise ValueError("Derived artifacts remain without their sealed candidate; refusing to rebuild")
    if "release-candidate" not in artifacts and int(os.environ.get("GITHUB_RUN_ATTEMPT", "1")) > 1:
        history = json.loads(output(
            "gh", "api", "--paginate", "--slurp",
            f"repos/{os.environ['GITHUB_REPOSITORY']}/actions/runs/{os.environ['GITHUB_RUN_ID']}/jobs?filter=all&per_page=100",
        ))
        if any(step["name"] == "Retain immutable release candidate" and step.get("conclusion") == "success"
               for page in history for job in page["jobs"] for step in job.get("steps", [])):
            raise ValueError("Previously retained candidate disappeared; refusing to rebuild")
    with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
        for index, name in enumerate(NAMES):
            stream.write(f"artifact-{index}={'true' if name in artifacts else 'false'}\n")
    if "release-candidate" not in artifacts:
        return
    subprocess.run(["gh", "run", "download", os.environ["GITHUB_RUN_ID"],
                    "--name", "release-candidate"], check=True)
    subprocess.run(["tar", "-xf", "release-candidate.tar"], check=True)
    check_seal(".release")
    metadata = json.loads(Path(".release/plan.json").read_text())
    if metadata["tag"] != os.environ["RELEASE_TAG"] or metadata["sha"] != os.environ["RELEASE_SHA"]:
        raise ValueError("Retained artifact belongs to a different release")


if __name__ == "__main__":
    main()
