#!/usr/bin/env python3
"""Offline tests for scripts/publish-pss-upstream.py (requires PyYAML and git).

Each test pushes to a local bare repository that stands in for the fork, and
replaces GitHub with an in-memory fake. No network is used.
"""

import argparse
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from unittest import mock

import yaml


ROOT = Path(__file__).resolve().parents[1]
# Keep the import from writing scripts/__pycache__.
sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("publish_pss_upstream", ROOT / "scripts/publish-pss-upstream.py")
publish = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(publish)

REPO = "example/kyverno"
BOT = ("pss-bot", "pss-bot@example.com")
PIN = ("# The upstream Pod Security Admission commit that the PSS ValidatingPolicies\n"
       "# in charts/kyverno-policies are verified against.\n"
       "repository: https://github.com/kubernetes/pod-security-admission\n"
       "branch: master\n"
       "commit: " + "1" * 40 + "\n")


def git(cwd, *args, author=("person", "person@example.com")):
    env = dict(os.environ, GIT_AUTHOR_NAME=author[0], GIT_AUTHOR_EMAIL=author[1],
               GIT_COMMITTER_NAME=author[0], GIT_COMMITTER_EMAIL=author[1])
    result = subprocess.run(["git", "-C", str(cwd), *args], capture_output=True, text=True, env=env)
    if result.returncode != 0:
        raise AssertionError(f"git {args}: {result.stderr}")
    return result.stdout.strip()


class FakeGitHub(publish.GitHub):
    """In-memory PRs, labels, and comments behind the real request() guard."""

    def __init__(self, dry_run=False):
        super().__init__(REPO, dry_run=dry_run)
        self.prs, self.labels, self.calls = [], set(), []

    def request(self, method, path, body=None):
        self.check_allowed(method, path, body)
        self.calls.append((method, path))
        if self.dry_run and method != "GET":
            self.planned.append(f"{method} {path}")
            return {}
        m = re.fullmatch(rf"repos/{REPO}/pulls\?state=open&base=(.+)&head=example:(.+)", path)
        if method == "GET" and m:
            return [p for p in self.prs if p["state"] == "open" and p["base"] == m[1] and p["head"] == m[2]]
        if method == "POST" and path == f"repos/{REPO}/pulls":
            pr = dict(body, number=len(self.prs) + 1, node_id=f"PR_{len(self.prs) + 1}", state="open",
                      labels=[], comments=[])
            self.prs.append(pr)
            return pr
        m = re.fullmatch(rf"repos/{REPO}/(pulls|issues)/(\d+)(/labels|/comments.*)?", path)
        if m:
            pr = self.prs[int(m[2]) - 1]
            if method == "PATCH":
                pr.update(body)
                return pr
            if m[3] == "/labels":
                pr["labels"] += body["labels"]
                return pr["labels"]
            if m[3] and m[3].startswith("/comments"):
                if method == "POST":
                    pr["comments"].append(body)
                return pr["comments"]
        if path == "graphql":
            for pr in self.prs:
                if pr["node_id"] == body["variables"]["id"]:
                    pr["draft"] = True
            return {}
        if path == f"repos/{REPO}/labels/{publish.LABEL}":
            return {} if publish.LABEL in self.labels else None
        if method == "POST" and path == f"repos/{REPO}/labels":
            self.labels.add(body["name"])
            return body
        raise AssertionError(f"unexpected request {method} {path}")


class PublishTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.remote = self.root / "fork.git"
        git(self.root, "init", "-q", "--bare", "-b", "main", str(self.remote))
        self.clone = self.root / "clone"
        git(self.root, "clone", "-q", str(self.remote), str(self.clone))
        (self.clone / "scripts/pss-upstream").mkdir(parents=True)
        (self.clone / publish.PIN_PATH).write_text(PIN, encoding="utf-8", newline="\n")
        (self.clone / "README.md").write_text("kyverno\n", encoding="utf-8")
        git(self.clone, "add", "-A")
        git(self.clone, "commit", "-q", "-m", "base")
        git(self.clone, "push", "-q", "origin", "HEAD:main")
        self.github = FakeGitHub()
        self.runs = 0

    def tearDown(self):
        self.tmp.cleanup()

    def remote_sha(self, branch):
        out = git(self.root, "--git-dir", str(self.remote), "rev-parse", "-q", "--verify", f"refs/heads/{branch}") \
            if self.branch_exists(branch) else ""
        return out

    def branch_exists(self, branch):
        return subprocess.run(["git", "--git-dir", str(self.remote), "rev-parse", "-q", "--verify",
                               f"refs/heads/{branch}"], capture_output=True).returncode == 0

    def result(self, classification, head):
        return ({"repository": "https://github.com/kubernetes/pod-security-admission", "branch": "master",
                 "pin": "1" * 40, "head": head, "classification": classification, "groups": [],
                 "preview": "pass", "preview_summary": "checked 1 fixture results: 0 unexpected"},
                {"repository": "https://github.com/kubernetes/pod-security-admission", "branch": "master",
                 "commit": head})

    def run_publish(self, classification, head, dry_run=False, git_class=None):
        self.runs += 1
        work = self.root / f"work{self.runs}"
        work.mkdir()
        args = argparse.Namespace(repo=REPO, base="main", branch=publish.BOT_BRANCH, dry_run=dry_run,
                                  bot_name=BOT[0], bot_email=BOT[1])
        self.github.dry_run = dry_run
        repo = (git_class or publish.Git)("git", work, str(self.remote))
        result, candidate = self.result(classification, head)
        with mock.patch("sys.stderr", io.StringIO()):
            return publish.publish(args, self.github, repo, result, candidate, "# report\n")

    def bot_log(self):
        return git(self.root, "--git-dir", str(self.remote), "log", "--format=%H %P|%an|%ae|%cn|%ce|%B%x1e",
                   f"main..{publish.BOT_BRANCH}")

    def pinned_commit(self, ref=publish.BOT_BRANCH):
        text = git(self.root, "--git-dir", str(self.remote), "show", f"{ref}:{publish.PIN_PATH}")
        return yaml.safe_load(text)["commit"], text

    def test_fixtures_only_creates_a_ready_pr(self):
        self.assertEqual(self.run_publish("fixtures-only", "2" * 40), 0)
        commit, text = self.pinned_commit()
        self.assertEqual(commit, "2" * 40)
        self.assertTrue(text.startswith("# The upstream Pod Security Admission commit"), text)
        records = [r for r in self.bot_log().split("\x1e") if r.strip()]
        self.assertEqual(len(records), 1)
        _, name, email, cname, cemail, body = records[0].strip().split("|", 5)
        self.assertEqual((name, email, cname, cemail), (BOT[0], BOT[1], BOT[0], BOT[1]))
        self.assertIn(f"Signed-off-by: {BOT[0]} <{BOT[1]}>", body)
        self.assertTrue(body.startswith("chore(pss): sync with upstream Pod Security Admission 2222222"))
        [pr] = self.github.prs
        self.assertFalse(pr["draft"])
        self.assertEqual(pr["labels"], [])
        self.assertIn(publish.BODY_MARKER, pr["body"])

    def test_rerun_with_the_same_head_adds_no_commit(self):
        self.run_publish("fixtures-only", "2" * 40)
        tip = self.remote_sha(publish.BOT_BRANCH)
        self.assertEqual(self.run_publish("fixtures-only", "2" * 40), 0)
        self.assertEqual(self.remote_sha(publish.BOT_BRANCH), tip)
        self.assertEqual(len(self.github.prs), 1)

    def test_manual_change_creates_a_labeled_draft(self):
        self.assertEqual(self.run_publish("manual", "3" * 40), 0)
        [pr] = self.github.prs
        self.assertTrue(pr["draft"])
        self.assertEqual(pr["labels"], [publish.LABEL])
        self.assertIn(publish.LABEL, self.github.labels)
        self.assertIn("[!IMPORTANT]", pr["body"])

    def test_manual_update_fast_forwards_and_turns_the_pr_into_a_draft(self):
        self.run_publish("fixtures-only", "2" * 40)
        first = self.remote_sha(publish.BOT_BRANCH)
        self.assertEqual(self.run_publish("manual", "3" * 40), 0)
        second = self.remote_sha(publish.BOT_BRANCH)
        self.assertEqual(git(self.root, "--git-dir", str(self.remote), "rev-parse", f"{second}^"), first)
        [pr] = self.github.prs
        self.assertTrue(pr["draft"])
        self.assertEqual(pr["labels"], [publish.LABEL])
        self.assertEqual(self.pinned_commit()[0], "3" * 40)

    def test_a_draft_is_never_marked_ready(self):
        self.run_publish("manual", "3" * 40)
        self.run_publish("fixtures-only", "4" * 40)
        [pr] = self.github.prs
        self.assertTrue(pr["draft"])
        self.assertNotIn(("POST", "graphql"), [c for c in self.github.calls if "ready" in str(c).lower()])

    def test_foreign_commit_pauses_with_one_comment(self):
        self.run_publish("fixtures-only", "2" * 40)
        git(self.clone, "fetch", "-q", "origin")
        git(self.clone, "checkout", "-q", "-B", "work", f"origin/{publish.BOT_BRANCH}")
        (self.clone / "README.md").write_text("edited by a maintainer\n", encoding="utf-8")
        git(self.clone, "commit", "-q", "-am", "maintainer fix")
        git(self.clone, "push", "-q", "origin", f"HEAD:{publish.BOT_BRANCH}")
        tip = self.remote_sha(publish.BOT_BRANCH)
        self.assertEqual(self.run_publish("manual", "3" * 40), 3)
        self.assertEqual(self.run_publish("manual", "3" * 40), 3)
        self.assertEqual(self.remote_sha(publish.BOT_BRANCH), tip)
        [pr] = self.github.prs
        self.assertEqual(len(pr["comments"]), 1)
        self.assertIn(publish.PAUSE_MARKER, pr["comments"][0]["body"])
        self.assertFalse(pr["draft"])

    def test_merged_branch_restarts_from_the_base(self):
        self.run_publish("fixtures-only", "2" * 40)
        git(self.clone, "fetch", "-q", "origin")
        git(self.clone, "checkout", "-q", "-B", "main", f"origin/{publish.BOT_BRANCH}")
        (self.clone / "README.md").write_text("later main change\n", encoding="utf-8")
        git(self.clone, "commit", "-q", "-am", "later")
        git(self.clone, "push", "-q", "origin", "HEAD:main")
        self.github.prs[0]["state"] = "closed"
        main = self.remote_sha("main")
        self.assertEqual(self.run_publish("fixtures-only", "5" * 40), 0)
        tip = self.remote_sha(publish.BOT_BRANCH)
        self.assertEqual(git(self.root, "--git-dir", str(self.remote), "rev-parse", f"{tip}^"), main)
        self.assertEqual(len(self.github.prs), 2)

    def test_dry_run_writes_nothing(self):
        self.assertEqual(self.run_publish("manual", "3" * 40, dry_run=True), 0)
        self.assertFalse(self.branch_exists(publish.BOT_BRANCH))
        self.assertEqual(self.github.prs, [])
        planned = " ".join(self.github.planned)
        self.assertIn(f"to {publish.BOT_BRANCH}", planned)
        self.assertIn(f"POST repos/{REPO}/pulls", planned)
        self.assertIn(f"add the {publish.LABEL} label", planned)

    def test_concurrent_change_rejects_the_push(self):
        self.run_publish("fixtures-only", "2" * 40)
        test = self

        class RacingGit(publish.Git):
            def run(self, *args, **kwargs):
                if args and args[0] == "push":
                    # Another bot run moves the branch between the read and the push.
                    git(test.clone, "fetch", "-q", "origin")
                    git(test.clone, "checkout", "-q", "-B", "race", f"origin/{publish.BOT_BRANCH}")
                    git(test.clone, "commit", "-q", "--allow-empty", "-m", "race", author=BOT)
                    git(test.clone, "push", "-q", "origin", f"HEAD:{publish.BOT_BRANCH}")
                return super().run(*args, **kwargs)

        with self.assertRaisesRegex(publish.PublishError, "git push"):
            self.run_publish("manual", "3" * 40, git_class=RacingGit)
        self.assertEqual(git(self.root, "--git-dir", str(self.remote), "log", "-1", "--format=%s",
                             publish.BOT_BRANCH), "race")

    def test_missing_pin_on_the_base_is_an_error(self):
        git(self.clone, "checkout", "-q", "main")
        git(self.clone, "rm", "-q", publish.PIN_PATH)
        git(self.clone, "commit", "-q", "-m", "drop pin")
        git(self.clone, "push", "-q", "origin", "HEAD:main")
        with self.assertRaisesRegex(publish.PublishError, "doesn't exist"):
            self.run_publish("fixtures-only", "2" * 40)


class BodyTest(unittest.TestCase):
    result = {"classification": "manual", "head": "3" * 40, "preview": "fail",
              "preview_summary": "checked 1 fixture results: 1 unexpected"}

    def test_short_report_is_kept_whole(self):
        body = publish.pr_body(self.result, "# report\nline\n")
        self.assertTrue(body.endswith("# report\nline\n"))
        self.assertNotIn("The report is cut here", body)

    def test_long_report_is_cut_under_the_github_limit(self):
        report = "# report\n" + "".join(f"+ go diff line {i}\n" for i in range(20000))
        body = publish.pr_body(self.result, report)
        self.assertLess(len(body), 65536)
        self.assertLessEqual(len(body), publish.MAX_BODY)
        self.assertIn("The report is cut here", body)
        self.assertIn("pss-upstream-sync` artifact", body)
        self.assertIn(publish.BODY_MARKER, body)
        # The cut lands on a line boundary.
        self.assertRegex(body, r"\+ go diff line \d+\n\n\*\*The report is cut here")


class GuardTest(unittest.TestCase):
    def test_forbidden_requests_are_refused(self):
        github = publish.GitHub(REPO)
        forbidden = [
            ("PUT", f"repos/{REPO}/pulls/1/merge", None),
            ("POST", f"repos/{REPO}/pulls/1/reviews", {"event": "APPROVE"}),
            ("DELETE", f"repos/{REPO}/git/refs/heads/bot", None),
            ("PATCH", f"repos/{REPO}/pulls/1", {"state": "closed"}),
            ("POST", "graphql", {"query": "mutation { markPullRequestReadyForReview(input: {}) { clientMutationId } }"}),
        ]
        for method, path, body in forbidden:
            with self.assertRaises(publish.PublishError, msg=f"{method} {path}"):
                github.request(method, path, body)

    def test_allowed_draft_conversion_passes_the_guard(self):
        publish.GitHub.check_allowed("POST", "graphql", {"query": "mutation { convertPullRequestToDraft }"})


class MainTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self.tmp.name)

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, classification, commit="2" * 40, pin_commit=None):
        (self.dir / "result.json").write_text(json.dumps({"classification": classification, "head": commit}))
        (self.dir / "upstream.yaml").write_text(yaml.safe_dump({
            "repository": "https://github.com/kubernetes/pod-security-admission", "branch": "master",
            "commit": pin_commit or commit}))
        (self.dir / "report.md").write_text("# report\n")

    def main(self, *argv, env=None):
        out = io.StringIO()
        with mock.patch.dict(os.environ, env or {}, clear=False), redirect_stdout(out), \
             mock.patch("sys.stderr", io.StringIO()):
            return publish.main(["--result", str(self.dir), *argv]), out.getvalue()

    def test_no_change_publishes_nothing(self):
        self.write("none")
        self.assertEqual(self.main("--repo", REPO)[0], 0)

    def test_errors_exit_1(self):
        self.write("manual")
        self.assertEqual(self.main("--repo", "not a repo")[0], 1)
        self.assertEqual(self.main("--repo", REPO, "--branch", "a..b")[0], 1)
        with mock.patch.dict(os.environ, {}, clear=True):
            code, out = self.main("--repo", REPO)
        self.assertEqual((code, "APP_SLUG" in out), (1, True))
        self.write("manual", pin_commit="3" * 40)
        code, out = self.main("--repo", REPO, env={"BOT_NAME": "b", "BOT_EMAIL": "b@example.com"})
        self.assertEqual((code, "different commits" in out), (1, True))
        self.write("mystery")
        self.assertEqual(self.main("--repo", REPO)[0], 1)


if __name__ == "__main__":
    unittest.main(verbosity=2)
