#!/usr/bin/env python3
"""Render advisory branch guidance; never modifies labels, PRs or checks."""

import argparse
import json
from pathlib import Path


def advisory(row):
    category = row["category"]
    base = row["base_ref"]
    if category == "LEGACY_ONLY" and base == "main":
        return (
            "Legacy-only candidate: review for release-1.19. "
            "Do not change the base without preparing and reviewing a rebased branch."
        )
    if category == "MIXED":
        return "Mixed or unavailable diff evidence: maintainer scope review is required."
    if category == "REVIEW-MIGRATION":
        return "Migration candidate: hold on main pending a maintainer decision."
    if base == "release-1.19":
        return (
            "Release PR: check backport eligibility and overlap. "
            "CEL fixes are valid on 1.19; classification is not a blanket rejection."
        )
    return "No legacy-only routing recommendation. Shared/CEL work normally stays on main."


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--summary", type=Path, required=True)
    args = parser.parse_args()
    rows = json.loads(args.report.read_text())
    if len(rows) != 1:
        parser.error("expected exactly one classified PR")
    row = rows[0]
    with args.summary.open("a") as stream:
        stream.write(
            "## Branch routing (advisory only)\n\n"
            f"{advisory(row)}\n\n"
            "This check does not approve retargeting, enforce branch policy, "
            "or change any labels or PRs. See `hack/pr-triage/README.md`.\n"
        )


if __name__ == "__main__":
    main()
