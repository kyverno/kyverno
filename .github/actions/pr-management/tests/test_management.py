import copy
import datetime
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import unittest
import uuid


ACTION = Path(__file__).resolve().parents[1]
ROOT = ACTION.parents[2]
spec = importlib.util.spec_from_file_location("management", ACTION / "run.py")
management = importlib.util.module_from_spec(spec)
spec.loader.exec_module(management)
SHA = "a" * 40
NOW = datetime.datetime.now(datetime.timezone.utc)


def at(days):
    return (NOW - datetime.timedelta(days=days)).strftime("%Y-%m-%dT%H:%M:%SZ")


def connection(nodes, more=False):
    return {"nodes": nodes, "pageInfo": {
        "hasNextPage": more, "endCursor": "next", "hasPreviousPage": more, "startCursor": "previous",
    }}


def labels(*names):
    return [{"name": name} for name in names]


def readiness():
    return {
        "number": 1, "state": "OPEN", "isDraft": False, "author": {"login": "contributor"},
        "labels": connection([]),
        "commits": {"nodes": [{"commit": {"oid": SHA, "statusCheckRollup": {
            "contexts": connection([{"name": "unit", "conclusion": "SUCCESS", "status": "COMPLETED"}]),
        }}}]},
        "reviewThreads": connection([]),
    }


def stale(stale_days=8, label_actor="Bot", warning_days=8):
    return {
        "state": "OPEN", "isDraft": False, "headRefOid": SHA,
        "author": {"login": "contributor"}, "labels": connection(labels("needs-author-action", "stale")),
        "commits": {"nodes": [{"commit": {"oid": SHA, "committedDate": at(40)}}]},
        "assignees": connection([]), "reviewRequests": connection([]),
        "reviews": connection([]), "comments": connection([{
            "author": {"login": "github-actions[bot]", "__typename": "Bot"},
            "createdAt": at(warning_days),
            "body": f"<!-- pr-stale-author-action: stale {int(datetime.datetime.strptime(at(40), '%Y-%m-%dT%H:%M:%SZ').replace(tzinfo=datetime.timezone.utc).timestamp())} {SHA} -->",
        }]),
        "timelineItems": connection([
            {"__typename": "PullRequestCommit", "commit": {"oid": SHA, "committedDate": at(40)}},
            {"__typename": "LabeledEvent", "label": {"name": "needs-author-action"}, "createdAt": at(35)},
            {"__typename": "LabeledEvent", "label": {"name": "stale"}, "createdAt": at(stale_days),
             "actor": {"__typename": label_actor}},
        ]),
    }


class AutomationTests(unittest.TestCase):
    def setUp(self):
        self.work = ROOT / f".pr-management-test-{uuid.uuid4().hex}"
        (self.work / ".github").mkdir(parents=True)
        (self.work / "bin").mkdir()
        shutil.copyfile(ACTION / "tests/gh", self.work / "bin/gh")
        (self.work / "bin/gh").chmod(0o755)
        (self.work / "bin/sleep").write_text("#!/bin/sh\nexit 0\n")
        (self.work / "bin/sleep").chmod(0o755)
        for file in ("pr-management.json", "pr-readiness-checks.yml", "labels.yml"):
            shutil.copyfile(ROOT / ".github" / file, self.work / ".github" / file)
        self.fixture = {
            "readiness_pr": readiness(),
            "live_pr": {"state": "open", "draft": False, "head": {"sha": SHA},
                        "labels": labels("needs-author-action", "stale")},
        }
        self.env = dict(os.environ, PATH=f"{self.work / 'bin'}:{os.environ['PATH']}",
                        GH_FIXTURE=str(self.work / "fixture.json"), GH_CALLS=str(self.work / "calls.jsonl"),
                        GITHUB_REPOSITORY="kyverno/example", GITHUB_WORKSPACE=str(self.work),
                        GITHUB_EVENT_PATH=str(self.work / "event.json"), GITHUB_EVENT_NAME="pull_request_target",
                        GITHUB_OUTPUT=str(self.work / "output"), PR_MODE="readiness", DRY_RUN="false",
                        PYTHONDONTWRITEBYTECODE="1", STALE_DAYS="7", NUDGE_DAYS="7",
                        CLOSE_DAYS="14", WARNING_DAYS="7", BACKLOG_DAYS="30",
                        FALLBACK_REVIEWERS="@kyverno/reviewers", EXEMPT_LABELS='["no-stale","hold","hold-merge"]')
        self.event = {"pull_request": {"number": 1, "draft": False, "user": {"login": "contributor"}}}

    def tearDown(self):
        shutil.rmtree(self.work)

    def run_action(self, mode="readiness", dry=False, success=True, shell=None):
        (self.work / "fixture.json").write_text(json.dumps(self.fixture))
        (self.work / "event.json").write_text(json.dumps(self.event))
        env = dict(self.env, PR_MODE=mode, DRY_RUN=str(dry).lower())
        command = ["python3", str(ACTION / "run.py")]
        if shell:
            env["PR_MANAGEMENT_LIBRARY_ONLY"] = "true"
            command = ["bash", "-c", f'source "{ACTION}/stale.sh"; {shell}']
        result = subprocess.run(command, env=env, cwd=self.work, capture_output=True, text=True, timeout=90)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        calls = self.work / "calls.jsonl"
        self.calls = [json.loads(line) for line in calls.read_text().splitlines()] if calls.exists() else []
        self.mutations = [
            call for call in self.calls if call[:1] == ["pr"] and call[1] in {"edit", "comment", "close"}
            or "--method" in call
        ]
        return result

    def assert_not_ready(self):
        self.assertFalse(any("--add-label" in call and "ready-for-review" in call for call in self.mutations))
        self.assertFalse(any("--remove-label" in call and "needs-author-action" in call for call in self.mutations))

    def test_clean_ready_and_production_mutation(self):
        self.run_action()
        self.assertTrue(any("--add-label" in call and "ready-for-review" in call for call in self.mutations))

    def test_pending_and_empty_ci_never_ready(self):
        for contexts in ([], [{"name": "unit", "status": "IN_PROGRESS", "conclusion": None}],
                         [{"context": "legacy", "state": "PENDING"}], [{}],
                         [{"name": "unit", "status": "COMPLETED", "conclusion": None}]):
            with self.subTest(contexts=contexts):
                self.fixture["readiness_pr"]["commits"]["nodes"][0]["commit"]["statusCheckRollup"]["contexts"]["nodes"] = contexts
                self.run_action()
                self.assert_not_ready()

    def test_missing_rollup_is_pending(self):
        self.fixture["readiness_pr"]["commits"]["nodes"][0]["commit"]["statusCheckRollup"] = None
        self.run_action()
        self.assert_not_ready()

    def test_gated_and_failed_approval_lookup(self):
        self.fixture["gated_count"] = 1
        self.run_action()
        self.assertFalse(any("ready-for-review" in c and "--add-label" in c for c in self.mutations))
        self.assertTrue(any("workflow-approval-required" in c for c in self.mutations))
        (self.work / "calls.jsonl").unlink()
        self.fixture["gated_failure"] = True
        self.run_action()
        self.assertEqual(self.mutations, [])

    def test_review_thread_pagination_blocks_and_marker_deduplicates(self):
        self.fixture["readiness_pr"]["reviewThreads"] = connection([], more=True)
        self.fixture["thread_page"] = connection([{
            "isResolved": False, "comments": {"nodes": [{"author": {"login": "reviewer"}, "createdAt": at(1)}]},
        }])
        self.fixture["comment_bodies"] = "<!-- pr-readiness-check: threads -->"
        self.run_action()
        self.assert_not_ready()
        self.assertTrue(any("needs-author-action" in call for call in self.mutations))
        self.assertFalse(any(call[:2] == ["pr", "comment"] for call in self.mutations))
        self.assertTrue(any("--paginate" in call for call in self.calls))

    def test_truncated_pagination_never_clears_author_or_marks_ready(self):
        self.fixture["readiness_pr"]["labels"] = connection(
            labels("needs-author-action", "workflow-approval-required"), more=True)
        self.run_action()
        self.assertEqual(self.mutations, [])

    def test_milestone_stamp_is_configured_and_disabled_by_empty_prefix(self):
        self.env["GITHUB_EVENT_NAME"] = "schedule"
        self.fixture["discovery"] = {
            "pageInfo": {"hasNextPage": False, "endCursor": None},
            "nodes": [{"number": 1, "isDraft": False, "closingIssuesReferences": connection([{
                "repository": {"nameWithOwner": "kyverno/example"},
                "milestone": {"state": "OPEN", "title": "Kyverno Release 1.18"},
            }])}],
        }
        self.run_action()
        self.assertTrue(any("--add-label" in call and "milestone-pr" in call for call in self.mutations))
        (self.work / "calls.jsonl").unlink()
        path = self.work / ".github/pr-management.json"
        config = json.loads(path.read_text())
        config["milestone_prefix"] = ""
        path.write_text(json.dumps(config))
        self.run_action()
        self.assertFalse(any("milestone-pr" in call for call in self.mutations))

    def test_partial_and_error_data_fail_closed(self):
        for field in ("labels", "reviewThreads"):
            with self.subTest(field=field):
                self.fixture["readiness_pr"][field] = None
                self.run_action(success=False)
                self.assertEqual(self.mutations, [])
                self.fixture["readiness_pr"] = readiness()
        self.fixture["graphql_errors"] = True
        self.run_action(success=False)
        self.assertEqual(self.mutations, [])

    def test_sweep_isolates_per_pr_fetch_failures(self):
        self.env["GITHUB_EVENT_NAME"] = "schedule"
        self.fixture["discovery"] = {
            "pageInfo": {"hasNextPage": False, "endCursor": None},
            "nodes": [{"number": n, "isDraft": False, "closingIssuesReferences": connection([])} for n in (1, 2)],
        }
        self.fixture["fail_detail_numbers"] = [1]
        result = self.run_action(success=False)
        self.assertIn("1 PR detail fetch(es) failed", result.stdout + result.stderr)
        self.assertFalse(any(call[:3] == ["pr", "edit", "1"] for call in self.mutations))
        self.assertTrue(any(call[:3] == ["pr", "edit", "2"] and "ready-for-review" in call
                            for call in self.mutations))

    def test_readiness_head_and_draft_races(self):
        for live in ({"head": {"sha": "b" * 40}}, {"draft": True}, {"state": "closed"}):
            with self.subTest(live=live):
                saved = copy.deepcopy(self.fixture["live_pr"])
                self.fixture["live_pr"].update(live)
                self.run_action()
                self.assertEqual(self.mutations, [])
                self.fixture["live_pr"] = saved

    def test_readiness_dry_run(self):
        result = self.run_action(dry=True)
        self.assertEqual(self.mutations, [])
        self.assertIn("[dry-run]", result.stdout)

    def test_draft_transition_clears_three_labels_and_preview(self):
        self.event["pull_request"]["draft"] = True
        self.fixture["live_pr"].update(draft=True, labels=labels(
            "ready-for-review", "needs-author-action", "workflow-approval-required"))
        self.run_action()
        self.assertEqual(len(self.mutations), 3)
        (self.work / "calls.jsonl").unlink()
        self.run_action(mode="stale", dry=True)
        self.assertEqual(self.mutations, [])

    def test_label_rendering_and_preview_no_api(self):
        result = self.run_action(mode="labels", dry=True)
        self.assertEqual(self.calls, [])
        self.assertIn("no labeler or merge-conflict mutations", result.stdout)
        output = (self.work / "output").read_text().split("=", 1)[1].strip()
        generated = json.loads(Path(output).read_text())
        self.assertEqual(generated["area/api"][0]["changed-files"][0]["any-glob-to-any-file"], ["api/**"])
        self.assertNotIn("needs-author-action", generated)

    def test_sync_idempotency_preview_and_failure(self):
        _, metadata = management.render_labels(management.yaml_file(self.work / ".github/labels.yml"))
        self.fixture["label_pages"] = [metadata]
        self.run_action(mode="sync-labels")
        self.assertEqual(self.mutations, [])
        self.fixture["label_pages"] = [[]]
        self.run_action(mode="sync-labels", dry=True)
        self.assertEqual(self.mutations, [])
        self.fixture["labels_failure"] = True
        self.run_action(mode="sync-labels", success=False)

    def test_sync_write_errors_fail(self):
        self.fixture["write_failure"] = True
        self.run_action(mode="sync-labels", success=False)

    def test_validate_offline_does_not_evaluate_checks(self):
        for key in ("GH_TOKEN", "GITHUB_TOKEN", "GITHUB_EVENT_PATH", "GITHUB_EVENT_NAME", "GITHUB_OUTPUT"):
            self.env.pop(key, None)
        (self.work / ".github/pr-readiness-checks.yml").write_text(
            'never:\n  test: \'error("must not execute")\'\n  message: \'"unused"\'\n')
        self.run_action(mode="validate")
        self.assertEqual(self.calls, [])
        (self.work / ".github/pr-readiness-checks.yml").write_text("bad:\n  test: 'invalid('\n  message: 'null'\n")
        self.run_action(mode="validate", success=False)
        self.assertEqual(self.calls, [])

    def test_validate_module_rejects_top_level_expression_injection(self):
        # Escape the check's generated delimiters: an imported module must reject a program.
        program = '.author.login as $author | []; error("must not execute")'
        folder = self.work / "module"
        folder.mkdir()
        (folder / "checks.jq").write_text(f'def readiness: ({program}); error("must not execute");')
        result = subprocess.run(
            ["jq", "-L", str(folder), "-n", 'import "checks" as checks; null'],
            capture_output=True, text=True, timeout=10,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("compile error", result.stderr)

    def test_action_and_workflow_syntax_and_preview_guards(self):
        action = management.yaml_file(ACTION / "action.yml")
        self.assertEqual(action["runs"]["using"], "composite")
        for step in action["runs"]["steps"]:
            if step.get("uses", "").startswith(("actions/labeler@", "eps1lon/")):
                self.assertIn("inputs.dry-run == 'false'", step["if"])
        for name in ("pr-labelling", "pr-readiness-check", "pr-stale-author-action",
                     "sync-label-colors", "check-pr-management"):
            workflow = management.yaml_file(ROOT / ".github/workflows" / f"{name}.yaml")
            for job in workflow["jobs"].values():
                self.assertGreater(job["timeout-minutes"], 0)

    def test_invalid_config_fails_before_api(self):
        config = json.loads((self.work / ".github/pr-management.json").read_text())
        for change in ({"warning_days": 0}, {"close_days": True}, {"close_days": 2.5},
                       {"backlog_days": 3651}, {"warning_days": 15}, {"unknown": 1},
                       {"exempt_labels": "hold"}, {"fallback_reviewers": "$(touch unsafe)"}):
            with self.subTest(change=change):
                (self.work / ".github/pr-management.json").write_text(json.dumps(dict(config, **change)))
                self.run_action(mode="validate", success=False)
                self.assertEqual(self.calls, [])
        config.pop("warning_days")
        (self.work / ".github/pr-management.json").write_text(json.dumps(config))
        self.run_action(mode="validate", success=False)
        self.assertEqual(self.calls, [])

    def test_invalid_overrides_and_label_rules_fail_before_api(self):
        for key, value in (("DRY_RUN", "yes"), ("STALE_DAYS", "0"), ("STALE_DAYS", "15"),
                           ("NUDGE_DAYS", "7.5"), ("NUDGE_DAYS", "3651")):
            with self.subTest(key=key, value=value):
                env = dict(self.env)
                self.env[key] = value
                # dry-run is set explicitly by run_action, so test it through the entry point.
                (self.work / "fixture.json").write_text(json.dumps(self.fixture))
                result = subprocess.run(
                    ["python3", str(ACTION / "run.py")], cwd=self.work,
                    env=dict(self.env, PR_MODE="validate"), capture_output=True, text=True, timeout=10,
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((self.work / "calls.jsonl").exists())
                self.env = env
        (self.work / ".github/labels.yml").write_text(
            'example:\n  color: "abcdef"\n  rules:\n    - changed-files:\n        - unsupported-matcher: ["**"]\n')
        self.run_action(mode="validate", success=False)
        self.assertEqual(self.calls, [])

    def test_stale_grace_requires_seven_days_after_actual_bot_warning(self):
        for label_actor, label_days, warning_days, should_close in (
            ("Bot", 0, 0, False), ("Bot", 6, 6, False), ("Bot", 7, 7, True), ("Bot", 8, 8, True),
            ("User", 20, 1, False), ("User", 20, 7, True), ("User", 20, 8, True),
        ):
            with self.subTest(actor=label_actor, label_days=label_days, warning_days=warning_days):
                (self.work / "calls.jsonl").unlink(missing_ok=True)
                self.fixture["stale_pr"] = stale(label_days, label_actor, warning_days)
                self.run_action(shell="handle_close 1")
                self.assertEqual(any(call[:2] == ["pr", "close"] for call in self.mutations), should_close)

    def test_manual_stale_label_without_bot_warning_never_closes(self):
        self.fixture["stale_pr"] = stale(20, "User", 20)
        self.fixture["stale_pr"]["comments"]["nodes"][0]["author"]["__typename"] = "User"
        self.run_action(shell="handle_close 1")
        self.assertEqual(self.mutations, [])

    def test_stale_close_dry_run(self):
        self.fixture["stale_pr"] = stale()
        self.run_action(dry=True, shell="handle_close 1")
        self.assertEqual(self.mutations, [])

    def test_holds_and_state_guards_in_all_handlers(self):
        for change in ({"state": "CLOSED"}, {"isDraft": True},
                       {"labels": connection(labels("needs-author-action", "stale", "ready-for-review", "hold"))},
                       {"labels": connection(labels("needs-author-action", "stale", "ready-for-review", "hold-merge"))}):
            with self.subTest(change=change):
                self.fixture["stale_pr"] = dict(stale(), **change)
                for handler in ("handle_stale_label", "handle_needs_author", "handle_close", "handle_ready"):
                    self.run_action(shell=f"{handler} 1", success="state" not in change and "isDraft" not in change)
                    if handler == "handle_stale_label" and "labels" in change:
                        # Exemption resets the stale cycle instead of preserving the old clock.
                        self.assertEqual(self.mutations, [["pr", "edit", "1", "--repo", "kyverno/example",
                                                           "--remove-label", "stale"]])
                    else:
                        self.assertEqual(self.mutations, [])
                    (self.work / "calls.jsonl").unlink(missing_ok=True)

    def test_force_push_resets_cycle_and_close_rechecks_live_hold(self):
        self.fixture["stale_pr"] = stale()
        self.fixture["stale_pr"]["timelineItems"]["nodes"].append({
            "__typename": "HeadRefForcePushedEvent", "createdAt": at(1)})
        self.run_action(shell="handle_close 1")
        self.assertEqual(self.mutations, [])
        self.fixture["stale_pr"] = stale()
        self.fixture["live_pr"]["labels"].append({"name": "hold"})
        self.run_action(shell="handle_close 1", success=False)
        self.assertEqual(self.mutations, [])

    def test_force_push_clears_stale_and_timeline_history_is_paginated(self):
        self.fixture["stale_pr"] = stale()
        timeline = self.fixture["stale_pr"]["timelineItems"]["nodes"]
        self.fixture["timeline_page"] = connection(timeline)
        self.fixture["stale_pr"]["timelineItems"] = connection([{
            "__typename": "HeadRefForcePushedEvent", "createdAt": at(1),
        }], more=True)
        self.run_action(shell="handle_stale_label 1")
        self.assertTrue(any("--remove-label" in call and "stale" in call for call in self.mutations))
        self.assertTrue(any("cursor=previous" in call for call in self.calls))

    def test_missing_stale_history_never_mutates(self):
        self.fixture["stale_pr"] = stale()
        self.fixture["stale_pr"]["labels"]["pageInfo"]["hasNextPage"] = True
        self.run_action(shell="handle_close 1", success=False)
        self.assertEqual(self.mutations, [])

    def test_old_backlog_marked_and_not_closed_same_sweep(self):
        self.fixture["stale_pr"] = stale()
        self.fixture["stale_pr"]["labels"] = connection(labels("needs-author-action"))
        self.fixture["stale_pr"]["timelineItems"]["nodes"].pop()
        self.fixture["stale_pr"]["comments"] = connection([])
        self.env["GITHUB_EVENT_NAME"] = "schedule"
        self.run_action(mode="stale")
        self.assertTrue(any("--add-label" in call and "stale" in call for call in self.mutations))
        self.assertFalse(any(call[:2] == ["pr", "close"] for call in self.mutations))

    def test_stale_bot_marker_on_older_comment_page_avoids_duplicate(self):
        self.fixture["stale_pr"] = stale()
        pr = self.fixture["stale_pr"]
        pr["labels"] = connection(labels("ready-for-review"))
        pr["timelineItems"]["nodes"].append({
            "__typename": "LabeledEvent", "label": {"name": "ready-for-review"}, "createdAt": at(10)})
        pr["comments"] = connection([], more=True)
        self.fixture["comment_page"] = connection([{
            "author": {"login": "github-actions[bot]", "__typename": "Bot"},
            "createdAt": at(8), "body": f"<!-- pr-review-nudge: {SHA} -->",
        }])
        self.run_action(shell="handle_ready 1")
        self.assertEqual(self.mutations, [])
        self.assertTrue(any("cursor=previous" in call for call in self.calls))

    def test_author_replies_do_not_reset_author_clock(self):
        self.fixture["stale_pr"] = stale()
        self.fixture["stale_pr"]["comments"]["nodes"].append({
            "author": {"login": "contributor", "__typename": "User"}, "createdAt": at(0), "body": "still here",
        })
        self.run_action(shell="handle_close 1")
        self.assertTrue(any(call[:2] == ["pr", "close"] for call in self.mutations))

    def test_unobserved_late_commit_starts_conservative_clock(self):
        self.fixture["stale_pr"] = stale()
        pr = self.fixture["stale_pr"]
        pr["labels"] = connection(labels("needs-author-action"))
        pr["timelineItems"]["nodes"].append({
            "__typename": "PullRequestCommit", "commit": {"oid": SHA, "committedDate": at(40)},
        })
        self.run_action(shell="handle_needs_author 1")
        self.assertTrue(any(call[:2] == ["pr", "comment"] for call in self.mutations))
        self.assertFalse(any("--add-label" in call for call in self.mutations))


if __name__ == "__main__":
    unittest.main()
