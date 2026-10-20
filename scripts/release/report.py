#!/usr/bin/env python3
"""Collect a release's full matrix, first failures, diagnostic logs and reproduction links."""

import json
import os
from pathlib import Path
import subprocess
import time

import artifacts
from release import output, write_json


def latest_jobs(pages):
    jobs = {}
    for page in pages:
        for job in page["jobs"]:
            previous = jobs.get(job["name"])
            if previous is None or job["id"] > previous["id"]:
                jobs[job["name"]] = job
    return sorted(jobs.values(), key=lambda job: job["id"])


def job_pages(repo, run_id):
    pages = []
    count = 0
    while True:
        # Large pages time out at GitHub's gateway once reusable-workflow jobs
        # span multiple attempts. Retrying one small page preserves earlier reads.
        url = f"repos/{repo}/actions/runs/{run_id}/jobs?filter=all&per_page=25&page={len(pages) + 1}"
        for retry in range(3):
            try:
                page = json.loads(subprocess.check_output(
                    ["gh", "api", url], text=True, timeout=60,
                ))
                break
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
                if retry == 2:
                    raise
                time.sleep(2 ** (retry + 1))
        pages.append(page)
        count += len(page["jobs"])
        if count >= page["total_count"]:
            return pages
        if not page["jobs"]:
            raise ValueError("Incomplete release job matrix; refusing to report a partial result")


def test_reports(root):
    rows = []
    links = []
    repo = os.environ["GITHUB_REPOSITORY"]
    run_id = os.environ["GITHUB_RUN_ID"]
    for artifact in artifacts.available():
        if not artifact["name"].startswith(("release-conformance-", "release-policy-library-", "k6-reports-")):
            continue
        directory = root / "test-reports" / artifact["name"]
        artifacts.download(artifact, directory)
        links.append(f"- [{artifact['name']}](https://github.com/{repo}/actions/runs/{run_id}/artifacts/{artifact['id']})")
        if artifact["name"].startswith("k6-reports-"):
            continue
        for path in directory.rglob("*.json"):
            document = json.loads(path.read_text())
            if not isinstance(document, dict):
                continue
            for test in document.get("tests", []):
                failures = [
                    {"step": step.get("name"), "operation": operation.get("name"),
                     "failure": operation["failure"],
                     "time": operation.get("endTime") or operation.get("startTime", "")}
                    for step in test.get("steps", []) for operation in step.get("operations", [])
                    if operation.get("failure")
                ]
                rows.append({
                    "artifact": artifact["name"], "test": test["name"],
                    "path": test.get("basePath"), "start": test.get("startTime", ""),
                    "result": "skipped" if test.get("skipped") else "failure" if failures else "success",
                    "failures": failures,
                })
    write_json(root / "per-test.json", rows)
    failed = sorted(
        (row for row in rows if row["result"] == "failure"),
        key=lambda row: min((failure["time"] for failure in row["failures"] if failure["time"]),
                            default=row["start"]),
    )
    first = json.dumps(failed[0], indent=2) if failed else "No failed per-test report was recorded."
    return [
        "", "## Per-test reports (all retained attempts)", f"{len(rows)} test results in `per-test.json`.",
        "Earlier failed attempts remain diagnostic history; the matrix shows each job's latest outcome.",
        "", "### First recorded test failure", "```json", first, "```", "", *links,
    ]


def main():
    repo = os.environ["GITHUB_REPOSITORY"]
    run_id = os.environ["GITHUB_RUN_ID"]
    attempt = os.environ["GITHUB_RUN_ATTEMPT"]
    pages = job_pages(repo, run_id)
    jobs = latest_jobs(pages)
    root = Path("release-report")
    root.mkdir(exist_ok=True)
    metadata = json.loads(Path(".release/plan.json").read_text()) if Path(".release/plan.json").exists() else {
        "sha": os.environ.get("RELEASE_SHA"), "tag": os.environ.get("RELEASE_TAG"),
    }
    write_json(root / "release.json", metadata)
    write_json(root / "jobs.json", jobs)
    lines = [
        f"# Release {metadata['tag']}", "",
        f"Commit: `{metadata['sha']}`; run `{run_id}`, attempt `{attempt}`.",
        "A failed, cancelled, or skipped mandatory job is NOT a completed release.", "",
        "## Versions and image digests", "```json", json.dumps(metadata, indent=2), "```", "",
        "## Full matrix", "| Job / Kubernetes / shard | Result | First failed step | Logs |",
        "| --- | --- | --- | --- |",
    ]
    log_flags = ["--allow-escape-sequences"] if "--allow-escape-sequences" in output(
        "gh", "api", "--help"
    ) else []
    for job in jobs:
        failed = next((step["name"] for step in job.get("steps", [])
                       if step.get("conclusion") == "failure"), "")
        lines.append(f"| {job['name']} | {job.get('conclusion') or job['status']} | "
                     f"{failed} | [logs]({job['html_url']}) |")
        if job["status"] == "completed" and job.get("conclusion") not in ("success", "skipped"):
            result = subprocess.run(
                ["gh", "api", *log_flags, f"repos/{repo}/actions/jobs/{job['id']}/logs"],
                capture_output=True,
            )
            (root / f"job-{job['id']}.log").write_bytes(
                result.stdout if result.returncode == 0 else result.stderr
            )
    lines += test_reports(root)
    lines += [
        "", "## Reproduce and recover", "```bash",
        f"git checkout --detach {metadata['sha']}",
        f"gh run download {run_id} -R {repo} --name release-candidate",
        "tar -xf release-candidate.tar",
        "python3 scripts/release/release.py check-seal",
        "# See each job's retained log for its exact test path, Kubernetes version and command.",
        f"gh run rerun {run_id} -R {repo} --failed", "```",
        "Rerun failed jobs in this run; never move/delete a tag, overwrite versions, or rebuild "
        "a partially published candidate. Expired candidates require maintainer recovery.",
    ]
    report = "\n".join(lines) + "\n"
    (root / "summary.md").write_text(report)
    with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as stream:
        stream.write(report)


if __name__ == "__main__":
    main()
