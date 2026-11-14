#!/usr/bin/env python3
"""Offline tests for scripts/e2e/report.py.

The fixtures in testdata/ follow the JSON report format of chainsaw v0.2.15
(chainsaw test --report-format JSON). Each directory holds what one conformance
job uploads: meta.json and, when chainsaw ran, results.json.

Run: python3 scripts/e2e/test_report.py
"""

import contextlib
import importlib.util
import io
import json
import shutil
import tempfile
import unittest
from pathlib import Path
from xml.etree import ElementTree

HERE = Path(__file__).resolve().parent
TESTDATA = HERE / "testdata"
spec = importlib.util.spec_from_file_location("report", HERE / "report.py")
report = importlib.util.module_from_spec(spec)
spec.loader.exec_module(report)


def run(*argv):
    out = io.StringIO()
    with contextlib.redirect_stdout(out):
        report.main(list(argv))
    return out.getvalue()


class PrepareTest(unittest.TestCase):
    def setUp(self):
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)

    def test_converts_the_json_report_to_junit(self):
        shutil.copy(TESTDATA / "e2e-reports-validating-policies-v1.37.0-0" / "results.json", self.dir)
        name = run("prepare", "--dir", str(self.dir), "--suite", "validating-policies",
                   "--k8s", "v1.37.0", "--shard", "0", "--outcome", "failure",
                   "--job-url", "https://example.com/job/1")
        self.assertEqual(name.strip(), "e2e-reports-validating-policies-v1.37.0-0")
        root = ElementTree.parse(self.dir / "results.xml").getroot()
        self.assertEqual(root.tag, "testsuites")
        self.assertEqual((root.get("tests"), root.get("failures"), root.get("skipped")), ("4", "2", "1"))
        cases = {case.get("name"): case for case in root.iter("testcase")}
        failure = cases["webhook-config"].find("failure")
        self.assertTrue(failure.get("message").startswith("v1/ConfigMap/default/check - data.mode"))
        self.assertIn("+mode: enforce", failure.text)
        self.assertIsNotNone(cases["skipped-test"].find("skipped"))
        self.assertIsNone(cases["passes"].find("failure"))
        self.assertEqual(cases["passes"].get("classname"), "chainsaw/validating-policies/passes")
        meta = json.loads((self.dir / "meta.json").read_text())
        self.assertEqual(meta, {"suite": "validating-policies", "k8s": "v1.37.0", "shard": "0",
                                "outcome": "failure", "jobUrl": "https://example.com/job/1",
                                "report": True})

    def test_records_a_job_without_a_report(self):
        name = run("prepare", "--dir", str(self.dir / "new"), "--suite", "tls-certificates/rsa-cert-manager",
                   "--k8s", "v1.37.0", "--outcome", "failure")
        self.assertEqual(name.strip(), "e2e-reports-tls-certificates-rsa-cert-manager-v1.37.0-0")
        self.assertFalse((self.dir / "new" / "results.xml").exists())
        self.assertFalse(json.loads((self.dir / "new" / "meta.json").read_text())["report"])

    def test_reads_nanosecond_timestamps(self):
        test = {"startTime": "2026-11-13T19:24:09.24659-07:00", "endTime": "2026-11-14T02:24:10.254864123Z"}
        self.assertAlmostEqual(report.seconds(test), 1.008274, places=5)
        self.assertEqual(report.seconds({}), 0.0)


class SummarizeTest(unittest.TestCase):
    def test_groups_failures_by_test_and_classifies_them(self):
        results, missing = report.collect(TESTDATA)
        rows = report.group(results)
        self.assertEqual([(r["test"], r["scope"], r["failed"], r["ran"]) for r in rows], [
            ("webhook-config", "all versions", ["v1.34.11", "v1.37.0"], ["v1.34.11", "v1.37.0"]),
            ("flaky-audit", "some versions", ["v1.37.0"], ["v1.34.11", "v1.37.0"]),
        ])
        self.assertEqual(rows[0]["error"],
                         'v1/ConfigMap/default/check - data.mode: Invalid value: "enforce": Expected value: "audit"')
        self.assertEqual(rows[0]["links"]["v1.37.0"], "https://github.com/kyverno/kyverno/actions/runs/1/job/11")
        self.assertEqual([(m["suite"], m["k8s"]) for m in missing], [("assert", "v1.37.0")])

    def test_writes_one_markdown_table(self):
        out = run("summarize", str(TESTDATA), "--run-url", "https://example.com/run")
        table = [line for line in out.splitlines() if line.startswith("| ")]
        self.assertEqual(table[0], "| Test | Suite | Scope | Failing Kubernetes versions | First error |")
        self.assertEqual(len(table), 4)
        self.assertIn("| webhook-config | validating-policies | all versions (2/2) | "
                      "[v1.34.11](https://github.com/kyverno/kyverno/actions/runs/1/job/10), "
                      "[v1.37.0](https://github.com/kyverno/kyverno/actions/runs/1/job/11) | "
                      "assert-webhook: v1/ConfigMap/default/check", table[2])
        self.assertIn("| flaky-audit | validating-policies | some versions (1/2) |", table[3])
        self.assertIn("- [assert on v1.37.0, shard 0](https://github.com/kyverno/kyverno/actions/runs/1/job/20)", out)
        self.assertNotIn("ok-test", out)
        self.assertIn("[run](https://example.com/run)", out)

    def test_reports_a_green_run(self):
        with tempfile.TemporaryDirectory() as directory:
            shutil.copytree(TESTDATA / "e2e-reports-assert-v1.34.11-0", Path(directory) / "a")
            self.assertIn("All 1 test results passed.", run("summarize", directory))
            self.assertIn("No chainsaw reports", run("summarize", str(Path(directory) / "none")))

    def test_escapes_table_cells(self):
        self.assertEqual(report.cell("a | b `c` <d>\ne"), "a \\| b 'c' &lt;d&gt; e")
        self.assertEqual(report.first_line("\n  first  \nsecond"), "first")
        self.assertEqual(len(report.first_line("x" * 500)), report.MAX_ERROR)


if __name__ == "__main__":
    unittest.main(verbosity=2)
