"""Tests conformance_report.py. Needs Python 3 only.

Usage: python3 -B -m unittest discover -s .github/actions/tests/conformance/run -p 'test_*.py'

testdata/ttl.json and testdata/policy-exceptions-disabled.json are chainsaw
reports of real runs; ttl.json is cut down to three of its tests.
testdata/summary.md is the expected summary of the run built in sample_run();
UPDATE_SUMMARY=1 rewrites it.
"""

import contextlib
import copy
import io
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch
from xml.etree import ElementTree

import conformance_report as cr

HERE = Path(__file__).resolve().parent
VERSIONS = ["v1.34.11", "v1.35.8", "v1.36.4", "v1.37.0"]
ENV = {
    "GITHUB_REPOSITORY": "kyverno/kyverno", "GITHUB_RUN_ID": "42", "GITHUB_RUN_ATTEMPT": "1",
    "GITHUB_SERVER_URL": "https://github.com", "GITHUB_API_URL": "https://api.github.com",
}
RUN_URL = "https://github.com/kyverno/kyverno/actions/runs/42"


def fixture(name):
    return json.loads((HERE / "testdata" / name).read_text())


def passing(report, *names):
    """A copy of a report in which the named tests, or all tests, pass."""
    report = copy.deepcopy(report)
    for test in report["tests"]:
        if names and test["name"] not in names:
            continue
        test.pop("skipped", None)
        for step in test.setdefault("steps", [{"name": "step-01", "operations": [{"name": "operation 1"}]}]):
            for operation in step.get("operations", []):
                operation.pop("failure", None)
    return report


def result(name, base_path, error=None, skipped=False):
    """A minimal chainsaw test result."""
    if skipped:
        return {"name": name, "basePath": base_path, "skipped": True}
    operation = {"name": "operation 1", "type": "assert"}
    if error:
        operation["failure"] = {"error": error}
    return {"name": name, "basePath": base_path, "steps": [{"name": "step-01", "operations": [operation]}]}


def api_job(job_id, name, conclusion="failure", step="Run ./.github/actions/tests/conformance/run"):
    steps = [{"name": "Set up job", "conclusion": "success"}]
    if conclusion == "failure":
        steps.append({"name": step, "conclusion": "failure"})
    return {"id": job_id, "name": name, "conclusion": conclusion, "steps": steps,
            "html_url": f"{RUN_URL}/job/{job_id}"}


class Run:
    """Builds the directory the summarize job downloads: one sub-directory per
    artifact, written the way a conformance job writes it."""

    def __init__(self, case):
        self.root = Path(tempfile.mkdtemp())
        case.addCleanup(shutil.rmtree, self.root)

    def add(self, suite, version, report=None, index=0, attempt=1, job_id="", tests_path=None, shard=None):
        directory = self.root / f"conformance-report-{suite}-{version}-{index}-{attempt}"
        directory.mkdir()
        if report is not None:
            (directory / "results.json").write_text(json.dumps(report))
        with contextlib.redirect_stdout(io.StringIO()):
            cr.prepare(directory, {
                "REPORT_JOB": suite, "REPORT_JOB_ID": str(job_id), "REPORT_JOB_INDEX": str(index),
                "REPORT_K8S_VERSION": version, "REPORT_TESTS_PATH": tests_path or suite,
                "REPORT_SHARD_INDEX": str(shard[0]) if shard else "0",
                "REPORT_SHARD_COUNT": str(shard[1]) if shard else "0",
                "GITHUB_RUN_ATTEMPT": str(attempt),
            })
        return directory

    def rows(self):
        return cr.group(cr.latest(cr.load(self.root))[0])

    def render(self, jobs=None, attempt=1):
        return cr.render(cr.load(self.root), jobs, {**ENV, "GITHUB_RUN_ATTEMPT": str(attempt)})


def sample_run(case):
    """A run with one test that fails on every version, one that fails on one
    version, one whose job has no report on one version, and two jobs that
    failed outside a test."""
    run, jobs = Run(case), [api_job(1, "Build test images", "success")]
    ttl, exceptions = fixture("ttl.json"), fixture("policy-exceptions-disabled.json")
    for index, version in enumerate(VERSIONS):
        run.add("ttl", version, ttl if version == "v1.35.8" else passing(ttl, "past-timestamp"),
                index=index, job_id=101 + index)
        jobs.append(api_job(101 + index, f"Conformance tests / ttl ({version})"))
        # These reports record no job id, so their jobs are found by name.
        run.add("policy-exceptions-disabled", version, None if version == "v1.36.4" else exceptions, index=index)
        jobs.append(api_job(201 + index, f"Conformance tests / policy-exceptions-disabled ({version})"))
    jobs.append(api_job(301, "K6 tests / load (v1.37.0)", step="Run k6"))
    return run, jobs


class Reports(unittest.TestCase):
    def test_reads_failed_passed_and_skipped_tests_of_a_real_report(self):
        ttl = {item["name"]: item for item in fixture("ttl.json")["tests"]}
        self.assertEqual(cr.status(ttl["delete-twice"]), cr.FAILED)
        self.assertEqual(cr.status(ttl["valid-label"]), cr.PASSED)
        self.assertEqual(cr.failures(ttl["delete-twice"]), [
            {"step": "step-02", "type": "error",
             "error": "v1/Pod/chainsaw-bright-loon/test-pod - resource matches expectation"},
            {"step": "cleanup (step-01)", "type": "delete", "error": "context deadline exceeded"},
        ])
        skipped = fixture("policy-exceptions-disabled.json")["tests"][1]
        self.assertEqual(cr.status(skipped), cr.SKIPPED)

    def test_test_path_is_relative_to_the_chainsaw_directory(self):
        self.assertEqual(cr.test_path("ttl", "delete-twice"), "ttl/delete-twice")
        self.assertEqual(cr.test_path("cel/http", "a/b"), "cel/http/a/b")
        self.assertEqual(cr.test_path("tls-certificates/rsa-cert-manager", "."), "tls-certificates/rsa-cert-manager")
        self.assertEqual(cr.test_path(".", "a"), "a")
        self.assertEqual(cr.test_path(".", "."), ".")

    def test_junit_report_matches_the_json_report(self):
        root = ElementTree.fromstring(cr.junit(fixture("ttl.json"), "ttl"))
        self.assertEqual(root.tag, "testsuites")
        self.assertEqual((root.get("name"), root.get("tests"), root.get("failures"), root.get("skipped")),
                         ("chainsaw-report", "3", "2", "0"))
        cases = {case.get("classname"): case for case in root.iter("testcase")}
        self.assertEqual(sorted(cases), ["ttl/delete-twice", "ttl/past-timestamp", "ttl/valid-label"])
        self.assertEqual(
            cases["ttl/delete-twice"].find("failure").get("message"),
            "v1/Pod/chainsaw-bright-loon/test-pod - resource matches expectation; context deadline exceeded")
        self.assertEqual(len(cases["ttl/valid-label"]), 0)
        self.assertGreater(float(cases["ttl/valid-label"].get("time")), 0)

        root = ElementTree.fromstring(cr.junit(fixture("policy-exceptions-disabled.json"), "exceptions"))
        self.assertEqual((root.get("tests"), root.get("failures"), root.get("skipped")), ("2", "1", "1"))
        suites = {suite.get("name"): suite for suite in root.iter("testsuite")}
        skipped = suites["exceptions/validating-policies"]
        self.assertEqual((skipped.get("tests"), skipped.get("failures"), skipped.get("skipped")), ("1", "0", "1"))
        self.assertIsNotNone(skipped.find("testcase/skipped"))

    def test_junit_report_of_an_empty_run(self):
        root = ElementTree.fromstring(cr.junit({}))
        self.assertEqual((root.get("tests"), root.get("failures"), len(root)), ("0", "0", 0))

    def test_prepare_writes_the_job_and_the_junit_report(self):
        directory = Run(self).add("deleting-policies", "v1.36.4", fixture("ttl.json"), index=17, attempt=2,
                                  job_id=132233564933, shard=(5, 6))
        self.assertEqual(json.loads((directory / "job.json").read_text()), {
            "job": "deleting-policies", "job-id": "132233564933", "job-index": "17", "k8s-version": "v1.36.4",
            "run-attempt": "2", "shard-count": "6", "shard-index": "5", "tests-path": "deleting-policies",
        })
        self.assertEqual(ElementTree.parse(directory / "results.xml").getroot().get("failures"), "2")

    def test_prepare_without_a_chainsaw_report_still_records_the_job(self):
        with tempfile.TemporaryDirectory() as root:
            # The job failed before chainsaw, so the directory doesn't exist yet.
            directory = Path(root) / "conformance-results"
            with contextlib.redirect_stdout(io.StringIO()):
                cr.prepare(directory, {"REPORT_JOB": "ttl", "REPORT_K8S_VERSION": "v1.36.4"})
            self.assertEqual(sorted(path.name for path in directory.iterdir()), ["job.json"])
            self.assertEqual(json.loads((directory / "job.json").read_text())["job"], "ttl")

    def test_cut_off_results_count_as_no_results(self):
        run = Run(self)
        directory = run.add("ttl", "v1.36.4")
        (directory / "results.json").write_text('{"name": "chainsaw-report", "tests": [{"na')
        with patch("sys.stdout", io.StringIO()):
            cr.prepare(directory, {"REPORT_JOB": "ttl", "REPORT_K8S_VERSION": "v1.36.4"})
            reports = cr.load(run.root)
        self.assertFalse((directory / "results.xml").exists())
        self.assertEqual([(report["job"], report["tests"]) for report in reports], [("ttl", None)])

    def test_a_single_report_is_read_from_the_download_directory_itself(self):
        # download-artifact puts a lone artifact straight into its path, and
        # several artifacts into one sub-directory each.
        run = Run(self)
        directory = run.add("ttl", "v1.36.4", fixture("ttl.json"))
        for path in directory.iterdir():
            path.rename(run.root / path.name)
        directory.rmdir()
        self.assertEqual([(report["job"], len(report["tests"])) for report in cr.load(run.root)], [("ttl", 3)])

    def test_action_sets_every_variable_the_script_reads(self):
        action = (HERE / "action.yaml").read_text()
        for variable in cr.JOB_FIELDS.values():
            if variable.startswith("REPORT_"):
                self.assertIn(f"        {variable}: ${{{{", action)
        self.assertIn('"$GITHUB_ACTION_PATH/conformance_report.py" prepare', action)


class Grouping(unittest.TestCase):
    def test_a_test_failing_on_every_version_is_one_row(self):
        run = Run(self)
        for index, version in enumerate(VERSIONS):
            run.add("ttl", version, passing(fixture("ttl.json"), "past-timestamp"), index=index)
        rows, totals = run.rows()
        self.assertEqual([(row["test"], row["failed"], row["scope"]) for row in rows],
                         [("ttl/delete-twice", VERSIONS, cr.ALL)])
        self.assertEqual(rows[0]["error"],
                         "step-02 / error: v1/Pod/chainsaw-bright-loon/test-pod - resource matches expectation")
        self.assertEqual(totals, {cr.FAILED: 1, cr.PASSED: 2, cr.SKIPPED: 0})

    def test_a_test_failing_on_some_versions_is_possibly_flaky(self):
        run = Run(self)
        for index, version in enumerate(VERSIONS):
            report = fixture("ttl.json") if version == "v1.35.8" else passing(fixture("ttl.json"))
            run.add("ttl", version, report, index=index)
        rows, _ = run.rows()
        self.assertEqual([(row["test"], row["failed"], row["scope"]) for row in rows], [
            ("ttl/delete-twice", ["v1.35.8"], cr.SOME), ("ttl/past-timestamp", ["v1.35.8"], cr.SOME)])
        self.assertEqual(rows[0]["passed"], ["v1.34.11", "v1.36.4", "v1.37.0"])

    def test_versions_without_a_result_are_named(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a", "boom"), result("b", "b", skipped=True)]})
        # Chainsaw stopped at another failure before it reached test a.
        run.add("suite", "v1.35.8", {"tests": [result("b", "b", "bang"), result("a", "a", skipped=True)]}, index=1)
        # The job was cancelled, so it has no results at all.
        run.add("suite", "v1.36.4", None, index=2)
        rows, totals = run.rows()
        self.assertEqual([(row["test"], row["failed"], row["scope"], row["missing"]) for row in rows], [
            ("suite/a", ["v1.34.11"], cr.RAN, ["v1.35.8", "v1.36.4"]),
            ("suite/b", ["v1.35.8"], cr.RAN, ["v1.34.11", "v1.36.4"]),
        ])
        self.assertEqual(totals, {cr.FAILED: 2, cr.PASSED: 0, cr.SKIPPED: 0})

    def test_quarantined_tests_are_skipped_not_failed_or_passed(self):
        run = Run(self)
        for index, version in enumerate(VERSIONS):
            run.add("suite", version, {"tests": [result("a", "a"), result("b", "b", skipped=True)]}, index=index)
        self.assertEqual(run.rows(), ([], {cr.FAILED: 0, cr.PASSED: 1, cr.SKIPPED: 1}))

    def test_shards_of_a_suite_share_its_versions(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a", "boom")]}, index=0, shard=(0, 2))
        run.add("suite", "v1.34.11", {"tests": [result("b", "b")]}, index=1, shard=(1, 2))
        run.add("suite", "v1.35.8", {"tests": [result("a", "a", "boom")]}, index=2, shard=(0, 2))
        run.add("suite", "v1.35.8", {"tests": [result("b", "b")]}, index=3, shard=(1, 2))
        rows, _ = run.rows()
        self.assertEqual([(row["test"], row["scope"]) for row in rows], [("suite/a", cr.ALL)])

    def test_tests_with_the_same_name_in_two_directories_stay_apart(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("same", "one", "boom"), result("same", "two")]})
        run.add("suite", "v1.35.8", {"tests": [result("same", "one", "boom"), result("same", "two")]}, index=1)
        rows, totals = run.rows()
        self.assertEqual([(row["test"], row["scope"]) for row in rows], [("suite/one", cr.ALL)])
        self.assertEqual(totals[cr.PASSED], 1)

    def test_two_tests_in_one_directory_are_told_apart_by_name(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("first", "dir", "boom"), result("second", "dir", "bang")]})
        self.assertEqual([row["test"] for row in run.rows()[0]], ["suite/dir (first)", "suite/dir (second)"])

    def test_first_error_comes_from_the_lowest_failing_version(self):
        run = Run(self)
        run.add("suite", "v1.35.8", {"tests": [result("a", "a", "later")]}, index=1)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a", "first")]}, index=0)
        self.assertEqual(run.rows()[0][0]["error"], "step-01 / assert: first")

    def test_a_failure_wins_when_two_jobs_run_a_test_on_one_version(self):
        for order in ((0, 1), (1, 0)):
            run = Run(self)
            run.add("suite", "v1.34.11", {"tests": [result("a", "a", "boom")]}, index=order[0])
            run.add("suite", "v1.34.11", {"tests": [result("a", "a")]}, index=order[1])
            rows, _ = run.rows()
            self.assertEqual([(row["failed"], row["passed"]) for row in rows], [(["v1.34.11"], [])])

    def test_versions_sort_by_number(self):
        run = Run(self)
        for index, version in enumerate(["v1.34.11", "v1.34.9", "v1.100.0"]):
            run.add("suite", version, {"tests": [result("a", "a", "boom")]}, index=index)
        self.assertEqual(run.rows()[0][0]["failed"], ["v1.34.9", "v1.34.11", "v1.100.0"])


class Attempts(unittest.TestCase):
    def test_the_latest_report_of_each_job_counts(self):
        run = Run(self)
        # Attempt 1: v1.34.11 passes, v1.35.8 and v1.36.4 fail. Attempt 2
        # re-runs only the failed jobs: v1.35.8 passes, v1.36.4 fails again.
        run.add("suite", "v1.34.11", {"tests": [result("a", "a")]}, index=0)
        run.add("suite", "v1.35.8", {"tests": [result("a", "a", "boom")]}, index=1)
        run.add("suite", "v1.36.4", {"tests": [result("a", "a", "boom")]}, index=2)
        run.add("suite", "v1.35.8", {"tests": [result("a", "a")]}, index=1, attempt=2)
        run.add("suite", "v1.36.4", {"tests": [result("a", "a", "again")]}, index=2, attempt=2)
        newest, earlier = cr.latest(cr.load(run.root))
        self.assertEqual(sorted((report["k8s"], report["attempt"]) for report in newest),
                         [("v1.34.11", 1), ("v1.35.8", 2), ("v1.36.4", 2)])
        rows, _ = cr.group(newest)
        self.assertEqual([(row["failed"], row["passed"], row["error"]) for row in rows],
                         [(["v1.36.4"], ["v1.34.11", "v1.35.8"], "step-01 / assert: again")])
        self.assertEqual(cr.flaky(newest, earlier), [("suite/a", "v1.35.8")])
        summary = run.render(attempt=2)
        self.assertIn("### Failed on an earlier attempt, passed on the re-run", summary)
        self.assertIn("- <code>suite/a</code> on v1.35.8", summary)
        self.assertIn("This is attempt 2 of the run.", summary)

    def test_a_failed_report_is_dropped_once_its_job_passed_without_a_report(self):
        run = Run(self)
        run.add("suite", "v1.35.8", {"tests": [result("a", "a", "boom")]}, job_id=7)
        # The re-run has a new job id, and a job that isn't re-run is listed
        # under a new id too, so only the name tells which job this is.
        jobs = [api_job(8, "Conformance tests / suite (v1.35.8)", "success")]
        self.assertNotIn("| Test |", run.render(jobs, attempt=2))
        jobs = [api_job(8, "Conformance tests / suite (v1.35.8)", "failure")]
        self.assertIn("| <code>suite/a</code> | v1.35.8 |", run.render(jobs, attempt=2))
        self.assertIn("| <code>suite/a</code> | v1.35.8 |", run.render(None, attempt=2))


class Names(unittest.TestCase):
    """A job that sets its own name: its name can't be rebuilt from the matrix,
    so after a re-run it is read from the job that the report recorded."""

    NAME = "Conformance tests / v1.36.4 - cert-manager rsa"
    ENV = {**ENV, "GITHUB_RUN_ATTEMPT": "2"}
    ROW = "| <code>tls-certificates/rsa-cert-manager</code> | v1.36.4 |"

    def run_with_old_failures(self):
        run = Run(self)
        run.add("cert-manager-certificates", "v1.36.4", {"tests": [result("rsa", ".", "boom")]}, job_id=5,
                tests_path="tls-certificates/rsa-cert-manager")
        run.add("suite", "v1.36.4", {"tests": [result("a", "a", "boom")]}, index=1, job_id=6)
        return run

    def opener(self, urls):
        def opener(request, timeout):
            urls.append(request.full_url)
            return io.BytesIO(json.dumps({"id": 5, "name": self.NAME}).encode())
        return opener

    def test_a_stale_failure_of_a_job_with_its_own_name_is_dropped(self):
        reports, urls = cr.load(self.run_with_old_failures().root), []
        jobs = [api_job(9, self.NAME, "success"), api_job(10, "Conformance tests / suite (v1.36.4)")]
        # Without the name the job isn't found, so its old failure stays.
        self.assertIn(self.ROW, cr.render(reports, jobs, self.ENV))
        cr.find_names(reports, jobs, self.ENV, opener=self.opener(urls))
        # The other job is found by its rebuilt name, so only one job is read.
        self.assertEqual(urls, ["https://api.github.com/repos/kyverno/kyverno/actions/jobs/5"])
        summary = cr.render(reports, jobs, self.ENV)
        self.assertNotIn(self.ROW, summary)
        self.assertIn("| <code>suite/a</code> | v1.36.4 |", summary)

    def test_a_job_with_its_own_name_that_still_fails_keeps_its_failure(self):
        reports = cr.load(self.run_with_old_failures().root)
        jobs = [api_job(9, self.NAME), api_job(10, "Conformance tests / suite (v1.36.4)")]
        cr.find_names(reports, jobs, self.ENV, opener=self.opener([]))
        summary = cr.render(reports, jobs, self.ENV)
        self.assertIn(self.ROW, summary)
        # Its failed test explains the job, so it isn't listed as unexplained.
        self.assertNotIn("### Failed jobs without a failed test", summary)

    def test_a_failed_lookup_leaves_the_reports_as_they_are(self):
        run = self.run_with_old_failures()
        run.add("self-signed-certificates", "v1.36.4", {"tests": [result("ecdsa", ".", "boom")]}, index=2, job_id=7,
                tests_path="tls-certificates/ecdsa-self-signed")
        reports, calls = cr.load(run.root), []

        def down(request, timeout):
            calls.append(request.full_url)
            raise OSError("down")

        jobs = [api_job(9, self.NAME, "success")]
        with patch("time.sleep"), patch("sys.stdout", io.StringIO()):
            cr.find_names(reports, jobs, self.ENV, opener=down)
        # Three tries for the first job, and none for the jobs after it.
        self.assertEqual(len(calls), 3)
        self.assertFalse(any("job-name" in report for report in reports))
        self.assertIn(self.ROW, cr.render(reports, jobs, self.ENV))
        calls.clear()
        cr.find_names(reports, jobs, self.ENV, deadline=-1, opener=down)
        self.assertEqual(calls, [])

    def test_only_old_failed_reports_with_a_job_id_are_looked_up(self):
        run, urls = Run(self), []
        path = "tls-certificates/rsa-cert-manager"
        # No job id, no failure, and a report of this attempt.
        run.add("cert-manager-certificates", "v1.34.11", {"tests": [result("rsa", ".", "boom")]}, tests_path=path)
        run.add("cert-manager-certificates", "v1.35.8", {"tests": [result("rsa", ".")]}, index=1, job_id=5,
                tests_path=path)
        run.add("cert-manager-certificates", "v1.36.4", {"tests": [result("rsa", ".", "boom")]}, index=2, attempt=2,
                job_id=6, tests_path=path)
        cr.find_names(cr.load(run.root), [], self.ENV, opener=self.opener(urls))
        self.assertEqual(urls, [])

    def test_the_number_of_lookups_is_capped(self):
        run, urls = Run(self), []
        for index in range(cr.MAX_LOOKUPS + 3):
            run.add("cert-manager-certificates", f"v1.{index}.0", {"tests": [result("rsa", ".", "boom")]},
                    index=index, job_id=100 + index, tests_path="tls-certificates/rsa-cert-manager")
        cr.find_names(cr.load(run.root), [], self.ENV, opener=self.opener(urls))
        self.assertEqual(len(urls), cr.MAX_LOOKUPS)

    def test_summarize_reads_the_job_list_and_then_the_names(self):
        run = self.run_with_old_failures()
        target = run.root / "summary.md"
        jobs = [api_job(9, self.NAME, "success"), api_job(10, "Conformance tests / suite (v1.36.4)")]

        def urlopen(request, timeout):
            if "/attempts/2/jobs?" in request.full_url:
                return io.BytesIO(json.dumps({"total_count": len(jobs), "jobs": jobs}).encode())
            return self.opener([])(request, timeout)

        env = {**self.ENV, "GITHUB_STEP_SUMMARY": str(target)}
        with patch.object(cr.urllib.request, "urlopen", urlopen), patch.dict(os.environ, env, clear=True):
            self.assertEqual(cr.main(["summarize", str(run.root)]), 0)
        summary = target.read_text()
        self.assertNotIn(self.ROW, summary)
        self.assertIn("| <code>suite/a</code> | v1.36.4 |", summary)


class Jobs(unittest.TestCase):
    def report(self, **fields):
        return {"job": "deleting-policies", "k8s": "v1.36.4", "index": "17", "attempt": 1, "job-id": "",
                "tests-path": "deleting-policies", "shard-index": "0", "shard-count": "0", "tests": [], **fields}

    def test_finds_the_job_by_id_then_by_name(self):
        jobs = [
            api_job(1, "Conformance tests / deleting-policies (v1.36.4)"),
            api_job(2, "Conformance tests / deleting-policies (v1.36.4, 5)"),
            api_job(3, "Conformance tests / deleting-policies (v1.37.0, 5)"),
            api_job(4, "caller / Conformance tests / namespaced-deleting-policies (v1.36.4)"),
            api_job(5, "Conformance tests / v1.36.4 - cert-manager rsa"),
        ]
        self.assertEqual(cr.find_job(self.report(), jobs)["id"], 1)
        self.assertEqual(cr.find_job(self.report(**{"shard-index": "5", "shard-count": "6"}), jobs)["id"], 2)
        self.assertEqual(cr.find_job(self.report(job="namespaced-deleting-policies"), jobs)["id"], 4)
        self.assertEqual(cr.find_job(self.report(**{"job-id": "3"}), jobs)["id"], 3)
        # A job with its own name can't be found by name.
        self.assertIsNone(cr.find_job(self.report(job="cert-manager-certificates"), jobs))
        self.assertEqual(cr.find_job(self.report(job="cert-manager-certificates", **{"job-id": "5"}), jobs)["id"], 5)
        self.assertIsNone(cr.find_job(self.report(), None))

    def test_log_link_degrades_to_the_run(self):
        jobs = [api_job(1, "Conformance tests / deleting-policies (v1.36.4)")]
        self.assertEqual(cr.log_url(self.report(**{"job-id": "9"}), None, RUN_URL), f"{RUN_URL}/job/9")
        self.assertEqual(cr.log_url(self.report(), jobs, RUN_URL), f"{RUN_URL}/job/1")
        self.assertEqual(cr.log_url(self.report(), None, RUN_URL), RUN_URL)
        self.assertEqual(cr.log_url(self.report(**{"job-id": "1.3E+11"}), [], RUN_URL), RUN_URL)

    def test_failed_jobs_without_a_failed_test_are_listed(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a", "boom")]}, index=0)
        run.add("suite", "v1.35.8", {"tests": [result("a", "a")]}, index=1)
        run.add("suite", "v1.36.4", None, index=2)
        jobs = [
            api_job(1, "Conformance tests / suite (v1.34.11)"),
            api_job(2, "Conformance tests / suite (v1.35.8)"),
            api_job(3, "Conformance tests / suite (v1.36.4)"),
            api_job(4, "Conformance tests / suite (v1.37.0)", "timed_out"),
            api_job(5, "Build test images", step="Build"),
            api_job(6, "Performance benchmarks", "success"),
            api_job(7, "Conformance tests / other (v1.37.0)", "cancelled"),
            api_job(8, "K6 tests / load", None),
        ]
        others = cr.other_failures(jobs, cr.load(run.root))
        self.assertEqual([(other["name"], other["conclusion"], other["step"], other["report"]) for other in others], [
            ("Conformance tests / suite (v1.35.8)", "failure",
             "Run ./.github/actions/tests/conformance/run", "no failed test"),
            ("Conformance tests / suite (v1.36.4)", "failure", "Run ./.github/actions/tests/conformance/run", "none"),
            ("Conformance tests / suite (v1.37.0)", "timed_out", "", "none"),
            ("Build test images", "failure", "Build", "none"),
        ])

    def page(self, jobs, total):
        return io.BytesIO(json.dumps({"total_count": total, "jobs": jobs}).encode())

    def test_fetches_every_page_of_the_job_list(self):
        pages = [self.page([{"id": 1}, {"id": 2}], 3), self.page([{"id": 3}], 3)]
        urls = []

        def opener(request, timeout):
            urls.append((request.full_url, request.get_header("Authorization")))
            return pages.pop(0)

        jobs = cr.fetch_jobs({**ENV, "GITHUB_RUN_ATTEMPT": "2", "GITHUB_TOKEN": "token"}, opener=opener)
        self.assertEqual([item["id"] for item in jobs], [1, 2, 3])
        base = "https://api.github.com/repos/kyverno/kyverno/actions/runs/42/attempts/2/jobs?per_page=25&page="
        self.assertEqual(urls, [(base + "1", "Bearer token"), (base + "2", "Bearer token")])

    def test_a_failing_job_list_is_retried_and_then_given_up(self):
        calls = []

        def opener(request, timeout):
            calls.append(request.full_url)
            if len(calls) < 3:
                raise OSError("HTTP Error 502: Bad Gateway")
            return self.page([{"id": 1}], 1)

        def down(request, timeout):
            calls.append(request.full_url)
            raise OSError("down")

        with patch("time.sleep"), patch("sys.stdout", io.StringIO()):
            self.assertEqual(cr.fetch_jobs(ENV, opener=opener), [{"id": 1}])
            self.assertEqual(len(calls), 3)
            calls.clear()
            self.assertIsNone(cr.fetch_jobs(ENV, opener=down))
            self.assertEqual(len(calls), 3)
            # Past the deadline the API isn't called again.
            calls.clear()
            self.assertIsNone(cr.fetch_jobs(ENV, deadline=-1, opener=down))
            self.assertEqual(calls, [])


class Summary(unittest.TestCase):
    def test_sample_run(self):
        run, jobs = sample_run(self)
        summary = run.render(jobs)
        expected = HERE / "testdata" / "summary.md"
        if os.environ.get("UPDATE_SUMMARY"):
            expected.write_text(summary)
        self.assertEqual(summary, expected.read_text())

    def test_no_reports_at_all(self):
        run = Run(self)
        summary = run.render([api_job(1, "Build test images", step="Build")])
        self.assertIn("No conformance job uploaded test results", summary)
        self.assertIn(f"| [Build test images]({RUN_URL}/job/1) | failure | Build | none |", summary)
        self.assertNotIn("### Totals", summary)
        # The directory doesn't exist when no artifact matched.
        self.assertEqual(cr.load(run.root / "missing"), [])

    def test_no_failures(self):
        run = Run(self)
        for index, version in enumerate(VERSIONS):
            run.add("ttl", version, passing(fixture("ttl.json")), index=index)
        summary = run.render([])
        self.assertIn("No test failed in the 4 job reports with test results.", summary)
        self.assertNotIn("| Test |", summary)
        self.assertIn("- Tests: 0 failed, 3 passed, 0 skipped.", summary)

    def test_a_job_without_results_on_its_re_run_no_longer_counts_with_its_old_failures(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a", "boom")]})
        # The re-run failed before chainsaw: what test a does now is unknown.
        run.add("suite", "v1.34.11", None, attempt=2)
        jobs = [api_job(9, "Conformance tests / suite (v1.34.11)")]
        summary = run.render(jobs, attempt=2)
        self.assertIn("No conformance job uploaded test results", summary)
        self.assertNotIn("| Test |", summary)
        self.assertIn(f"| [Conformance tests / suite (v1.34.11)]({RUN_URL}/job/9) | failure | "
                      "Run ./.github/actions/tests/conformance/run | none |", summary)
        self.assertIn("- Reports: 1, on Kubernetes v1.34.11. 1 job uploaded no test results.", summary)

    def test_unreadable_job_list_is_reported(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a", "boom")]})
        summary = run.render(None)
        self.assertIn("The job list of this run couldn't be read", summary)
        self.assertIn(f"[v1.34.11]({RUN_URL})", summary)

    def test_report_text_cannot_break_the_table(self):
        run = Run(self)
        error = "line one\nline two | <img src=x> @kyverno/maintainers `code` [link](http://x) #1 " + "x" * 500
        run.add("suite", "v1.34.11", {"tests": [result("a", "a|b", error)]})
        row = next(line for line in run.render([]).splitlines() if line.startswith("| <code>suite"))
        self.assertEqual(row.count("|"), 6)
        self.assertNotIn("<img", row)
        self.assertNotIn("@", row)
        self.assertNotIn("`", row)
        self.assertNotIn("[link]", row)
        self.assertIn("line one line two", row)
        self.assertIn("…</code>", row)
        self.assertLess(len(row), 900)

    def test_long_lists_are_cut(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result(f"t{n}", f"t{n}", "boom") for n in range(cr.MAX_ROWS + 5)]})
        jobs = [api_job(n, f"job {n}", step="Build") for n in range(cr.MAX_JOBS + 5)]
        summary = run.render(jobs)
        self.assertEqual(sum(line.startswith("| <code>suite") for line in summary.splitlines()), cr.MAX_ROWS)
        self.assertIn(f"Only the first {cr.MAX_ROWS} failed tests are listed.", summary)
        self.assertIn(f"Only the first {cr.MAX_JOBS} of {cr.MAX_JOBS + 5} such jobs are listed.", summary)

    def test_summarize_writes_the_job_summary(self):
        run, _ = sample_run(self)
        target = run.root / "summary.md"
        # Without a run id the job list isn't read.
        with patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(target)}, clear=True):
            code = cr.main(["summarize", str(run.root)])
        self.assertEqual(code, 0)
        self.assertIn("**3 tests failed**", target.read_text())

    def test_failed_download_is_reported(self):
        run = Run(self)
        run.add("suite", "v1.34.11", {"tests": [result("a", "a")]})
        summary = cr.render(cr.load(run.root), [], {**ENV, "REPORTS_DOWNLOAD": "failure"})
        self.assertIn("Downloading the test reports failed", summary)
        self.assertNotIn("Downloading the test reports failed", run.render([]))

    def test_summarize_reports_its_own_failure_without_failing(self):
        run, _ = sample_run(self)
        target = run.root / "summary.md"
        with patch.object(cr, "render", side_effect=KeyError("boom")), patch("sys.stdout", io.StringIO()) as out, \
                patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": str(target)}, clear=True):
            code = cr.main(["summarize", str(run.root)])
        self.assertEqual(code, 0)
        self.assertIn("The summary couldn't be built: KeyError: 'boom'", target.read_text())
        self.assertIn("::error::", out.getvalue())


if __name__ == "__main__":
    unittest.main()
