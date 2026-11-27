#!/usr/bin/env python3
"""Prepares and summarizes chainsaw reports from the E2E conformance jobs.

Uses the Python 3 standard library only, so it runs on any runner without
setup.

  report.py prepare --dir DIR --suite SUITE [--variant VARIANT] --k8s VERSION
                    --shard N [--outcome OUTCOME] [--job-url URL]
      Run at the end of each conformance job. Converts DIR/results.json, the
      chainsaw JSON report, to DIR/results.xml in JUnit format, writes
      DIR/meta.json with the job details, and prints the artifact name
      e2e-reports-<suite>[-<variant>]-<k8s>-<shard>. The variant holds the
      matrix values, other than the version and the shard, that change what a
      job of the suite tests, such as the kyverno-configs of the rbac jobs.

  report.py summarize DIR [--run-url URL]
      Run once after all conformance jobs. Reads every DIR/*/meta.json and
      results.json and prints one Markdown table with a row per failing test:
      the failing Kubernetes versions with links to their jobs, the scope, and
      the first error line.

  report.py counts DIR
      Reads the same reports and prints a JSON object with the number of
      passed, failed, and skipped test results across every job and shard,
      the number of jobs that failed without a report, and the number of
      chainsaw reports read. The /conformance
      PR comment command uses it to tell a pass from a regex that matched no
      test.

A test that fails on every Kubernetes version it ran on is likely
deterministic ("all versions"). A test that fails on only some of them is
version-specific or flaky ("some versions").
"""

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path
from xml.etree import ElementTree

ARTIFACT_PREFIX = "e2e-reports-"
REPORT = "results.json"
JUNIT = "results.xml"
META = "meta.json"
MAX_ERROR = 200
MAX_SKIPPED_NAMES = 10


def artifact_name(suite, k8s, shard, variant=""):
    """Returns the artifact name, with characters artifacts reject replaced."""
    parts = [suite, variant, k8s, shard] if variant else [suite, k8s, shard]
    safe = re.sub(r"[^A-Za-z0-9._-]+", "-", "-".join(parts)).strip("-")
    return ARTIFACT_PREFIX + safe


def suite_label(suite, variant):
    """Returns the suite name, followed by the variant when there's one."""
    return f"{suite} ({variant})" if variant else suite


def failures(test):
    """Returns the failures of a chainsaw test report, in step order."""
    return [
        {"step": step.get("name", ""), "operation": operation.get("name", ""),
         "error": (operation.get("failure") or {}).get("error", "")}
        for step in test.get("steps") or []
        for operation in step.get("operations") or []
        if operation.get("failure") or operation.get("status") == "failed"
    ]


def status(test):
    """Returns passed, failed, or skipped for a chainsaw test report."""
    if test.get("skipped") or test.get("status") == "skipped":
        return "skipped"
    if test.get("status") == "failed" or failures(test):
        return "failed"
    return "passed"


def first_line(text):
    """Returns the first non-empty line of an error, shortened."""
    line = next((line.strip() for line in str(text).splitlines() if line.strip()), "")
    return line if len(line) <= MAX_ERROR else line[:MAX_ERROR - 1] + "\u2026"


def seconds(test):
    """Returns the duration of a test in seconds, or 0 when unknown."""

    def parse(value):
        # Chainsaw writes RFC 3339 times with up to nine fractional digits,
        # which datetime.fromisoformat doesn't accept before Python 3.11.
        match = re.match(r"(.*T\d\d:\d\d:\d\d)(?:\.(\d+))?(Z|[+-]\d\d:\d\d)$", value or "")
        if not match:
            return None
        base, fraction, zone = match.groups()
        zone = "+00:00" if zone == "Z" else zone
        return datetime.fromisoformat(f"{base}.{(fraction or '0')[:6].ljust(6, '0')}{zone}")

    start, end = parse(test.get("startTime")), parse(test.get("endTime"))
    return max((end - start).total_seconds(), 0.0) if start and end else 0.0


def to_junit(report, suite):
    """Converts a chainsaw JSON report to a JUnit XML tree, one case per test."""
    tests = report.get("tests") or []
    root = ElementTree.Element("testsuites", name=suite)
    element = ElementTree.SubElement(root, "testsuite", name=suite)
    counts = {"failed": 0, "skipped": 0}
    total = 0.0
    for test in tests:
        duration = seconds(test)
        total += duration
        case = ElementTree.SubElement(
            element, "testcase", name=test.get("name", ""),
            classname=test.get("basePath", ""), time=f"{duration:.3f}")
        result = status(test)
        if result == "skipped":
            counts["skipped"] += 1
            ElementTree.SubElement(case, "skipped")
        elif result == "failed":
            counts["failed"] += 1
            found = failures(test)
            message = first_line(found[0]["error"]) if found else "failed"
            failure = ElementTree.SubElement(case, "failure", message=message)
            failure.text = "\n\n".join(
                f"step {f['step']}, {f['operation']}:\n{f['error']}" for f in found)
    for node in (root, element):
        node.set("tests", str(len(tests)))
        node.set("failures", str(counts["failed"]))
        node.set("skipped", str(counts["skipped"]))
        node.set("time", f"{total:.3f}")
    return ElementTree.ElementTree(root)


def prepare(args):
    directory = Path(args.dir)
    directory.mkdir(parents=True, exist_ok=True)
    report = directory / REPORT
    has_report = report.is_file()
    if has_report:
        tree = to_junit(json.loads(report.read_text()), suite_label(args.suite, args.variant))
        ElementTree.indent(tree)
        tree.write(directory / JUNIT, encoding="unicode", xml_declaration=True)
    meta = {
        "suite": args.suite, "variant": args.variant, "k8s": args.k8s, "shard": args.shard,
        "outcome": args.outcome, "jobUrl": args.job_url, "report": has_report,
    }
    (directory / META).write_text(json.dumps(meta, indent=2) + "\n")
    print(artifact_name(args.suite, args.k8s, args.shard, args.variant))


def version_key(version):
    return tuple(int(part) if part.isdigit() else 0 for part in re.findall(r"\d+|\D+", version))


def collect(root):
    """Reads the downloaded artifacts. Returns (results, jobs without a report)."""
    results, missing = [], []
    # download-artifact puts each artifact in its own subdirectory, but may
    # extract a single matching artifact straight into root, so read both.
    root = Path(root)
    for meta_path in sorted({*root.glob(META), *root.glob(f"*/{META}")}):
        meta = json.loads(meta_path.read_text())
        report = meta_path.parent / REPORT
        if not report.is_file():
            if meta.get("outcome") != "success":
                missing.append(meta)
            continue
        for test in json.loads(report.read_text()).get("tests") or []:
            found = failures(test)
            results.append({
                "suite": meta.get("suite", ""), "variant": meta.get("variant", ""),
                "k8s": meta.get("k8s", ""),
                "jobUrl": meta.get("jobUrl", ""),
                "test": test.get("name", ""), "path": test.get("basePath", ""),
                "status": status(test),
                "error": first_line(found[0]["error"]) if found else "",
                "step": found[0]["step"] if found else "",
            })
    return results, missing


def group(results):
    """Groups the results per test and variant. Returns one entry per failing test.

    Jobs of the same suite with different variants test different settings,
    so their results are grouped separately.
    """
    tests = {}
    for result in results:
        if result["status"] == "skipped":
            continue
        key = (result["suite"], result["variant"], result["path"] or result["test"])
        entry = tests.setdefault(key, {
            "suite": suite_label(result["suite"], result["variant"]),
            "test": result["test"], "path": result["path"],
            "ran": set(), "failed": {},
        })
        entry["ran"].add(result["k8s"])
        if result["status"] == "failed":
            entry["failed"][result["k8s"]] = result
    rows = []
    for entry in tests.values():
        if not entry["failed"]:
            continue
        failed = sorted(entry["failed"], key=version_key)
        first = entry["failed"][failed[0]]
        rows.append({
            "suite": entry["suite"], "test": entry["test"], "path": entry["path"],
            "failed": failed, "ran": sorted(entry["ran"], key=version_key),
            "scope": "all versions" if len(failed) == len(entry["ran"]) else "some versions",
            "error": first["error"], "step": first["step"],
            "links": {k8s: entry["failed"][k8s]["jobUrl"] for k8s in failed},
        })
    # Failures on every version first: they're likely one deterministic cause.
    rows.sort(key=lambda row: (row["scope"] != "all versions", row["suite"], row["test"]))
    return rows


def cell(text):
    """Escapes text for a Markdown table cell."""
    text = str(text).replace("\\", "\\\\").replace("|", "\\|").replace("`", "'")
    return text.replace("<", "&lt;").replace(">", "&gt;").replace("\n", " ")


def link(text, url):
    return f"[{text}]({url})" if url else text


def skipped_tests(results):
    """Returns the skipped tests, each once, with the versions it was skipped on."""
    tests = {}
    for result in results:
        if result["status"] == "skipped":
            label = f"{result['test']} ({suite_label(result['suite'], result['variant'])})"
            tests.setdefault(label, set()).add(result["k8s"])
    return {label: sorted(versions, key=version_key) for label, versions in sorted(tests.items())}


def markdown(rows, missing, results, run_url=""):
    lines = ["## E2E failures", ""]
    if not results and not missing:
        lines.append("No chainsaw reports were found, so there's nothing to summarize.")
        return "\n".join(lines) + "\n"
    skipped = skipped_tests(results)
    counts = {state: sum(r["status"] == state for r in results) for state in ("passed", "failed", "skipped")}
    lines += [
        f"{len(results)} test results: {counts['passed']} passed, {counts['failed']} failed, "
        f"{counts['skipped']} skipped.",
        "",
    ]
    if not rows and not missing and not skipped:
        lines.append("All test results passed.")
        return "\n".join(lines) + "\n"
    if rows:
        deterministic = sum(row["scope"] == "all versions" for row in rows)
        lines += [
            f"{len(rows)} failing tests: {deterministic} fail on all versions they ran on "
            f"(likely deterministic), {len(rows) - deterministic} on some versions "
            "(version-specific or flaky).",
            "",
            "| Test | Suite | Scope | Failing Kubernetes versions | First error |",
            "| --- | --- | --- | --- | --- |",
        ]
        for row in rows:
            versions = ", ".join(link(k8s, row["links"][k8s]) for k8s in row["failed"])
            error = cell(row["error"])
            if row["step"]:
                error = f"{cell(row['step'])}: {error}"
            lines.append(
                f"| {cell(row['test'])} | {cell(row['suite'])} | {row['scope']} "
                f"({len(row['failed'])}/{len(row['ran'])}) | {versions} | {error} |")
        lines.append("")
    if missing:
        lines += [
            "These jobs failed without a test report, for example while creating the "
            "cluster or installing Kyverno. Check their logs:",
            "",
        ]
        for meta in sorted(missing, key=lambda m: (m.get("suite", ""), m.get("variant", ""),
                                                   version_key(m.get("k8s", "")))):
            label = (f"{suite_label(meta.get('suite', ''), meta.get('variant', ''))} "
                     f"on {meta.get('k8s')}, shard {meta.get('shard')}")
            lines.append(f"- {link(cell(label), meta.get('jobUrl', ''))}")
        lines.append("")
    if skipped:
        lines.append(f"Tests skipped on at least one version: {len(skipped)}.")
        if len(skipped) <= MAX_SKIPPED_NAMES:
            lines += [""] + [f"- {cell(label)}: {', '.join(versions)}" for label, versions in skipped.items()]
        else:
            lines.append("Find them in the `results.xml` and `results.json` files of the `e2e-reports-*` artifacts.")
        lines.append("")
    if run_url:
        lines.append(
            f"A failing job that deploys Kyverno also uploads a `diagnostics-*` artifact with "
            f"controller logs, events, webhooks, policies, and reports. Find it in the [run]({run_url}). "
            "The `cli` suite runs the Kyverno CLI without deploying Kyverno, so it has no diagnostics.")
    return "\n".join(lines) + "\n"


def summarize(args):
    results, missing = collect(args.dir)
    sys.stdout.write(markdown(group(results), missing, results, args.run_url))


def counts(args):
    directory = Path(args.dir)
    results, missing = collect(directory) if directory.is_dir() else ([], [])
    totals = {state: sum(r["status"] == state for r in results) for state in ("passed", "failed", "skipped")}
    totals["missing"] = len(missing)
    # Jobs whose chainsaw report was read, even one with no tests in it.
    totals["reports"] = len({*directory.glob(f"{REPORT}"), *directory.glob(f"*/{REPORT}")}) \
        if directory.is_dir() else 0
    sys.stdout.write(json.dumps(totals, sort_keys=True) + "\n")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    commands = parser.add_subparsers(dest="command", required=True)
    command = commands.add_parser("prepare")
    command.add_argument("--dir", required=True)
    command.add_argument("--suite", required=True)
    command.add_argument("--variant", default="")
    command.add_argument("--k8s", required=True)
    command.add_argument("--shard", default="0")
    command.add_argument("--outcome", default="")
    command.add_argument("--job-url", default="")
    command.set_defaults(run=prepare)
    command = commands.add_parser("summarize")
    command.add_argument("dir")
    command.add_argument("--run-url", default="")
    command.set_defaults(run=summarize)
    command = commands.add_parser("counts")
    command.add_argument("dir")
    command.set_defaults(run=counts)
    args = parser.parse_args(argv)
    args.run(args)


if __name__ == "__main__":
    main()
