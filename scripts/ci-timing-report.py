#!/usr/bin/env python3
"""Collect read-only GitHub Actions data and report selected-workflow timing by SHA."""

import argparse
from collections import Counter, defaultdict
from datetime import datetime, timezone
import json
import math
from pathlib import Path
import re
import subprocess
from urllib.parse import urlencode


def timestamp(value):
    if not isinstance(value, str):
        raise ValueError("timestamp is missing or is not a string")
    result = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if result.tzinfo is None:
        raise ValueError("timestamps must include a timezone")
    return result


def elapsed(start, finish):
    seconds = (timestamp(finish) - timestamp(start)).total_seconds()
    if seconds < 0:
        raise ValueError("completion precedes start")
    return seconds


def percentile(values, fraction):
    """Nearest-rank percentile, returning null for an empty population."""
    return sorted(values)[math.ceil(len(values) * fraction) - 1] if values else None


def distribution(values):
    return {"n": len(values), "p50_seconds": percentile(values, 0.5),
            "p90_seconds": percentile(values, 0.9)}


def api_pages(endpoint):
    result = subprocess.run(
        ["gh", "api", "--paginate", "--slurp", endpoint],
        check=True, capture_output=True, text=True, encoding="utf-8", timeout=120,
    )
    return json.loads(result.stdout)


def collect(args):
    if not re.fullmatch(r"[\w.-]+/[\w.-]+", args.repo):
        raise ValueError("repo must be OWNER/REPO")
    if timestamp(args.since) >= timestamp(args.until):
        raise ValueError("since must precede until")
    workflows = sorted(set(args.workflow))
    runs = []
    for workflow in workflows:
        if not re.fullmatch(r"[\w.-]+\.ya?ml", workflow):
            raise ValueError("workflow must be a workflow filename, for example check-unit-tests.yaml")
        query = {"event": args.event, "created": f"{args.since}..{args.until}", "per_page": 100}
        if args.branch:
            query["branch"] = args.branch
        pages = api_pages(f"repos/{args.repo}/actions/workflows/{workflow}/runs?{urlencode(query)}")
        if any(page["total_count"] >= 1000 for page in pages):
            raise ValueError("run search may reach GitHub's 1,000-result cap; use a smaller time window")
        workflow_runs = [run for page in pages for run in page["workflow_runs"]]
        if len({run["id"] for run in workflow_runs}) != len(workflow_runs) or any(
                page["total_count"] != len(workflow_runs) for page in pages):
            raise ValueError("run pagination changed during collection; retry after runs settle")
        for run in workflow_runs:
            attempt = run["run_attempt"]
            jobs = api_pages(f"repos/{args.repo}/actions/runs/{run['id']}/attempts/{attempt}/jobs?per_page=100")
            inventory = [job for batch in jobs for job in batch["jobs"]]
            if len({job["id"] for job in inventory}) != len(inventory) or any(
                    batch.get("total_count", len(inventory)) != len(inventory) for batch in jobs):
                raise ValueError("job inventory changed or is incomplete; retry after runs settle")
            runs.append({"workflow": workflow, "id": run["id"], "attempt": attempt,
                         "sha": run["head_sha"], "event": run["event"],
                         "created_at": run["created_at"], "status": run["status"],
                         "conclusion": run["conclusion"], "url": run["html_url"],
                         "jobs": inventory})
    return {"schema_version": 1, "repository": args.repo, "event": args.event,
            "branch": args.branch, "since": args.since, "until": args.until,
            "collected_at": datetime.now(timezone.utc).isoformat(),
            "workflows": workflows, "runs": runs}


def analyze(snapshot):
    if snapshot.get("schema_version") != 1:
        raise ValueError("unsupported snapshot schema")
    expected = set(snapshot["workflows"])
    if not expected:
        raise ValueError("at least one expected workflow is required")
    groups = defaultdict(list)
    for run in snapshot["runs"]:
        if run["workflow"] not in expected or run["event"] != snapshot["event"]:
            raise ValueError("snapshot contains an unexpected workflow or event")
        timestamp(run["created_at"])
        groups[run["sha"]].append(run)
    cohorts, latencies, occupancies, delays = [], [], [], []
    workflow_timings = defaultdict(list)
    for sha, runs in sorted(groups.items()):
        latest = {}
        for run in runs:
            key = (timestamp(run["created_at"]), int(run["id"]), int(run["attempt"]))
            previous = latest.get(run["workflow"])
            if previous is None or key > previous[0]:
                latest[run["workflow"]] = (key, run)
        selected = [entry[1] for entry in latest.values()]
        missing = sorted(expected - set(latest))
        conclusions = Counter(run["conclusion"] or run["status"] for run in selected)
        state = "success"
        if missing:
            state = "missing_workflows"
        elif conclusions["action_required"] or conclusions["waiting"]:
            state = "action_required"
        elif conclusions["cancelled"]:
            state = "cancelled"
        elif any(run["conclusion"] in ("failure", "timed_out", "startup_failure", "stale") for run in selected):
            state = "failure"
        elif any(run["status"] != "completed" for run in selected):
            state = "in_progress"
        elif any(run["conclusion"] != "success" for run in selected):
            state = "non_success"
        row = {"sha": sha, "state": state, "missing_workflows": missing,
               "outcomes": dict(conclusions), "runs": [
                   {"workflow": r["workflow"], "id": r["id"], "attempt": r["attempt"], "url": r.get("url")}
                   for r in sorted(selected, key=lambda r: r["workflow"])],
               "observed_wall_seconds": None, "latest_attempt_runner_seconds": None}
        if state == "success":
            starts, finishes, seconds, per_workflow = [], [], [], {}
            try:
                for run in selected:
                    active = [j for j in run["jobs"] if j.get("conclusion") != "skipped"]
                    if not active or any(j.get("status") != "completed" or
                                         j.get("conclusion") not in ("success", "neutral") for j in active):
                        raise ValueError("incomplete or inconsistent job inventory")
                    job_starts = [timestamp(j["started_at"]) for j in active]
                    job_finishes = [timestamp(j["completed_at"]) for j in active]
                    seconds.extend(elapsed(j["started_at"], j["completed_at"]) for j in active)
                    first, last = min(job_starts), max(job_finishes)
                    created = timestamp(run["created_at"])
                    if first < created:
                        raise ValueError("job started before workflow creation")
                    starts.append(first)
                    finishes.append(last)
                    per_workflow[run["workflow"]] = (last - created).total_seconds()
                origin = min(timestamp(r["created_at"]) for r in runs)
                wall = (max(finishes) - origin).total_seconds()
                delay = (min(starts) - origin).total_seconds()
                row.update(observed_wall_seconds=wall, latest_attempt_runner_seconds=sum(seconds),
                           creation_to_first_job_seconds=delay)
                latencies.append(wall)
                occupancies.append(sum(seconds))
                delays.append(delay)
                for name, duration in per_workflow.items():
                    workflow_timings[name].append(duration)
            except (ValueError, TypeError, KeyError) as error:
                row["state"] = "incomplete_jobs"
                row["reason"] = str(error)
        cohorts.append(row)
    return {"repository": snapshot["repository"], "event": snapshot["event"],
            "collected_at": snapshot.get("collected_at"),
            "since": snapshot.get("since"), "until": snapshot.get("until"),
            "workflows": sorted(expected), "cohort_count": len(cohorts),
            "states": dict(Counter(row["state"] for row in cohorts)),
            "observed_wall": distribution(latencies),
            "latest_attempt_runner_occupancy": distribution(occupancies),
            "creation_to_first_job_delay": distribution(delays),
            "per_workflow_wall": {name: distribution(values) for name, values in sorted(workflow_timings.items())},
            "cohorts": cohorts}


def markdown(report):
    def seconds(value):
        return "n/a" if value is None else f"{value:.1f}s"
    lines = ["# Selected-workflow CI timing", "", f"Repository: {report['repository']}; event: {report['event']}.",
             f"Workflow creation window: {report['since']} through {report['until']}.",
             f"Snapshot collected: {report['collected_at']}.",
             "", "Workflows: " + ", ".join(report["workflows"]), "",
             f"Cohorts: {report['cohort_count']}; outcomes: {json.dumps(report['states'], sort_keys=True)}.", "",
             "Percentiles use nearest rank and only complete, successful SHA cohorts. Other cohorts remain listed.",
             "This is not an all-required-checks or merge-readiness measurement. External checks are outside this snapshot.",
             "Outcome counts describe selected latest runs, not all historical attempts.",
             "Approval, queue and retry waits cannot be separated: pre-job delay includes all three.",
             "Runner occupancy sums job elapsed time for latest attempts only; it is not billed minutes or dollars.",
             "SHA wall time begins at earliest collected workflow creation and includes waits before reruns/replacement runs.",
             "The creation-window sample can omit workflows created outside that window; missing cohorts are excluded.", "",
             "| Metric (successful cohorts only) | n | p50 | p90 |", "|---|---:|---:|---:|"]
    for name, key in [("Observed SHA wall time", "observed_wall"),
                      ("Latest-attempt runner occupancy", "latest_attempt_runner_occupancy"),
                      ("Creation to first job delay", "creation_to_first_job_delay")]:
        metric = report[key]
        lines.append(f"| {name} | {metric['n']} | {seconds(metric['p50_seconds'])} | {seconds(metric['p90_seconds'])} |")
    if report["observed_wall"]["n"] < 100:
        lines += ["", "Fewer than 100 successful cohorts: exploratory sample, not evidence of a stable p90 or speedup."]
    lines += ["", "| Workflow (same successful cohorts) | n | p50 wall | p90 wall |", "|---|---:|---:|---:|"]
    for name, metric in report["per_workflow_wall"].items():
        lines.append(f"| {name} | {metric['n']} | {seconds(metric['p50_seconds'])} | {seconds(metric['p90_seconds'])} |")
    lines += ["", "| SHA | Outcome | Missing workflows | Wall | Runner occupancy |", "|---|---|---|---:|---:|"]
    for row in report["cohorts"]:
        lines.append(f"| {row['sha']} | {row['state']} | {', '.join(row['missing_workflows']) or '-'} | "
                     f"{seconds(row['observed_wall_seconds'])} | {seconds(row['latest_attempt_runner_seconds'])} |")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    grab = commands.add_parser("collect", help="read Actions metadata with an authenticated gh CLI")
    grab.add_argument("--repo", default="kyverno/kyverno")
    grab.add_argument("--workflow", action="append", required=True)
    grab.add_argument("--event", choices=["pull_request", "push"], required=True)
    grab.add_argument("--branch", help="head branch filter; normally main for push cohorts")
    grab.add_argument("--since", required=True, help="ISO 8601 timestamp with timezone")
    grab.add_argument("--until", required=True, help="ISO 8601 timestamp with timezone")
    grab.add_argument("--output", type=Path, required=True)
    report = commands.add_parser("report", help="analyze a saved snapshot without network access")
    report.add_argument("snapshot", type=Path)
    report.add_argument("--format", choices=["json", "markdown"], default="markdown")
    args = parser.parse_args()
    try:
        if args.command == "collect":
            data = collect(args)
            args.output.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")
        else:
            data = analyze(json.loads(args.snapshot.read_text(encoding="utf-8")))
            print(json.dumps(data, indent=2) if args.format == "json" else markdown(data), end="\n" if args.format == "json" else "")
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        parser.exit(1, f"error: {error}\n")


if __name__ == "__main__":
    main()
