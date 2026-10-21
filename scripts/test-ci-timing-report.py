#!/usr/bin/env python3
"""Offline regression tests for the CI timing collector and reporter."""

import argparse
import importlib.util
import json
from pathlib import Path
import unittest
from unittest.mock import patch

SPEC = importlib.util.spec_from_file_location("ci_timing", Path(__file__).with_name("ci-timing-report.py"))
TIMING = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(TIMING)


def at(minute):
    return f"2026-10-21T00:{minute:02d}:00Z"


def run(workflow="unit.yaml", sha="abc", identifier=1, created=0, start=1, finish=3, **changes):
    data = {"workflow": workflow, "sha": sha, "id": identifier, "attempt": 1,
            "event": "pull_request", "created_at": at(created),
            "status": "completed", "conclusion": "success", "url": "https://example.invalid/run",
            "jobs": [{"id": identifier, "status": "completed", "conclusion": "success",
                      "started_at": at(start), "completed_at": at(finish)}]}
    data.update(changes)
    return data


def snapshot(*runs, workflows=None):
    return {"schema_version": 1, "repository": "kyverno/kyverno", "event": "pull_request",
            "workflows": workflows or ["unit.yaml"], "runs": list(runs)}


class TimingTests(unittest.TestCase):
    def test_parallel_workflows_use_envelope_not_duration_sum(self):
        result = TIMING.analyze(snapshot(run(), run("lint.yaml", identifier=2, finish=4),
                                         workflows=["unit.yaml", "lint.yaml"]))
        self.assertEqual(result["observed_wall"]["p50_seconds"], 240)
        self.assertEqual(result["latest_attempt_runner_occupancy"]["p50_seconds"], 300)
        self.assertEqual(result["creation_to_first_job_delay"]["p50_seconds"], 60)

    def test_workflow_updated_at_is_not_used(self):
        result = TIMING.analyze(snapshot(run(updated_at="2099-01-01T00:00:00Z")))
        self.assertEqual(result["observed_wall"]["p50_seconds"], 180)

    def test_same_sha_required_and_missing_workflows_excluded(self):
        result = TIMING.analyze(snapshot(run(sha="a"), run("lint.yaml", sha="b"),
                                         workflows=["unit.yaml", "lint.yaml"]))
        self.assertEqual(result["states"], {"missing_workflows": 2})
        self.assertEqual(result["observed_wall"]["n"], 0)

    def test_failures_cancellations_approval_waits_and_pending_retained(self):
        cases = [("failure", "completed", "failure"), ("cancelled", "completed", "cancelled"),
                 ("action_required", "completed", "action_required"),
                 (None, "in_progress", "in_progress"), ("skipped", "completed", "non_success")]
        for conclusion, status, expected in cases:
            with self.subTest(conclusion=conclusion):
                result = TIMING.analyze(snapshot(run(conclusion=conclusion, status=status, jobs=[])))
                self.assertEqual(result["cohorts"][0]["state"], expected)
                self.assertEqual(result["observed_wall"]["n"], 0)

    def test_newer_failed_run_does_not_reuse_previous_success(self):
        result = TIMING.analyze(snapshot(run(), run(identifier=2, created=4, conclusion="failure")))
        self.assertEqual(result["states"], {"failure": 1})

    def test_latest_attempt_only_but_wall_includes_retry_wait(self):
        result = TIMING.analyze(snapshot(run(conclusion="failure"),
                                         run(attempt=2, start=10, finish=12)))
        self.assertEqual(result["observed_wall"]["p50_seconds"], 720)
        self.assertEqual(result["latest_attempt_runner_occupancy"]["p50_seconds"], 120)
        self.assertEqual(result["cohorts"][0]["runs"][0]["attempt"], 2)

    def test_success_without_complete_jobs_is_not_measured(self):
        cases = [[], [{"status": "queued"}], [{"status": "completed", "conclusion": "success"}],
                 [{"status": "completed", "conclusion": "skipped"}],
                 [{"status": "completed", "conclusion": "success", "started_at": None, "completed_at": at(3)}],
                 [{"status": "completed", "conclusion": "success", "started_at": at(4), "completed_at": at(2)}]]
        for jobs in cases:
            with self.subTest(jobs=jobs):
                result = TIMING.analyze(snapshot(run(jobs=jobs)))
                self.assertEqual(result["states"], {"incomplete_jobs": 1})
                self.assertEqual(result["observed_wall"]["n"], 0)

    def test_skipped_matrix_jobs_do_not_need_timestamps(self):
        data = run()
        data["jobs"].append({"conclusion": "skipped", "status": "completed"})
        self.assertEqual(TIMING.analyze(snapshot(data))["observed_wall"]["n"], 1)

    def test_percentiles_and_empty_populations(self):
        self.assertIsNone(TIMING.percentile([], 0.9))
        self.assertEqual(TIMING.percentile(list(range(1, 101)), 0.9), 90)
        self.assertEqual(TIMING.analyze(snapshot())["cohort_count"], 0)

    def test_mixed_events_unknown_workflows_and_bad_schema_rejected(self):
        for data in [snapshot(run(event="push")), snapshot(run(workflow="other.yaml")),
                     {**snapshot(), "schema_version": 2}, {**snapshot(), "workflows": []}]:
            with self.assertRaises(ValueError):
                TIMING.analyze(data)

    def test_small_sample_caveat_and_scope_are_visible(self):
        text = TIMING.markdown(TIMING.analyze(snapshot(run())))
        self.assertIn("Fewer than 100", text)
        self.assertIn("not an all-required-checks", text)
        self.assertIn("not billed", text)

    def test_collector_reads_all_pages_and_specific_attempt(self):
        raw = {"id": 42, "run_attempt": 3, "head_sha": "abc", "event": "pull_request",
               "created_at": at(0), "status": "completed", "conclusion": "success", "html_url": "url"}
        args = argparse.Namespace(repo="kyverno/kyverno", workflow=["unit.yaml"], event="pull_request",
                                  branch=None, since=at(0), until=at(30))
        pages = [[{"total_count": 1, "workflow_runs": [raw]}, {"total_count": 1, "workflow_runs": []}],
                 [{"jobs": [{"id": 1}]}, {"jobs": [{"id": 2}]}]]
        with patch.object(TIMING, "api_pages", side_effect=pages) as api:
            result = TIMING.collect(args)
        self.assertEqual(len(result["runs"][0]["jobs"]), 2)
        self.assertIn("/42/attempts/3/jobs", api.call_args_list[1].args[0])

    def test_collector_rejects_truncated_search(self):
        args = argparse.Namespace(repo="kyverno/kyverno", workflow=["unit.yaml"], event="push",
                                  branch="main", since=at(0), until=at(30))
        with patch.object(TIMING, "api_pages", return_value=[{"total_count": 1000, "workflow_runs": []}]):
            with self.assertRaisesRegex(ValueError, "smaller time window"):
                TIMING.collect(args)

    def test_collector_rejects_missing_run_pages(self):
        args = argparse.Namespace(repo="kyverno/kyverno", workflow=["unit.yaml"], event="push",
                                  branch="main", since=at(0), until=at(30))
        with patch.object(TIMING, "api_pages", return_value=[{"total_count": 2, "workflow_runs": []}]):
            with self.assertRaisesRegex(ValueError, "pagination"):
                TIMING.collect(args)

    def test_collector_rejects_missing_job_pages(self):
        raw = {"id": 1, "run_attempt": 1}
        args = argparse.Namespace(repo="kyverno/kyverno", workflow=["unit.yaml"], event="push",
                                  branch="main", since=at(0), until=at(30))
        with patch.object(TIMING, "api_pages", side_effect=[
                [{"total_count": 1, "workflow_runs": [raw]}],
                [{"total_count": 2, "jobs": [{"id": 1}]}]]):
            with self.assertRaisesRegex(ValueError, "inventory"):
                TIMING.collect(args)

    def test_gh_pagination_is_requested_without_shell(self):
        with patch.object(TIMING.subprocess, "run") as call:
            call.return_value.stdout = json.dumps([{"jobs": []}, {"jobs": []}])
            self.assertEqual(len(TIMING.api_pages("endpoint")), 2)
        self.assertEqual(call.call_args.args[0], ["gh", "api", "--paginate", "--slurp", "endpoint"])


if __name__ == "__main__":
    unittest.main()
