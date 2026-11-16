#!/usr/bin/env python3
"""Chainsaw test reports of the conformance jobs.

Usage: conformance_report.py prepare <directory>
       conformance_report.py summarize <directory>

prepare runs in every conformance job, after chainsaw. Chainsaw writes one
report format per run, so this builds the JUnit report (results.xml) from the
JSON one (results.json). It also writes job.json, which says which job the
report belongs to. It reads the REPORT_* variables set by action.yaml.

summarize runs once per Tests run. It reads the reports of all conformance
jobs under <directory>, one sub-directory per artifact, and adds one table to
the job summary: one row per failed test, with the Kubernetes versions it
failed on. One broken test is then one row, not one red job per version.

Uses the standard library only. Tests: test_conformance_report.py.
"""

import argparse
from datetime import datetime
import html
import json
import os
from pathlib import Path
import posixpath
import re
import sys
import time
import urllib.request
from xml.etree import ElementTree

# job.json fields and the variables action.yaml sets them from.
JOB_FIELDS = {
    "job": "REPORT_JOB",
    "job-id": "REPORT_JOB_ID",
    "job-index": "REPORT_JOB_INDEX",
    "k8s-version": "REPORT_K8S_VERSION",
    "tests-path": "REPORT_TESTS_PATH",
    "shard-index": "REPORT_SHARD_INDEX",
    "shard-count": "REPORT_SHARD_COUNT",
    "run-attempt": "GITHUB_RUN_ATTEMPT",
}

FAILED, PASSED, SKIPPED = "failed", "passed", "skipped"

ALL = "all versions"
SOME = "some versions (possibly flaky)"
# The test failed wherever it ran, but some versions have no result for it:
# their job didn't report, or chainsaw stopped at an earlier failure.
RAN = "all versions that ran it"

# The job summary is limited to 1 MiB per step, so long lists are cut.
MAX_ROWS = 100
MAX_JOBS = 30
MAX_ERROR = 200
# Reading a job by its id takes one API call per job.
MAX_LOOKUPS = 20


def failures(test):
    """The failed operations of a test, in the order they ran."""
    return [
        {"step": step.get("name", ""), "type": operation.get("type", ""),
         "error": (operation.get("failure") or {}).get("error", "")}
        for step in test.get("steps") or []
        for operation in step.get("operations") or []
        if operation.get("failure")
    ]


def status(test):
    if test.get("skipped"):
        return SKIPPED
    return FAILED if failures(test) else PASSED


def test_path(tests_path, base_path):
    """The test directory relative to test/conformance/chainsaw, as in quarantine.yaml."""
    parts = [part for part in (tests_path, base_path) if part not in ("", ".", None)]
    return posixpath.normpath("/".join(parts)) if parts else "."


def seconds(item):
    """The duration of a report item, in seconds."""
    def parse(value):
        # Go prints up to nine fractional digits, more than strptime reads.
        match = re.fullmatch(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(\.\d+)?Z", str(value))
        if not match:
            raise ValueError(value)
        whole = datetime.strptime(match.group(1), "%Y-%m-%dT%H:%M:%S")
        return (whole - datetime(1970, 1, 1)).total_seconds() + float(match.group(2) or 0)
    try:
        return max(parse(item.get("endTime")) - parse(item.get("startTime")), 0.0)
    except ValueError:
        return 0.0


def junit(report, tests_path=""):
    """The report as JUnit XML, laid out like chainsaw's JUNIT-TEST format:
    one test suite per directory and one test case per test."""
    suites = {}
    for test in report.get("tests") or []:
        suites.setdefault(test.get("basePath", ""), []).append(test)
    root = ElementTree.Element("testsuites", name=report.get("name", ""), time=f"{seconds(report):.6f}")
    totals = {FAILED: 0, PASSED: 0, SKIPPED: 0}
    for index, (base_path, tests) in enumerate(suites.items()):
        path = test_path(tests_path, base_path)
        counts = {FAILED: 0, PASSED: 0, SKIPPED: 0}
        suite = ElementTree.SubElement(root, "testsuite", name=path, id=str(index))
        for test in tests:
            case = ElementTree.SubElement(
                suite, "testcase", name=test.get("name", ""), classname=path, time=f"{seconds(test):.6f}")
            result = status(test)
            counts[result] += 1
            if result == SKIPPED:
                ElementTree.SubElement(case, "skipped", message="")
            elif result == FAILED:
                message = "; ".join(failure["error"] for failure in failures(test))
                ElementTree.SubElement(case, "failure", message=message)
        suite.set("tests", str(len(tests)))
        suite.set("failures", str(counts[FAILED]))
        suite.set("errors", "0")
        suite.set("skipped", str(counts[SKIPPED]))
        suite.set("time", f"{sum(seconds(test) for test in tests):.6f}")
        for result, number in counts.items():
            totals[result] += number
    root.set("tests", str(sum(totals.values())))
    root.set("failures", str(totals[FAILED]))
    root.set("skipped", str(totals[SKIPPED]))
    ElementTree.indent(root)
    return ElementTree.tostring(root, encoding="utf-8", xml_declaration=True)


def prepare(directory, env):
    """Writes job.json and, when chainsaw wrote its report, results.xml."""
    directory = Path(directory)
    directory.mkdir(parents=True, exist_ok=True)
    job = {field: env.get(variable, "") for field, variable in JOB_FIELDS.items()}
    (directory / "job.json").write_text(json.dumps(job, indent=2, sort_keys=True) + "\n")
    results = directory / "results.json"
    if not results.is_file():
        print(f"No {results}: chainsaw didn't run or didn't finish, so there is no JUnit report.")
        return
    try:
        report = json.loads(results.read_text())
        (directory / "results.xml").write_bytes(junit(report, job["tests-path"]))
    except (ValueError, AttributeError, TypeError) as error:
        print(f"::warning::Can't build the JUnit report from {results}: {error}")


def load(root):
    """Reads the reports under root. A report without tests comes from a job
    that failed or was cancelled before chainsaw wrote its results."""
    reports = []
    for path in sorted(Path(root).rglob("job.json")):
        try:
            job = json.loads(path.read_text())
            attempt = int(job.get("run-attempt") or 1)
        except (ValueError, AttributeError, TypeError) as error:
            print(f"::warning::Skipped the report in {path.parent.name}: can't read job.json: {error}")
            continue
        tests = None
        results = path.with_name("results.json")
        if results.is_file():
            # A job that was cancelled while chainsaw wrote the file leaves it cut off.
            try:
                tests = [test for test in json.loads(results.read_text()).get("tests") or []
                         if isinstance(test, dict)]
            except (ValueError, AttributeError, TypeError) as error:
                print(f"::warning::Can't read the test results in {path.parent.name}: {error}")
        reports.append({
            "job": str(job.get("job", "")), "k8s": str(job.get("k8s-version", "")),
            "index": str(job.get("job-index", "")), "attempt": attempt,
            "job-id": str(job.get("job-id", "")), "tests-path": str(job.get("tests-path", "")),
            "shard-index": str(job.get("shard-index", "")), "shard-count": str(job.get("shard-count", "")),
            "tests": tests,
        })
    return reports


def latest(reports):
    """Splits the reports into the latest one of each job and the earlier ones.

    Re-running the failed jobs of a run keeps the reports of the earlier
    attempt, and the jobs that passed aren't run again. So the state of a run
    is the latest report of each job, whichever attempt wrote it.
    """
    newest, earlier = {}, []
    for report in sorted(reports, key=lambda report: report["attempt"]):
        key = (report["job"], report["k8s"], report["index"])
        if key in newest:
            earlier.append(newest[key])
        newest[key] = report
    return list(newest.values()), earlier


def results(report):
    """Yields (test, status, first failure) for each test of a report."""
    for test in report["tests"] or []:
        name = test.get("name", "")
        key = (test_path(report["tests-path"], test.get("basePath", "")), name)
        yield key, status(test), next(iter(failures(test)), None)


def version_key(version):
    return [int(number) for number in re.findall(r"\d+", version)]


def group(reports):
    """Groups the test results by test. Returns the rows of failed tests and
    the number of failed, passed and skipped tests."""
    order = {SKIPPED: 0, PASSED: 1, FAILED: 2}
    tests, versions = {}, {}
    for report in reports:
        versions.setdefault(report["job"], set()).add(report["k8s"])
        for key, result, failure in results(report):
            entry = tests.setdefault(key, {"job": report["job"], "versions": {}})
            current = entry["versions"].get(report["k8s"])
            if current is None or order[result] > order[current["status"]]:
                entry["versions"][report["k8s"]] = {"status": result, "failure": failure, "report": report}
    paths = {}
    for path, name in tests:
        paths.setdefault(path, set()).add(name)
    rows, totals = [], {FAILED: 0, PASSED: 0, SKIPPED: 0}
    for (path, name), entry in tests.items():
        by_status = {result: sorted((version for version, item in entry["versions"].items()
                                     if item["status"] == result), key=version_key)
                     for result in order}
        if not by_status[FAILED]:
            totals[PASSED if by_status[PASSED] else SKIPPED] += 1
            continue
        totals[FAILED] += 1
        missing = sorted(versions[entry["job"]] - set(by_status[FAILED]) - set(by_status[PASSED]),
                         key=version_key)
        scope = SOME if by_status[PASSED] else RAN if missing else ALL
        first = entry["versions"][by_status[FAILED][0]]["failure"]
        rows.append({
            # Two tests may share a directory; then the name tells them apart.
            "test": path if len(paths[path]) == 1 else f"{path} ({name})",
            "failed": by_status[FAILED], "passed": by_status[PASSED], "missing": missing, "scope": scope,
            "error": f"{first['step']} / {first['type']}: {first['error']}",
            "reports": [entry["versions"][version]["report"] for version in by_status[FAILED]],
        })
    scopes = [ALL, RAN, SOME]
    rows.sort(key=lambda row: (scopes.index(row["scope"]), row["test"]))
    return rows, totals


def flaky(newest, earlier):
    """Tests that failed on an earlier attempt and passed when their job was re-run."""
    passed = {}
    for report in newest:
        key = (report["job"], report["k8s"], report["index"])
        passed[key] = {test for test, result, _ in results(report) if result == PASSED}
    found = set()
    for report in earlier:
        key = (report["job"], report["k8s"], report["index"])
        for test, result, _ in results(report):
            if result == FAILED and test in passed.get(key, ()):
                found.add((test[0], report["k8s"]))
    return sorted(found, key=lambda item: (item[0], version_key(item[1])))


def api_url(env):
    return f"{env.get('GITHUB_API_URL', 'https://api.github.com')}/repos/{env['GITHUB_REPOSITORY']}/actions"


def api_get(url, env, end, opener=None):
    """Reads a GitHub API URL as JSON. Returns None after three failed tries,
    or once the time is past end."""
    headers = {"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"}
    if env.get("GITHUB_TOKEN"):
        headers["Authorization"] = f"Bearer {env['GITHUB_TOKEN']}"
    request = urllib.request.Request(url, headers=headers)
    for retry in range(3):
        if time.monotonic() > end:
            return None
        try:
            with (opener or urllib.request.urlopen)(request, timeout=30) as response:
                return json.load(response)
        except (OSError, ValueError) as error:
            print(f"Reading {url} failed: {error}")
            if retry < 2:
                time.sleep(2 ** (retry + 1))
    return None


def fetch_jobs(env, deadline=240, opener=None):
    """Lists the jobs of this attempt, or returns None when the API can't be read.

    Small pages: a page of 100 jobs times out at GitHub's gateway on runs with
    hundreds of jobs, and a page of 25 takes about five seconds. The deadline
    keeps a slow API from using up the job's timeout.
    """
    end = time.monotonic() + deadline
    base = f"{api_url(env)}/runs/{env['GITHUB_RUN_ID']}/attempts/{env.get('GITHUB_RUN_ATTEMPT', '1')}/jobs"
    jobs, page = [], 1
    while True:
        data = api_get(f"{base}?per_page=25&page={page}", env, end, opener)
        if data is None:
            return None
        jobs += data.get("jobs") or []
        if not data.get("jobs") or len(jobs) >= data.get("total_count", 0):
            return jobs
        page += 1


def find_job(report, jobs):
    """The job that wrote a report: by the id it recorded, else by its name.

    Only names that GitHub derives from the matrix can be rebuilt. A job with
    its own name, such as "v1.36.4 - cert-manager rsa", is found by id, or by
    the name that find_names read for it.
    """
    for job in jobs or []:
        if report["job-id"] and str(job.get("id")) == report["job-id"]:
            return job
    name = f"{report['job']} ({report['k8s']})"
    if report["shard-count"] not in ("", "0"):
        name = f"{report['job']} ({report['k8s']}, {report['shard-index']})"
    for job in jobs or []:
        # The jobs of a called workflow are named "<calling job> / <job>".
        full = str(job.get("name", ""))
        if full.split(" / ")[-1] == name or (report.get("job-name") and full == report["job-name"]):
            return job
    return None


def has_failures(report):
    return any(result == FAILED for _, result, _ in results(report))


def find_names(reports, jobs, env, deadline=60, opener=None):
    """Reads the name of the job behind a failed report of an earlier attempt.

    A re-run lists every job under a new id, so such a report is matched to
    its job by name. The name of a job that sets its own can't be rebuilt, so
    it is read from the job the report recorded: one call per report, up to
    MAX_LOOKUPS. When a call fails, the reports are left as they are.
    """
    attempt = int(env.get("GITHUB_RUN_ATTEMPT") or 1)
    end = time.monotonic() + deadline
    unnamed = [report for report in reports
               if report["attempt"] < attempt and has_failures(report)
               and re.fullmatch(r"\d+", report["job-id"]) and not find_job({**report, "job-id": ""}, jobs)]
    for report in unnamed[:MAX_LOOKUPS]:
        job = api_get(f"{api_url(env)}/jobs/{report['job-id']}", env, end, opener)
        if not isinstance(job, dict):
            return
        report["job-name"] = str(job.get("name", ""))


def log_url(report, jobs, run_url):
    """The log of the job that wrote a report, or the run when the job is unknown."""
    if re.fullmatch(r"\d+", report["job-id"]):
        return f"{run_url}/job/{report['job-id']}"
    job = find_job(report, jobs)
    return job["html_url"] if job and job.get("html_url") else run_url


def superseded(reports, jobs, attempt):
    """Reports with failures from an earlier attempt whose job has passed since.

    That happens when the re-run of a job passes without uploading a report.
    The job has a new id in the re-run, so it is found by name.
    """
    stale = []
    for report in reports:
        if report["attempt"] >= attempt or not has_failures(report):
            continue
        job = find_job({**report, "job-id": ""}, jobs)
        if job and job.get("conclusion") == "success":
            stale.append(report)
    return stale


def other_failures(jobs, reports):
    """Failed jobs that no failed test explains: they failed outside a test, for
    example in setup or on a timeout, or they aren't conformance jobs."""
    by_job = {}
    for report in reports:
        job = find_job(report, jobs)
        if job:
            by_job[job.get("id")] = report
    rows = []
    for job in jobs or []:
        if job.get("conclusion") not in ("failure", "timed_out"):
            continue
        report = by_job.get(job.get("id"))
        if report and has_failures(report):
            continue
        step = next((step.get("name", "") for step in job.get("steps") or []
                     if step.get("conclusion") == "failure"), "")
        rows.append({
            "name": job.get("name", ""), "url": job.get("html_url", ""), "conclusion": job["conclusion"],
            "step": step, "report": "no failed test" if report and report["tests"] is not None else "none",
        })
    return rows


def text(value, limit=None):
    """Escapes text for a table cell. Test names and errors are whatever the
    tests print, so Markdown, HTML and mentions in them must not take effect."""
    value = " ".join(str(value).split())
    if limit and len(value) > limit:
        value = value[:limit - 1].rstrip() + "…"
    value = html.escape(value, quote=False)
    return re.sub(r"[|`*_~\[\]\\@#]", lambda match: f"&#{ord(match.group())};", value)


def count(number, noun):
    return f"{number} {noun}" if number == 1 else f"{number} {noun}s"


def render(reports, jobs, env):
    """The Markdown summary of a run."""
    attempt = int(env.get("GITHUB_RUN_ATTEMPT") or 1)
    run_url = (f"{env.get('GITHUB_SERVER_URL', 'https://github.com')}/{env.get('GITHUB_REPOSITORY', '')}"
               f"/actions/runs/{env.get('GITHUB_RUN_ID', '')}")
    newest, earlier = latest(reports)
    stale = superseded(newest, jobs, attempt)
    newest = [report for report in newest if report not in stale]
    earlier += stale
    rows, totals = group(newest)
    empty = sum(report["tests"] is None for report in newest)
    lines = ["## Conformance test summary", ""]

    if env.get("REPORTS_DOWNLOAD") == "failure":
        lines += ["Downloading the test reports failed, so reports may be missing below.", ""]
    if len(newest) == empty:
        lines += ["No conformance job uploaded test results, so there is nothing to group. "
                  "The conformance jobs didn't start, or didn't get to the end of their tests.", ""]
    elif not rows:
        lines += [f"No test failed in the {count(len(newest) - empty, 'job report')} with test results.", ""]
    else:
        by_scope = [f"{sum(row['scope'] == scope for row in rows)} on {scope}" for scope in (ALL, RAN, SOME)
                    if any(row["scope"] == scope for row in rows)]
        lines += [
            f"**{count(len(rows), 'test')} failed**: {', '.join(by_scope)}.", "",
            "| Test | Failing Kubernetes versions | Scope | First error | Logs |",
            "| --- | --- | --- | --- | --- |",
        ]
        for row in rows[:MAX_ROWS]:
            scope = row["scope"]
            if row["passed"]:
                scope += f"<br>passed on {', '.join(row['passed'])}"
            if row["missing"]:
                scope += f"<br>no result on {', '.join(row['missing'])}"
            logs = " ".join(f"[{report['k8s']}]({log_url(report, jobs, run_url)})" for report in row["reports"])
            lines.append(f"| <code>{text(row['test'])}</code> | {', '.join(row['failed'])} | {scope} | "
                         f"<code>{text(row['error'], MAX_ERROR)}</code> | {logs} |")
        if len(rows) > MAX_ROWS:
            lines += ["", f"Only the first {MAX_ROWS} failed tests are listed."]
        lines += ["", "The first error is the one from the lowest failing version.", ""]

    if jobs is None:
        lines += [f"The job list of this run couldn't be read, so failed jobs without a test report "
                  f"aren't listed, and a log link may point to [the run]({run_url}).", ""]
    others = other_failures(jobs, newest)
    if others:
        lines += [
            "### Failed jobs without a failed test", "",
            "These jobs failed outside a chainsaw test, for example in setup or on a timeout, "
            "or they aren't conformance jobs.", "",
            "| Job | Result | Failed step | Test report |", "| --- | --- | --- | --- |",
        ]
        lines += [f"| [{text(job['name'])}]({job['url']}) | {job['conclusion']} | "
                  f"{text(job['step']) or '—'} | {job['report']} |" for job in others[:MAX_JOBS]]
        if len(others) > MAX_JOBS:
            lines += ["", f"Only the first {MAX_JOBS} of {len(others)} such jobs are listed."]
        lines.append("")

    recovered = flaky(newest, earlier)
    if recovered:
        lines += ["### Failed on an earlier attempt, passed on the re-run", ""]
        lines += [f"- <code>{text(path)}</code> on {version}" for path, version in recovered[:MAX_ROWS]]
        lines.append("")

    if newest:
        versions = sorted({report["k8s"] for report in newest}, key=version_key)
        lines += ["### Totals", "",
                  f"- Tests: {totals[FAILED]} failed, {totals[PASSED]} passed, {totals[SKIPPED]} skipped. "
                  "A skipped test is quarantined, or chainsaw stopped at an earlier failure in the same job.",
                  f"- Reports: {len(newest)}, on Kubernetes {', '.join(versions)}."
                  + (f" {count(empty, 'job')} uploaded no test results." if empty else "")]
        if attempt > 1:
            lines.append(f"- This is attempt {attempt} of the run. Each job counts with its latest report, "
                         "so a job that wasn't re-run counts with the report of its earlier attempt.")
        lines.append("")
    return "\n".join(lines)


def summarize(directory, env):
    """Writes the summary of a run to the job summary, or prints it."""
    try:
        reports = load(directory)
        jobs = fetch_jobs(env) if env.get("GITHUB_RUN_ID") and env.get("GITHUB_REPOSITORY") else None
        if jobs:
            find_names(latest(reports)[0], jobs, env)
        summary = render(reports, jobs, env)
    except Exception as error:
        # The summary is an aid, so a bug here must not show up as a failed job
        # of the run. Say what went wrong instead, here and as an annotation.
        summary = f"## Conformance test summary\n\nThe summary couldn't be built: {type(error).__name__}: {error}\n"
        print(f"::error::The conformance test summary couldn't be built: {type(error).__name__}: {error}")
    if env.get("GITHUB_STEP_SUMMARY"):
        with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as stream:
            stream.write(summary)
    else:
        print(summary)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    parser.add_argument("command", choices=("prepare", "summarize"))
    parser.add_argument("directory", help="the report directory of a job, or the directory with all reports")
    args = parser.parse_args(argv)
    if args.command == "prepare":
        prepare(args.directory, os.environ)
    else:
        summarize(args.directory, os.environ)
    return 0


if __name__ == "__main__":
    sys.exit(main())
