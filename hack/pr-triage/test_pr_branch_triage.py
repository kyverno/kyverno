import importlib.util
import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("pr-branch-triage.py")
SPEC = importlib.util.spec_from_file_location("pr_branch_triage", SCRIPT)
triage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(triage)


class MigrationClassificationTest(unittest.TestCase):
    def test_migration_word_forms_are_detected_on_migration_paths(self):
        for title in (
            "Policy migration",
            "Migrate legacy policies",
            "Deprecation warning",
            "Legacy support",
            "Remove v1.20 compatibility",
        ):
            with self.subTest(title=title):
                self.assertTrue(triage.is_migration_pr(title, "", ["pkg/deprecations/warnings.go"]))

    def test_migration_word_without_migration_path_is_not_flagged(self):
        self.assertFalse(triage.is_migration_pr("Policy migration", "", ["pkg/engine/rule.go"]))

    def test_explicit_retarget_override_survives_migration_classification(self):
        row = {
            "number": 123,
            "title": "Policy migration",
            "body": "",
            "files": ["pkg/deprecations/warnings.go"],
            "override": "RETARGET",
            "head_sha": "approved-head",
        }
        args = SimpleNamespace(
            diff_scan=False,
            repo="kyverno/kyverno",
            probe_rebase=True,
            base="main",
            probe_target_base="release-1.19",
        )
        with patch.object(triage, "probe_rebase", return_value=("OK", [])) as probe:
            triage.finalize_row(row, args)

        self.assertEqual(row["category"], "REVIEW-MIGRATION")
        self.assertEqual(row["override"], "RETARGET")
        self.assertEqual(row["probe"], "OK")
        probe.assert_called_once_with(
            "kyverno/kyverno", 123, "main", "release-1.19", "approved-head",
        )

    def test_null_migration_override_defaults_to_keep_main(self):
        row = {
            "number": 124,
            "title": "Policy migration",
            "body": "",
            "files": ["pkg/deprecations/warnings.go"],
            "override": None,
            "head_sha": "approved-head",
        }
        args = SimpleNamespace(
            diff_scan=False,
            repo="kyverno/kyverno",
            probe_rebase=False,
            base="main",
            probe_target_base="release-1.19",
        )
        triage.finalize_row(row, args)
        self.assertEqual(row["category"], "REVIEW-MIGRATION")
        self.assertEqual(row["override"], "KEEP_MAIN")


class RebaseProbeCleanupTest(unittest.TestCase):
    def test_ref_collision_does_not_remove_existing_refs_or_worktrees(self):
        def run(args, **kwargs):
            if args[:3] == ["git", "show-ref", "--verify"]:
                return subprocess.CompletedProcess(args, 0 if args[-1].endswith("/base-main") else 1)
            return subprocess.CompletedProcess(args, 0)

        with patch.object(triage.subprocess, "run", side_effect=run) as mocked_run:
            status, conflicts = triage.probe_rebase(
                "kyverno/kyverno", 123, "main", "release-1.19", "abc123",
            )

        self.assertEqual((status, conflicts), ("ERROR:ref-collision", []))
        self.assertFalse(any(c.args[0][1:3] == ["worktree", "remove"] for c in mocked_run.call_args_list))
        self.assertFalse(any(c.args[0][1:2] == ["update-ref"] for c in mocked_run.call_args_list))

    def test_successful_probe_removes_only_its_unique_worktree_and_refs(self):
        expected_sha = "abc123"

        def run(args, **kwargs):
            if args[:3] == ["git", "show-ref", "--verify"]:
                return subprocess.CompletedProcess(args, 1)
            if args[1] == "rev-parse":
                return subprocess.CompletedProcess(args, 0, stdout=f"{expected_sha}\n")
            if "merge-base" in args:
                return subprocess.CompletedProcess(args, 0, stdout="base-sha\n")
            return subprocess.CompletedProcess(args, 0)

        with patch.object(triage.subprocess, "run", side_effect=run) as mocked_run:
            status, conflicts = triage.probe_rebase(
                "kyverno/kyverno", 123, "main", "release-1.19", expected_sha,
            )

        self.assertEqual((status, conflicts), ("OK", []))
        fetch = next(c.args[0] for c in mocked_run.call_args_list if c.args[0][1] == "fetch")
        refs = [
            spec.rsplit(":", 1)[1] for spec in fetch
            if ":" in spec and spec.startswith(("pull/", "main:", "release-1.19:"))
        ]
        removed_refs = [
            c.args[0][-1] for c in mocked_run.call_args_list
            if c.args[0][1:3] == ["update-ref", "-d"]
        ]
        self.assertCountEqual(removed_refs, refs)
        worktree_add = next(c.args[0] for c in mocked_run.call_args_list if c.args[0][1:3] == ["worktree", "add"])
        worktree_remove = next(c.args[0] for c in mocked_run.call_args_list if c.args[0][1:3] == ["worktree", "remove"])
        self.assertEqual(worktree_add[-2], worktree_remove[-1])
        self.assertNotEqual(worktree_add[-2], ".wt-123")

    def test_head_change_between_snapshot_and_probe_is_not_accepted(self):
        def run(args, **kwargs):
            if args[:3] == ["git", "show-ref", "--verify"]:
                return subprocess.CompletedProcess(args, 1)
            if args[1] == "rev-parse":
                return subprocess.CompletedProcess(args, 0, stdout="new-head\n")
            return subprocess.CompletedProcess(args, 0)

        with patch.object(triage.subprocess, "run", side_effect=run) as mocked_run:
            status, conflicts = triage.probe_rebase(
                "kyverno/kyverno", 123, "main", "release-1.19", "approved-head",
            )

        self.assertEqual((status, conflicts), ("ERROR:head-sha-mismatch", []))
        self.assertFalse(any(c.args[0][1:3] == ["worktree", "add"] for c in mocked_run.call_args_list))


class LabelSnapshotSafetyTest(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.temp_path = Path(self.temp_dir.name)
        self.report = self.temp_path / "report.json"
        self.gh = self.temp_path / "gh"
        self.gh_log = self.temp_path / "gh.log"
        self.gh.write_text(
            "#!/bin/sh\n"
            "if [ \"$1 $2\" = \"label list\" ]; then "
            "printf '%s\\n' type_legacy type_cel type_mixed type_shared; exit 0; fi\n"
            "if [ \"$1 $2\" = \"pr view\" ]; then "
            "printf '%s\\n' \"$GH_STUB_PR_JSON\"; exit 0; fi\n"
            "if [ \"$1 $2\" = \"pr edit\" ]; then "
            "printf '%s\\n' \"$*\" >> \"$GH_STUB_LOG\"; exit 0; fi\n"
            "echo \"unexpected gh call: $*\" >&2; exit 2\n"
        )
        self.gh.chmod(0o755)
        self.env = os.environ.copy()
        self.env["PATH"] = f"{self.temp_path}:{self.env['PATH']}"
        self.live_pr = {
            "labels": [],
            "state": "OPEN",
            "baseRefName": "main",
            "headRefOid": "approved-sha",
            "updatedAt": "2026-09-28T12:00:00Z",
        }
        self.row = {
            "number": 42,
            "category": "LEGACY_ONLY",
            "proposed_label": "type_legacy",
            "base_ref": "main",
            "head_sha": "approved-sha",
            "updated_at": "2026-09-28T12:00:00Z",
        }

    def run_script(self, execute=False):
        self.env["GH_STUB_PR_JSON"] = json.dumps(self.live_pr)
        self.env["GH_STUB_LOG"] = str(self.gh_log)
        args = [
            "bash",
            str(SCRIPT.with_name("apply-labels.sh")),
            "--json",
            str(self.report),
        ]
        if execute:
            args.append("--execute")
        return subprocess.run(
            args,
            env=self.env,
            text=True,
            capture_output=True,
            check=True,
        )

    def test_matching_live_snapshot_is_labeled(self):
        self.report.write_text(json.dumps([self.row]))
        result = self.run_script()
        self.assertIn("gh pr edit 42", result.stdout)
        self.assertFalse(self.gh_log.exists())

    def test_execute_edits_labels_for_matching_snapshot(self):
        self.report.write_text(json.dumps([self.row]))
        self.run_script(execute=True)
        self.assertEqual(
            self.gh_log.read_text().strip(),
            "pr edit 42 -R kyverno/kyverno --add-label type_legacy",
        )

    def test_metadata_only_update_invalidates_report_snapshot(self):
        self.report.write_text(json.dumps([self.row]))
        self.live_pr["updatedAt"] = "2026-09-28T12:01:00Z"
        result = self.run_script(execute=True)
        self.assertIn("metadata changed since triage", result.stderr)
        self.assertNotIn("gh pr edit 42", result.stdout)
        self.assertFalse(self.gh_log.exists())

    def test_older_snapshot_without_update_time_is_skipped(self):
        self.row.pop("updated_at")
        self.report.write_text(json.dumps([self.row]))
        result = self.run_script(execute=True)
        self.assertIn("no captured head SHA/base/update timestamp", result.stderr)
        self.assertNotIn("gh pr edit 42", result.stdout)
        self.assertFalse(self.gh_log.exists())


if __name__ == "__main__":
    unittest.main()
