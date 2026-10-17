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
    for name in NAMES:
        if name in artifacts and artifacts[name]["expired"]:
            raise ValueError(f"{name} expired: do not rebuild a partially published release")
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
