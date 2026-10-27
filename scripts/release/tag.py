#!/usr/bin/env python3
"""Create a validated tag. Never force-update or delete a ref."""

import json
import os
import re
import subprocess


def main():
    tag = os.environ["RELEASE_TAG"]
    sha = os.environ["RELEASE_SHA"]
    if not re.fullmatch(r"v[0-9A-Za-z.-]+", tag) or not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("Invalid validated tag/SHA")
    if sha != os.environ["GITHUB_SHA"]:
        raise ValueError("Dispatch SHA changed: run the workflow on the branch containing the exact SHA")
    repo = os.environ["GH_REPO"]
    result = subprocess.run(
        ["gh", "api", f"repos/{repo}/git/ref/tags/{tag}"], capture_output=True, text=True,
    )
    if result.returncode == 0:
        ref = json.loads(result.stdout)["object"]
        while ref["type"] == "tag":
            ref = json.loads(subprocess.check_output(
                ["gh", "api", f"repos/{repo}/git/tags/{ref['sha']}"], text=True
            ))["object"]
        if ref["type"] != "commit" or ref["sha"] != sha:
            raise ValueError("Existing tag points elsewhere; never move or delete it")
    elif "(HTTP 404)" in result.stderr:
        subprocess.run(["gh", "api", f"repos/{repo}/git/refs",
                        "-f", f"ref=refs/tags/{tag}", "-f", f"sha={sha}"], check=True)
    else:
        raise ValueError(f"Cannot check existing tag: {result.stderr}")


if __name__ == "__main__":
    main()
