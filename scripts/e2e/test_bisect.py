#!/usr/bin/env python3
"""Offline tests for scripts/e2e/bisect.py.

Uses the fixtures in testdata/, which test_report.py describes.

Run: python3 scripts/e2e/test_bisect.py
"""

import contextlib
import importlib.util
import io
import json
import shutil
import tempfile
import types
import unittest
from pathlib import Path
from typing import Callable, List, Optional, Tuple

HERE = Path(__file__).resolve().parent
TESTDATA = HERE / "testdata"
spec = importlib.util.spec_from_file_location("bisect_e2e", HERE / "bisect.py")
bisect = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bisect)


def result(suite, k8s, test, status, variant=""):
    return {"suite": suite, "variant": variant, "k8s": k8s, "jobUrl": "", "test": test,
            "path": f"chainsaw/{suite}/{test}", "status": status, "error": "", "step": ""}


class SelectTest(unittest.TestCase):
    def test_picks_the_tests_that_fail_on_every_version(self):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            bisect.main(["select", str(TESTDATA)])
        plan = json.loads(out.getvalue())
        # webhook-config fails on both versions; the rbac test on its only
        # one, so validating-policies wins the tie. flaky-audit fails on some
        # versions only and isn't bisected.
        self.assertTrue(plan["bisect"])
        self.assertEqual((plan["suite"], plan["variant"], plan["k8s"]), ("validating-policies", "", "v1.37.0"))
        self.assertEqual(plan["tests"], ["webhook-config"])
        self.assertEqual(plan["regex"], "^(webhook-config)$")
        self.assertEqual(plan["others"], ["background-controller-permissions (rbac (default))"])

    def test_prefers_the_suite_with_the_most_failing_tests(self):
        results = [result("a", "v1.37.0", "x", "failed"),
                   result("b", "v1.34.0", "y", "failed"), result("b", "v1.37.0", "y", "failed"),
                   result("b", "v1.34.0", "z", "failed"), result("b", "v1.37.0", "z", "failed")]
        plan = bisect.select(results)
        self.assertEqual((plan["suite"], plan["k8s"], plan["tests"]), ("b", "v1.37.0", ["y", "z"]))
        self.assertEqual(plan["regex"], "^(y|z)$")

    def test_uses_the_latest_version_the_tests_ran_on(self):
        results = [result("map", "v1.34.11", "t", "failed"), result("map", "v1.35.8", "t", "failed")]
        self.assertEqual(bisect.select(results)["k8s"], "v1.35.8")

    def test_skips_runs_without_a_deterministic_failure(self):
        flaky = [result("a", "v1.34.0", "x", "passed"), result("a", "v1.37.0", "x", "failed")]
        self.assertEqual(bisect.select(flaky), {
            "bisect": False, "reason": "no test fails on every Kubernetes version it ran on"})
        self.assertFalse(bisect.select([result("a", "v1.37.0", "x", "passed")])["bisect"])
        self.assertEqual(bisect.select([])["reason"], "no chainsaw test failed")

    def test_skips_runs_with_too_many_failures(self):
        results = [result("a", "v1.37.0", f"t{i}", "failed") for i in range(6)]
        plan = bisect.select(results, max_tests=5)
        self.assertFalse(plan["bisect"])
        self.assertIn("more than the limit of 5", plan["reason"])

    def test_escapes_regex_characters(self):
        self.assertEqual(bisect.test_regex(["a.b", "c+d"]), r"^(a\.b|c\+d)$")


class NextProbeTest(unittest.TestCase):
    def test_halves_the_range(self):
        # 8 commits; the head (7) is bad, so the range is (-1, 7).
        self.assertEqual(bisect.next_probe(8, {7: "bad"}), 3)
        self.assertEqual(bisect.next_probe(8, {7: "bad", 3: "good"}), 5)
        self.assertEqual(bisect.next_probe(8, {7: "bad", 3: "bad"}), 1)

    def test_routes_around_skipped_commits(self):
        self.assertEqual(bisect.next_probe(8, {7: "bad", 3: "skip"}), 2)
        self.assertIsNone(bisect.next_probe(3, {2: "bad", 0: "good", 1: "skip"}))

    def test_stops_without_a_bad_commit_or_range(self):
        self.assertIsNone(bisect.next_probe(8, {7: "good"}))
        self.assertIsNone(bisect.next_probe(8, {7: "bad", 6: "good"}))


class BisectTest(unittest.TestCase):
    commits = [f"c{i}" for i in range(32)]

    def run_with(self, verdict: Callable[[int], str], max_steps: int = 6,
                 deadline: Optional[float] = None) -> Tuple[dict, List[str]]:
        probed: List[str] = []

        def probe(sha: str) -> str:
            probed.append(sha)
            return verdict(self.commits.index(sha))
        return bisect.bisect(self.commits, probe, max_steps, deadline, clock=lambda: 0), probed

    def test_finds_the_first_bad_commit_of_32_in_6_steps(self):
        for culprit in (0, 1, 13, 30, 31):
            out, probed = self.run_with(lambda i, c=culprit: "bad" if i >= c else "good")
            self.assertEqual((out["outcome"], out["suspects"]), ("found", [f"c{culprit}"]), culprit)
            self.assertEqual(probed[0], "c31")
            self.assertLessEqual(len(probed), 6)

    def test_stops_when_the_head_passes(self):
        out, probed = self.run_with(lambda i: "good")
        self.assertEqual((out["outcome"], probed), ("not-reproducible", ["c31"]))

    def test_stops_when_the_head_cannot_be_tested(self):
        out, probed = self.run_with(lambda i: "skip")
        self.assertEqual((out["outcome"], probed), ("inconclusive", ["c31"]))

    def test_reports_a_range_around_skipped_commits(self):
        out, _ = self.run_with(lambda i: "bad" if i >= 20 else ("skip" if i == 19 else "good"), max_steps=10)
        self.assertEqual((out["outcome"], out["suspects"]), ("range", ["c19", "c20"]))

    def test_reports_the_range_left_when_the_budget_runs_out(self):
        out, probed = self.run_with(lambda i: "bad" if i >= 13 else "good", max_steps=3)
        self.assertEqual(len(probed), 3)
        self.assertEqual(out["outcome"], "budget")
        self.assertIn("c13", out["suspects"])

    def test_stops_at_the_deadline(self):
        out, probed = self.run_with(lambda i: "bad", deadline=0)
        self.assertEqual((probed, out["outcome"]), ([], "inconclusive"))


class VerdictTest(unittest.TestCase):
    def setUp(self):
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)

    def test_classifies_from_the_chainsaw_report(self):
        self.assertEqual(bisect.verdict_of(self.dir, ["webhook-config"]), "skip")
        shutil.copy(TESTDATA / "e2e-reports-validating-policies-v1.37.0-0" / "results.json", self.dir)
        self.assertEqual(bisect.verdict_of(self.dir, ["webhook-config"]), "bad")
        self.assertEqual(bisect.verdict_of(self.dir, ["passes"]), "good")
        self.assertEqual(bisect.verdict_of(self.dir, ["passes", "webhook-config"]), "bad")
        # A skipped target test says nothing about the commit.
        self.assertEqual(bisect.verdict_of(self.dir, ["skipped-test"]), "skip")
        self.assertEqual(bisect.verdict_of(self.dir, ["passes", "skipped-test"]), "skip")
        # A test that doesn't exist yet at a commit can't fail there, but one
        # that exists and is missing from the report wasn't run.
        self.assertEqual(bisect.verdict_of(self.dir, ["added-later"]), "good")
        self.assertEqual(bisect.verdict_of(self.dir, ["added-later"], {"added-later"}), "skip")

    def test_finds_the_name_of_every_conformance_test(self):
        root = HERE.parents[1] / "test" / "conformance" / "chainsaw"
        files = list(root.rglob("chainsaw-test.y*ml"))
        self.assertGreater(len(files), 100)
        unnamed = [str(f) for f in files if not bisect._TEST_NAME.search(f.read_text())]
        self.assertEqual(unnamed, [])
        self.assertIn("invalid-label", bisect.test_names(root / "ttl"))


class Completed:
    def __init__(self, returncode: int = 0) -> None:
        self.returncode = returncode


class ProbeTest(unittest.TestCase):
    """Runs make_probe with a fake subprocess runner, so no cluster is needed."""

    def setUp(self):
        self.root = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.root)
        suite = self.root / "test" / "conformance" / "chainsaw" / "validating-policies" / "webhook-config"
        suite.mkdir(parents=True)
        (suite / "chainsaw-test.yaml").write_text(
            "apiVersion: chainsaw.kyverno.io/v1alpha1\nkind: Test\nmetadata:\n  name: webhook-config\n")
        self.calls: List[List[str]] = []
        self.report_dirs: List[Path] = []

    def probe(self, checkout: int = 0, report: Optional[str] = "results.json",
              tests: Tuple[str, ...] = ("webhook-config",)) -> str:
        def runner(command: List[str], **_: object) -> Completed:
            self.calls.append(command)
            if command[:2] == ["git", "checkout"]:
                return Completed(checkout)
            if "test-e2e-local" in command:
                work = Path(next(a for a in command if a.startswith("E2E_REPORT_DIR="))[len("E2E_REPORT_DIR="):])
                self.report_dirs.append(work)
                self.assertTrue(work.is_dir())
                if report:
                    shutil.copy(TESTDATA / "e2e-reports-validating-policies-v1.37.0-0" / report, work)
                return Completed(1)
            return Completed(0)

        args = types.SimpleNamespace(tests_json=json.dumps(list(tests)), suite="validating-policies",
                                     variant="", k8s="v1.37.0", step_timeout=2100)
        probe = bisect.make_probe(args, log=io.StringIO(), runner=runner, root=self.root)
        return probe("abc")

    def assert_cleaned_up(self) -> None:
        self.assertEqual(self.calls[-1][:2], ["make", "kind-delete-cluster"])
        for work in self.report_dirs:
            self.assertFalse(work.exists())

    def test_a_failed_test_makes_the_commit_bad(self):
        self.assertEqual(self.probe(), "bad")
        self.assertEqual(self.calls[0], ["git", "checkout", "--force", "--quiet", "abc"])
        make = self.calls[1]
        self.assertEqual(make[:5], ["timeout", "--kill-after=60", "2100", "make", "test-e2e-local"])
        self.assertEqual(make[5:9], ["SUITE=validating-policies", "TEST=^(webhook-config)$$",
                                     "K8S=v1.37.0", "VARIANT="])
        self.assert_cleaned_up()

    def test_dollar_signs_are_doubled_for_make(self):
        self.probe(tests=("a$b", "c"))
        self.assertIn(r"TEST=^(a\$$b|c)$$", self.calls[1])

    def test_a_setup_failure_without_a_report_is_skipped(self):
        self.assertEqual(self.probe(report=None), "skip")
        self.assert_cleaned_up()

    def test_a_test_that_exists_but_did_not_run_is_skipped(self):
        # A test missing from both the report and the suite doesn't exist yet
        # at the commit. Once a chainsaw-test.yaml has its name, it does.
        self.assertEqual(self.probe(tests=("passes",)), "good")
        self.assertEqual(self.probe(tests=("missing",)), "good")
        (self.root / "test" / "conformance" / "chainsaw" / "validating-policies" / "webhook-config"
         / "chainsaw-test.yaml").write_text("metadata:\n  name: missing\n")
        self.assertEqual(self.probe(tests=("missing",)), "skip")

    def test_a_checkout_failure_is_skipped_and_cleaned_up(self):
        self.assertEqual(self.probe(checkout=1), "skip")
        self.assertEqual(len(self.calls), 2)
        self.assert_cleaned_up()

    def test_each_probe_gets_its_own_report_directory(self):
        self.probe()
        self.probe(report=None)
        self.assertEqual(len(self.report_dirs), 2)
        self.assertNotEqual(self.report_dirs[0], self.report_dirs[1])
        self.assert_cleaned_up()


class CommentTest(unittest.TestCase):
    plan = {"suite": "rbac", "variant": "default", "k8s": "v1.37.0", "tests": ["t"],
            "base": "a" * 40, "head": "b" * 40, "others": ["u (mutate)"]}

    def test_names_the_first_bad_commit_and_its_pull_request(self):
        prs = [{"html_url": "https://github.com/o/r/pull/7", "merged_at": "2026-01-01T00:00:00Z",
                "title": "fix: <b>it</b> | @someone `x`"}]
        out = bisect.comment({"outcome": "found", "suspects": ["c" * 40],
                              "steps": [{"sha": "b" * 40, "verdict": "bad", "seconds": 600}]},
                             self.plan, prs, "https://example.com/run", "https://github.com/o/r")
        self.assertIn("First bad commit: [`ccccccccc`](https://github.com/o/r/commit/" + "c" * 40 + ")", out)
        self.assertIn("https://github.com/o/r/pull/7 (fix: &lt;b&gt;it&lt;/b&gt; \\| @&#8203;someone 'x')", out)
        self.assertNotIn("@someone", out)
        self.assertIn("`rbac (default)`", out)
        self.assertIn("not bisected: u (mutate).", out)
        self.assertIn("| bad | 10 |", out)

    def test_explains_other_outcomes(self):
        out = bisect.comment({"outcome": "not-reproducible", "suspects": [], "steps": []}, self.plan)
        self.assertIn("isn't reproducible", out)
        out = bisect.comment({"outcome": "budget", "suspects": ["c" * 40, "d" * 40], "steps": []}, self.plan)
        self.assertIn("one of these 2 commits", out)


if __name__ == "__main__":
    unittest.main(verbosity=2)
