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
import os
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
        self.assertEqual(meta, {"suite": "validating-policies", "variant": "", "k8s": "v1.37.0", "shard": "0",
                                "outcome": "failure", "jobUrl": "https://example.com/job/1",
                                "report": True})

    def test_names_each_variant_of_a_suite_apart(self):
        names = {run("prepare", "--dir", str(self.dir / variant), "--suite", "rbac", "--variant", variant,
                     "--k8s", "v1.37.0", "--shard", "0").strip()
                 for variant in ("standard", "default", "force-failure-policy-ignore")}
        self.assertEqual(names, {"e2e-reports-rbac-standard-v1.37.0-0", "e2e-reports-rbac-default-v1.37.0-0",
                                 "e2e-reports-rbac-force-failure-policy-ignore-v1.37.0-0"})
        self.assertEqual(json.loads((self.dir / "default" / "meta.json").read_text())["variant"], "default")
        self.assertEqual(report.artifact_name("deprecations", "v1.37.0", "0", "standard,legacy-denial-probe"),
                         "e2e-reports-deprecations-standard-legacy-denial-probe-v1.37.0-0")

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
        self.assertEqual([(r["test"], r["suite"], r["scope"], r["failed"], r["ran"]) for r in rows], [
            ("background-controller-permissions", "rbac (default)", "all versions", ["v1.37.0"], ["v1.37.0"]),
            ("webhook-config", "validating-policies", "all versions",
             ["v1.34.11", "v1.37.0"], ["v1.34.11", "v1.37.0"]),
            ("flaky-audit", "validating-policies", "some versions", ["v1.37.0"], ["v1.34.11", "v1.37.0"]),
        ])
        self.assertEqual(rows[1]["error"],
                         'v1/ConfigMap/default/check - data.mode: Invalid value: "enforce": Expected value: "audit"')
        self.assertEqual(rows[1]["links"]["v1.37.0"], "https://github.com/kyverno/kyverno/actions/runs/1/job/11")
        self.assertEqual([(m["suite"], m["k8s"]) for m in missing], [("assert", "v1.37.0")])

    def test_keeps_variants_of_a_suite_apart(self):
        # The same rbac test passes with standard and fails with default:
        # merged, it would read as "some versions" (1/2) on a single version.
        results, _ = report.collect(TESTDATA)
        rbac = [r for r in results if r["suite"] == "rbac"]
        self.assertEqual(sorted((r["variant"], r["status"]) for r in rbac),
                         [("default", "failed"), ("standard", "passed")])
        row = next(r for r in report.group(results) if r["suite"].startswith("rbac"))
        self.assertEqual((row["suite"], row["scope"], row["ran"]), ("rbac (default)", "all versions", ["v1.37.0"]))

    def test_writes_one_markdown_table(self):
        out = run("summarize", str(TESTDATA), "--run-url", "https://example.com/run")
        table = [line for line in out.splitlines() if line.startswith("| ")]
        self.assertEqual(table[0], "| Test | Suite | Scope | Failing Kubernetes versions | First error |")
        self.assertEqual(len(table), 5)
        self.assertIn("| background-controller-permissions | rbac (default) | all versions (1/1) |", table[2])
        self.assertIn("| webhook-config | validating-policies | all versions (2/2) | "
                      "[v1.34.11](https://github.com/kyverno/kyverno/actions/runs/1/job/10), "
                      "[v1.37.0](https://github.com/kyverno/kyverno/actions/runs/1/job/11) | "
                      "assert-webhook: v1/ConfigMap/default/check", table[3])
        self.assertIn("| flaky-audit | validating-policies | some versions (1/2) |", table[4])
        self.assertIn("- [assert on v1.37.0, shard 0](https://github.com/kyverno/kyverno/actions/runs/1/job/20)", out)
        self.assertIn("11 test results: 5 passed, 4 failed, 2 skipped.", out)
        self.assertIn("- skipped-test (validating-policies): v1.34.11, v1.37.0", out)
        self.assertNotIn("ok-test", out)
        self.assertIn("[run](https://example.com/run)", out)
        # The CLI job doesn't deploy Kyverno, so the summary must not promise
        # diagnostics for every failing job.
        self.assertIn("A failing job that deploys Kyverno also uploads a `diagnostics-*` artifact", out)
        self.assertIn("The `cli` suite runs the Kyverno CLI without deploying Kyverno", out)
        self.assertNotIn("Each failing job uploads", out)

    def test_reports_a_green_run(self):
        with tempfile.TemporaryDirectory() as directory:
            shutil.copytree(TESTDATA / "e2e-reports-assert-v1.34.11-0", Path(directory) / "a")
            out = run("summarize", directory)
            self.assertIn("1 test results: 1 passed, 0 failed, 0 skipped.", out)
            self.assertIn("All test results passed.", out)
            self.assertIn("No chainsaw reports", run("summarize", str(Path(directory) / "none")))

    def test_counts_results_across_jobs(self):
        with tempfile.TemporaryDirectory() as directory:
            for name in ("e2e-reports-assert-v1.34.11-0", "e2e-reports-rbac-default-v1.37.0-0"):
                shutil.copytree(TESTDATA / name, Path(directory) / name)
            self.assertEqual(json.loads(run("counts", directory)),
                             {"passed": 1, "failed": 1, "skipped": 0, "missing": 0, "reports": 2})
            # No reports at all, for example when the download failed.
            self.assertEqual(json.loads(run("counts", str(Path(directory) / "none"))),
                             {"passed": 0, "failed": 0, "skipped": 0, "missing": 0, "reports": 0})
            # A report without tests, for example when a regex matched no test.
            empty = Path(directory) / "empty"
            empty.mkdir()
            (empty / "meta.json").write_text('{"suite": "mutate", "k8s": "v1.37.0", "outcome": "success"}')
            (empty / "results.json").write_text('{"tests": []}')
            self.assertEqual(json.loads(run("counts", directory))["reports"], 3)

    def test_reads_a_single_artifact_extracted_into_the_root(self):
        with tempfile.TemporaryDirectory() as directory:
            source = TESTDATA / "e2e-reports-rbac-default-v1.37.0-0"
            for name in ("meta.json", "results.json"):
                shutil.copy(source / name, directory)
            results, missing = report.collect(directory)
            self.assertEqual([(r["suite"], r["variant"], r["status"]) for r in results],
                             [("rbac", "default", "failed")])
            self.assertIn("| background-controller-permissions | rbac (default) |", run("summarize", directory))
            # A mix of both layouts is read once each.
            shutil.copytree(TESTDATA / "e2e-reports-rbac-standard-v1.37.0-0", Path(directory) / "standard")
            self.assertEqual(len(report.collect(directory)[0]), 2)

    def test_never_reports_skipped_tests_as_passed(self):
        with tempfile.TemporaryDirectory() as directory:
            for name in ("e2e-reports-assert-v1.34.11-0", "e2e-reports-validating-policies-v1.34.11-0"):
                shutil.copytree(TESTDATA / name, Path(directory) / name)
            # Keep only the passed and the skipped tests of validating-policies.
            path = Path(directory) / "e2e-reports-validating-policies-v1.34.11-0" / "results.json"
            data = json.loads(path.read_text())
            data["tests"] = [t for t in data["tests"] if t["status"] != "failed"]
            path.write_text(json.dumps(data))
            out = run("summarize", directory)
            self.assertNotIn("All test results passed", out)
            self.assertIn("4 test results: 3 passed, 0 failed, 1 skipped.", out)
            self.assertIn("Tests skipped on at least one version: 1.", out)
            self.assertIn("- skipped-test (validating-policies): v1.34.11", out)
            self.assertNotIn("| Test |", out)

    def test_lists_many_skipped_tests_by_count_only(self):
        results = [{"suite": "s", "variant": "", "k8s": "v1.37.0", "test": f"t{i}", "path": "", "status": "skipped",
                    "error": "", "step": "", "jobUrl": ""} for i in range(report.MAX_SKIPPED_NAMES + 1)]
        out = report.markdown([], [], results)
        self.assertIn(f"Tests skipped on at least one version: {report.MAX_SKIPPED_NAMES + 1}.", out)
        self.assertNotIn("- t0", out)
        self.assertNotIn("All test results passed", out)

    def test_escapes_table_cells(self):
        self.assertEqual(report.cell("a | b `c` <d>\ne"), "a \\| b 'c' &lt;d&gt; e")
        self.assertEqual(report.first_line("\n  first  \nsecond"), "first")
        self.assertEqual(len(report.first_line("x" * 500)), report.MAX_ERROR)


try:
    import yaml
except ImportError:
    yaml = None


# CI installs PyYAML for this test, so there it fails instead of skipping.
@unittest.skipUnless(yaml or os.environ.get("CI"), "PyYAML isn't installed")
class WorkflowTest(unittest.TestCase):
    def setUp(self):
        self.assertIsNotNone(yaml, "PyYAML is required in CI: pip install PyYAML")

    def test_every_matrix_dimension_is_part_of_the_artifact_names(self):
        """Jobs of one suite must not overwrite each other's reports.

        Artifact names hold the suite (tests-path), the variant, the version,
        and the shard, so any other matrix dimension of a job must appear in
        its tests-path or variant input.
        """
        workflow = yaml.safe_load((HERE.parents[1] / ".github/workflows/tests-conformance.yaml").read_text())
        checked = 0
        for name, job in workflow["jobs"].items():
            dimensions = set((job.get("strategy") or {}).get("matrix") or {}) - {"k8s-version", "shard-index"}
            for step in job.get("steps", []):
                if step.get("uses") != "./.github/actions/tests/conformance/run":
                    continue
                identity = str(step["with"].get("tests-path", "")) + str(step["with"].get("variant", ""))
                for dimension in dimensions:
                    checked += 1
                    self.assertIn(f"matrix.{dimension}", identity, f"job {name}: add matrix.{dimension} to variant")
        self.assertGreaterEqual(checked, 3)

    BLANK = {"ACTIONS_RUNTIME_TOKEN": "", "ACTIONS_CACHE_URL": "", "ACTIONS_RESULTS_URL": ""}

    def assert_isolated(self, steps, where):
        for step in steps:
            if "run" in step:
                env = step.get("env") or {}
                self.assertEqual({k: env.get(k) for k in self.BLANK}, self.BLANK,
                                 f"{where}: step {step.get('name', step['run'][:40])!r} must blank the ACTIONS_* variables")

    def test_steps_that_may_run_pr_code_blank_the_actions_service_variables(self):
        """/conformance runs PR code in these steps, under the default branch's cache scope."""
        root = HERE.parents[1]
        workflow = yaml.safe_load((root / ".github/workflows/tests-conformance.yaml").read_text())
        checked = 0
        for name, job in workflow["jobs"].items():
            steps = job.get("steps", [])
            checked += len(steps)
            self.assert_isolated(steps, f"tests-conformance.yaml job {name}")
        action = yaml.safe_load((root / ".github/actions/tests/conformance/run/action.yaml").read_text())
        self.assert_isolated(action["runs"]["steps"], "conformance run action")
        comment = yaml.safe_load((root / ".github/workflows/comment-commands.yaml").read_text())
        for name in ("conformance-images", "conformance-cli"):
            self.assert_isolated(comment["jobs"][name]["steps"], f"comment-commands.yaml job {name}")
        self.assertGreater(checked, 100)

    def test_trusted_actions_replace_the_pr_copy_only_for_pr_comments(self):
        root = HERE.parents[1]
        workflow = yaml.safe_load((root / ".github/workflows/tests-conformance.yaml").read_text())
        self.assertIs(workflow.get("on", workflow.get(True))["workflow_call"]["inputs"]["untrusted-ref"]["default"], False)
        for name, job in workflow["jobs"].items():
            if name == "kubernetes-matrix":
                continue
            steps = job["steps"]
            names = [step.get("name") for step in steps]
            self.assertIn("Use the CI actions of the workflow commit", names, name)
            overlay = steps[names.index("Use the CI actions of the workflow commit")]
            self.assertEqual(overlay["if"], "inputs.untrusted-ref", name)
            self.assertIn("git checkout FETCH_HEAD -- .github/actions scripts/e2e", overlay["run"])
            # It comes right after the checkout of the code under test, before any local action.
            checkout = next(i for i, step in enumerate(steps) if step.get("uses", "").startswith("actions/checkout@"))
            self.assertEqual(names.index("Use the CI actions of the workflow commit"), checkout + 1, name)
        comment = yaml.safe_load((root / ".github/workflows/comment-commands.yaml").read_text())
        self.assertIs(comment["jobs"]["conformance-tests"]["with"]["untrusted-ref"], True)
        for name in ("release-validate.yaml", "check-tests.yaml"):
            text = (root / ".github/workflows" / name).read_text()
            self.assertNotIn("untrusted-ref", text, name)


if __name__ == "__main__":
    unittest.main(verbosity=2)
