import contextlib
import hashlib
import fcntl
import importlib.util
import io
import json
import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

HERE = Path(__file__).parent
SPEC = importlib.util.spec_from_file_location("retarget", HERE / "retarget.py")
retarget = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(retarget)

REPO = "kyverno/kyverno"
FORK = "alice/kyverno"
TEMPLATE = "## Explanation\n\n<!-- why -->\n\n## Checklist\n\n```yaml\n# not a heading\n```\n"
GIT_ENV = {
    "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
    "GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
    "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
}


def sh(cwd, *args):
    return subprocess.run(["git", "-C", str(cwd)] + list(args), check=True, capture_output=True,
                          text=True, timeout=60).stdout.strip()


class FakeGH:
    """In-memory GitHub; PR head SHAs are read from the local bare repos."""

    def __init__(self, root):
        self.root = root
        self.calls, self.writes = [], []
        self.fail_after = set()
        self.fail_before = set()
        self.api_head = None
        self.base = "main"
        self.original = {"state": "open", "merged": False, "updated_at": "2026-10-01T00:00:00Z",
                         "maintainer_can_modify": True, "author": "alice", "title": "fix legacy"}
        self.comment = {"user": {"login": "alice"}, "issue_url": f"https://api.github.com/repos/{REPO}/issues/7",
                        "body": None}
        self.siblings = {}

    def head(self, repo, branch):
        res = subprocess.run(["git", f"--git-dir={self.root / f'{repo}.git'}", "rev-parse", "--verify", "--quiet",
                              f"refs/heads/{branch}"], capture_output=True, text=True, timeout=30)
        return res.stdout.strip() or None

    def pull(self, n):
        if n == 7:
            o = self.original
            return {"number": 7, "state": o["state"], "merged": o["merged"], "draft": o.get("draft", False), "updated_at": o["updated_at"],
                    "maintainer_can_modify": o["maintainer_can_modify"], "user": {"login": o["author"]},
                    "title": o["title"],
                    "base": {"ref": self.base, "repo": {"full_name": REPO}},
                    "head": {"ref": "feature", "sha": self.api_head or self.head(FORK, "feature"),
                             "repo": {"full_name": FORK}}}
        s = self.siblings[n]
        return {"number": n, "state": s["state"], "merged": s["merged"], "body": s.get("body"),
                "user": {"login": s.get("author", "someone")},
                "base": {"ref": s["base"], "repo": {"full_name": REPO}},
                "head": {"ref": s["branch"], "sha": s.get("api_head") or self.head(REPO, s["branch"]),
                         "repo": {"full_name": REPO}}}

    def __call__(self, args, timeout=None):
        args = list(args)
        self.calls.append(args)
        key = " ".join(args[:2])
        if key in self.fail_before:
            self.fail_before.discard(key)
            raise retarget.RetargetError(f"simulated {key} failure")
        if args[0] == "api" and "/pulls/" in args[1]:
            return json.dumps(self.pull(int(args[1].rsplit("/", 1)[1])))
        if args == ["api", "user"]:
            return json.dumps({"login": "maint"})
        if args[0] == "api" and "/issues/comments/" in args[1]:
            return json.dumps(self.comment)
        if key == "pr list":
            branch = args[args.index("--head") + 1]
            return json.dumps([{"number": n, "url": f"https://github.com/{REPO}/pull/{n}",
                                "state": "MERGED" if s["merged"] else s["state"].upper(),
                                "headRefName": s["branch"], "headRefOid": self.head(REPO, s["branch"]),
                                "baseRefName": s["base"], "isCrossRepository": False}
                               for n, s in self.siblings.items() if s["branch"] == branch])
        self.writes.append(args)
        if key == "pr edit":
            self.base = args[args.index("--base") + 1]
        elif key == "pr create":
            n = 100 + len(self.siblings)
            self.siblings[n] = {"branch": args[args.index("--head") + 1], "base": args[args.index("--base") + 1],
                                "state": "open", "merged": False, "author": "maint",
                                "body": args[args.index("--body") + 1]}
            self.created = n
        elif key == "pr close":
            self.siblings[int(args[2])]["state"] = "closed"
        else:
            raise AssertionError(f"unexpected gh call {args}")
        if key in self.fail_after:
            self.fail_after.discard(key)
            raise retarget.RetargetError("timed out after 60s, outcome unknown")
        if key == "pr create":
            return f"https://github.com/{REPO}/pull/{n}\n"
        return ""


class Base(unittest.TestCase):
    def setUp(self):
        env = patch.dict(
            os.environ,
            {**{k: v for k, v in os.environ.items() if not k.startswith("GIT_")}, **GIT_ENV},
            clear=True,
        )
        env.start()
        self.addCleanup(env.stop)
        tmp = tempfile.TemporaryDirectory(dir=os.environ.get("RETARGET_TEST_TMPDIR", HERE), prefix=".rt-test-")
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name)
        # retarget.py strips GIT_*, so isolate its global config through HOME/XDG instead.
        (self.root / "home").mkdir()
        home = patch.dict(os.environ, {"HOME": str(self.root / "home"), "XDG_CONFIG_HOME": str(self.root / "xdg")})
        home.start()
        self.addCleanup(home.stop)
        for name in (REPO, FORK):
            subprocess.run(["git", "init", "-q", "--bare", str(self.root / f"{name}.git")], check=True, timeout=30)
        w = self.work = self.root / "work"
        subprocess.run(["git", "init", "-q", "-b", "main", str(w)], check=True, timeout=30)
        (w / ".github").mkdir()
        (w / ".github" / "PULL_REQUEST_TEMPLATE.md").write_text(TEMPLATE)
        (w / "a.txt").write_text("a\n")
        sh(w, "add", "-A")
        sh(w, "commit", "-qm", "base")
        sh(w, "checkout", "-qb", "release-1.19")
        (w / "r.txt").write_text("r\n")
        sh(w, "add", "-A")
        sh(w, "commit", "-qm", "release")
        sh(w, "checkout", "-q", "main")
        (w / "m.txt").write_text("m\n")
        sh(w, "add", "-A")
        sh(w, "commit", "-qm", "main")
        self.source = sh(w, "rev-parse", "HEAD")
        sh(w, "checkout", "-qb", "feature")
        (w / "f.txt").write_text("f\n")
        sh(w, "add", "-A")
        sh(w, "commit", "-qm", "feature")
        self.head = sh(w, "rev-parse", "HEAD")
        sh(w, "checkout", "-q", "--detach", "release-1.19")
        sh(w, "cherry-pick", self.head)
        self.rebased = sh(w, "rev-parse", "HEAD")
        self.target = sh(w, "rev-parse", "release-1.19")
        sh(w, "update-ref", "refs/retarget/prepared/run1/7/original", self.head)
        sh(w, "update-ref", "refs/retarget/prepared/run1/7/rebased", self.rebased)
        sh(w, "push", "-q", str(self.root / f"{REPO}.git"), "main", "release-1.19")
        sh(w, "push", "-q", str(self.root / f"{FORK}.git"), "feature")

        self.gh = FakeGH(self.root)
        self.gh.comment["body"] = f"/retarget release-1.19 {self.head}\nthanks"
        real_run = retarget.run_cmd

        def offline_run(cmd, timeout):
            self.assertEqual(cmd[0], "git", f"unexpected subprocess {cmd}")
            self.assertFalse(any(re.search(r"(://|^git@)", a) for a in cmd), f"network URL in {cmd}")
            return real_run(cmd, timeout)

        for target, value in (("gh", self.gh), ("run_cmd", offline_run), ("REMOTE_URL_TEMPLATE", str(self.root) + "/{repo}.git"),
                              ("POLL_DELAY", 0)):
            p = patch.object(retarget, target, value)
            p.start()
            self.addCleanup(p.stop)
        (self.root / "evidence").mkdir()
        (self.root / "evidence" / "range.diff").write_text("range\n")
        (self.root / "evidence" / "final.diff").write_text("final\n")
        self.body = self.root / "body.md"
        self.body.write_text(f"## Explanation\n\nRetarget of https://github.com/{REPO}/pull/7\n\n## Checklist\n\n- ok\n")
        self.journal = self.root / "journal.json"
        self.write_manifest()

    def candidate(self, **over):
        c = {"number": 7, "title": "fix legacy", "author": "alice", "draft": False, "base_ref": "main",
             "head_sha": self.head, "updated_at": "2026-10-01T00:00:00Z", "head_repository": FORK,
             "head_ref": "feature", "category": "LEGACY_ONLY", "probe": "OK", "merge_base": self.source,
             "rebased_sha": self.rebased, "backup_ref": "refs/retarget/prepared/run1/7/original",
             "prepared_ref": "refs/retarget/prepared/run1/7/rebased", "shared_files": [], "overlap_prs": [],
             "conflict_files": [], "recommendation": "RETARGET", "reasons": [],
             "evidence": {"range_diff": "evidence/range.diff", "final_diff": "evidence/final.diff"},
             "evidence_sha256": self.evidence_hashes(),
             "approval": {"reviewer": "maint", "head_sha": self.head, "target_sha": self.target,
                          "evidence_sha256": self.evidence_hashes(),
                          "rebased_sha": self.rebased, "range_diff_reviewed": True, "final_diff_reviewed": True,
                          "tests": [{"command": "go test ./pkg/engine/...", "result": "passed"}],
                          "release_feature_approved": False},
             "strategy": "sibling", "consent_comment_id": None}
        c.update(over)
        return c

    def evidence_hashes(self):
        return {k: hashlib.sha256((self.root / "evidence" / f).read_bytes()).hexdigest()
                for k, f in (("range_diff", "range.diff"), ("final_diff", "final.diff"))}

    def write_manifest(self, **over):
        self.manifest = self.root / "manifest.json"
        self.manifest.write_text(json.dumps({
            "schema_version": 1, "repository": REPO,
            "source": {"branch": "main", "sha": self.source}, "target": {"branch": "release-1.19", "sha": self.target},
            "generated_at": "2026-10-01T00:00:00Z", "candidates": [self.candidate(**over)]}))

    def author(self, **over):
        self.write_manifest(strategy="author", consent_comment_id=55, **over)

    def run_cli(self, *extra, execute=True):
        argv = ["--manifest", str(self.manifest), "--pr", "7", "--journal", str(self.journal),
                "--git-repo", str(self.work)] + list(extra)
        if execute:
            argv += ["--execute", "--updater-paused"]
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = retarget.main(argv)
        return rc, out.getvalue(), err.getvalue()

    def states(self):
        return [e["state"] for e in json.loads(self.journal.read_text())["events"]]

    def remote(self, repo, branch):
        return self.gh.head(repo, branch)

    def advance(self, repo, branch):
        """Simulate someone else pushing newer work."""
        sh(self.work, "checkout", "-q", "--detach", self.head)
        (self.work / "other.txt").write_text("x\n")
        sh(self.work, "add", "-A")
        sh(self.work, "commit", "-qm", "other")
        sh(self.work, "push", "-qf", str(self.root / f"{repo}.git"), f"HEAD:refs/heads/{branch}")
        return sh(self.work, "rev-parse", "HEAD")


class PreviewAndGateTest(Base):
    def assert_untouched(self, locked=False):
        self.assertEqual(self.gh.writes, [])
        self.assertFalse(self.journal.exists())
        self.assertEqual(Path(str(self.journal) + ".lock").exists(), locked)
        self.assertEqual(self.remote(FORK, "feature"), self.head)
        self.assertIsNone(self.remote(REPO, "retarget/legacy-7-" + self.head[:12]))

    def test_preview_makes_no_calls_and_no_journal(self):
        for strategy in ("sibling", "author"):
            self.write_manifest(strategy=strategy, consent_comment_id=55)
            for extra in ((), ("--resume",), ("--rollback",)):
                rc, out, _ = self.run_cli(*extra, execute=False)
                self.assertEqual(rc, 0)
                self.assertIn("DRY RUN", out)
        self.assertEqual(self.gh.calls, [])
        self.assert_untouched()

    def test_preview_shows_blocked_and_execute_refuses_without_approval(self):
        self.write_manifest(approval=None)
        rc, out, _ = self.run_cli(execute=False)
        self.assertEqual(rc, 0)
        self.assertIn("BLOCKED", out)
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 2)
        self.assertIn("approval pending", err)
        self.assertEqual(self.gh.calls, [])
        self.assert_untouched()

    def test_approval_must_bind_shas_and_evidence(self):
        cases = {
            "approval.head_sha": dict(approval=dict(self.candidate()["approval"], head_sha="0" * 40)),
            "approval.tests": dict(approval=dict(self.candidate()["approval"], tests=[])),
            "evidence.range_diff": dict(evidence={"range_diff": "../x", "final_diff": "evidence/final.diff"}),
            "recommendation": dict(recommendation="KEEP_MAIN"),
            "'NEEDS_AUTHOR', not 'RETARGET'": dict(recommendation="NEEDS_AUTHOR", draft=True),
            "approval.rebased_sha": dict(approval=dict(self.candidate()["approval"], rebased_sha=self.head)),
            "probe": dict(probe="CONFLICT"),
        }
        for needle, over in cases.items():
            with self.subTest(needle):
                self.write_manifest(**over)
                rc, _, err = self.run_cli("--body-file", str(self.body))
                self.assertEqual(rc, 2)
                self.assertIn(needle, err)
        self.assert_untouched()

    def test_feature_flag_required_only_for_feature_titles_and_draft_never_publishes(self):
        for title in ("feat(engine): add x", "feat: x", "feat/x", "Feature: x", "feat!: x", "feat add metrics",
                      "  FEAT - x", "feature add y"):
            with self.subTest(title):
                self.write_manifest(title=title)
                rc, _, err = self.run_cli("--body-file", str(self.body))
                self.assertEqual(rc, 2)
                self.assertIn("release_feature_approved", err)
        self.write_manifest(draft=True)
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 2)
        self.assertIn("draft", err)
        self.assert_untouched()
        self.write_manifest(title="feat(engine): add x",
                            approval=dict(self.candidate()["approval"], release_feature_approved=True))
        rc, out, _ = self.run_cli(execute=False)
        self.assertIn("STATUS: READY", out)
        for title in ("fix: featureless", "features: plural is not a feature prefix", "defeat x"):
            self.write_manifest(title=title)
            rc, out, _ = self.run_cli(execute=False)
            self.assertIn("STATUS: READY", out, title)
        self.write_manifest(title="fix: featureless")
        rc, out, _ = self.run_cli(execute=False)
        self.assertIn("STATUS: READY", out)

    def test_evidence_bytes_bound_to_candidate_and_approval(self):
        good = self.evidence_hashes()
        approval = self.candidate()["approval"]
        cases = {
            "bytes do not match evidence_sha256.final_diff": dict(),
            "evidence_sha256 must map exactly": dict(evidence_sha256={"range_diff": good["range_diff"]}),
            "approval.evidence_sha256 does not match": dict(approval=dict(approval, evidence_sha256=None)),
        }
        for needle, over in cases.items():
            with self.subTest(needle):
                (self.root / "evidence" / "final.diff").write_text("final\n")
                self.write_manifest(**over)
                if needle.startswith("bytes"):
                    (self.root / "evidence" / "final.diff").write_text("tampered\n")
                rc, _, err = self.run_cli("--body-file", str(self.body))
                self.assertEqual(rc, 2)
                self.assertIn(needle, err)
        (self.root / "evidence" / "final.diff").write_text("final\n")
        self.write_manifest(approval=dict(approval, evidence_sha256=dict(good, final_diff="0" * 64)))
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 2)
        self.assertIn("approval.evidence_sha256", err)
        self.assert_untouched()

    def test_only_main_to_release_routing_is_supported(self):
        good = json.loads(self.manifest.read_text())
        cases = {
            "reverse": dict(source={"branch": "release-1.19", "sha": self.target},
                            target={"branch": "main", "sha": self.source}),
            "other target": dict(target={"branch": "release-1.18", "sha": self.target}),
            "other source": dict(source={"branch": "develop", "sha": self.source}),
            "missing target": dict(target={"sha": self.target}),
        }
        for name, over in cases.items():
            with self.subTest(name):
                self.manifest.write_text(json.dumps(dict(good, **over)))
                for execute in (False, True):
                    rc, _, err = self.run_cli("--body-file", str(self.body), execute=execute)
                    self.assertEqual(rc, 1)
                    self.assertIn("invalid manifest", err)
        self.assertEqual(self.gh.calls, [])
        self.assert_untouched()

    def git_config(self, path, *args):
        subprocess.run(["git", "config", "--file", str(path)] + list(args), check=True, timeout=30)

    def rewrite_to_evil(self, scope):
        evil = self.root / "evil.git"
        subprocess.run(["git", "init", "-q", "--bare", str(evil)], check=True, timeout=30)
        real = str(self.root / f"{REPO}.git")
        if scope == "global":
            self.git_config(self.root / "home" / ".gitconfig", f"url.{evil}.insteadOf", real)
        elif scope == "xdg":
            (self.root / "xdg" / "git").mkdir(parents=True)
            self.git_config(self.root / "xdg" / "git" / "config", f"url.{evil}.pushInsteadOf", real)
        elif scope == "local":
            self.git_config(self.work / ".git" / "config", f"url.{evil}.pushInsteadOf", real)
        elif scope == "include":
            inc = self.root / "home" / "included.cfg"
            self.git_config(inc, f"url.{evil}.insteadOf", real)
            self.git_config(self.root / "home" / ".gitconfig", "include.path", str(inc))
        return evil

    def test_url_rewrite_config_refused_before_any_write(self):
        for scope in ("global", "xdg", "local", "include"):
            with self.subTest(scope):
                self.setUp()
                evil = self.rewrite_to_evil(scope)
                rc, _, err = self.run_cli("--body-file", str(self.body))
                self.assertEqual(rc, 1)
                self.assertIn("rewrites remote URLs", err)
                self.assertEqual(self.gh.calls, [])
                self.assertFalse(self.journal.exists())
                self.assertEqual(sh(self.root, "--git-dir", str(evil), "for-each-ref"), "")
                self.assertIsNone(self.remote(REPO, "retarget/legacy-7-" + self.head[:12]))

    def test_url_rewrite_refused_for_rollback(self):
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
        writes = len(self.gh.writes)
        self.rewrite_to_evil("global")
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("rewrites remote URLs", err)
        self.assertEqual(len(self.gh.writes), writes)
        self.assertEqual(self.states()[-1], "complete")

    def test_inherited_git_env_is_stripped(self):
        evil = self.root / "evil.git"
        subprocess.run(["git", "init", "-q", "--bare", str(evil)], check=True, timeout=30)
        injected = {"GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": f"url.{evil}.insteadOf",
                    "GIT_CONFIG_VALUE_0": str(self.root / f"{REPO}.git"),
                    "GIT_CONFIG_PARAMETERS": f"'url.{evil}.pushinsteadof'='{self.root / REPO}.git'",
                    "GIT_DIR": str(evil), "GIT_NAMESPACE": "evil"}
        with patch.dict(os.environ, injected):
            rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.remote(REPO, "retarget/legacy-7-" + self.head[:12]), self.rebased)
        self.assertEqual(sh(self.root, "--git-dir", str(evil), "for-each-ref"), "")

    def test_only_legacy_only_category_is_publishable(self):
        for category in (None, "MIXED", "CEL_ONLY", "SHARED_ONLY", "REVIEW-MIGRATION", "legacy_only"):
            with self.subTest(category):
                self.write_manifest(category=category)
                rc, out, _ = self.run_cli(execute=False)
                self.assertIn("BLOCKED", out)
                rc, _, err = self.run_cli("--body-file", str(self.body))
                self.assertEqual(rc, 2)
                self.assertIn("category", err)
        self.assert_untouched()

    def test_live_title_drift_refused_initially(self):
        self.gh.original["title"] = "feat: now a feature"
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("title", err)
        self.assert_untouched(locked=True)

    def test_refs_outside_owned_namespace_are_refused(self):
        for key, ref in (("backup_ref", "refs/heads/feature"), ("prepared_ref", "refs/retarget/prepared/run1/8/rebased"),
                         ("prepared_ref", "refs/retarget/prepared/run1/7/original")):
            with self.subTest(ref):
                self.write_manifest(**{key: ref})
                for execute in (False, True):
                    rc, _, err = self.run_cli("--body-file", str(self.body), execute=execute)
                    self.assertEqual(rc, 1)
                    self.assertIn(f"invalid manifest: {key}", err)
        self.assert_untouched()

    def test_rebased_commit_must_descend_from_target(self):
        sh(self.work, "update-ref", "refs/retarget/prepared/run1/7/rebased", self.head)
        self.write_manifest(rebased_sha=self.head,
                            approval=dict(self.candidate()["approval"], rebased_sha=self.head))
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("descendant of target.sha", err)
        self.assert_untouched(locked=True)

    def test_prepared_ref_must_point_at_approved_sha(self):
        sh(self.work, "update-ref", "refs/retarget/prepared/run1/7/rebased", self.target)
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("prepared_ref", err)
        self.assert_untouched(locked=True)

    def test_live_draft_change_is_stale(self):
        self.gh.original["draft"] = True
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("draft", err)
        self.assert_untouched(locked=True)

    def test_execute_requires_updater_paused_attestation(self):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            rc = retarget.main(["--manifest", str(self.manifest), "--pr", "7", "--journal", str(self.journal),
                                "--git-repo", str(self.work), "--execute", "--body-file", str(self.body)])
        self.assertEqual(rc, 1)
        self.assertIn("--updater-paused", err.getvalue())
        self.assert_untouched()

    def test_stale_live_state_refuses_before_any_write(self):
        def stale_head():
            self.advance(FORK, "feature")

        def stale_target():
            sh(self.work, "push", "-qf", str(self.root / f"{REPO}.git"), f"{self.rebased}:refs/heads/release-1.19")

        def stale_source():
            sh(self.work, "push", "-qf", str(self.root / f"{REPO}.git"), f"{self.head}:refs/heads/main")

        def stale_updated():
            self.gh.original["updated_at"] = "2026-10-02T00:00:00Z"

        def bad_backup_ref():
            sh(self.work, "update-ref", "refs/retarget/prepared/run1/7/original", self.source)

        for name, mutate in (("head", stale_head), ("target", stale_target), ("source", stale_source),
                             ("updated", stale_updated), ("backup_ref", bad_backup_ref)):
            with self.subTest(name):
                self.setUp()
                mutate()
                rc, _, err = self.run_cli("--body-file", str(self.body))
                self.assertEqual(rc, 1, err)
                self.assertEqual(self.gh.writes, [])
                self.assertFalse(self.journal.exists())
                self.assertIsNone(self.remote(REPO, "retarget/legacy-7-" + self.head[:12]))

    def test_journal_lock_is_exclusive(self):
        fd = os.open(str(self.journal) + ".lock", os.O_RDWR | os.O_CREAT, 0o600)
        self.addCleanup(os.close, fd)
        fcntl.flock(fd, fcntl.LOCK_EX)
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("locked", err)
        self.assertEqual(self.gh.writes, [])


class AuthorTest(Base):
    def test_resume_refuses_new_draft_after_partial_publication(self):
        self.author()
        self.gh.fail_before.add("pr edit")
        rc, _, _ = self.run_cli()
        self.assertEqual(rc, 1)
        self.gh.original["draft"] = True
        writes = list(self.gh.writes)
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertIn("draft", err)
        self.assertEqual(self.gh.writes, writes)

    def test_happy_path(self):
        self.author()
        rc, _, err = self.run_cli()
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.remote(FORK, "feature"), self.rebased)
        self.assertEqual(self.gh.base, "release-1.19")
        self.assertEqual(self.states(), ["planned", "push_pending", "pushed", "base_pending", "complete"])
        self.assertEqual(self.gh.writes, [["pr", "edit", "7", "--repo", REPO, "--base", "release-1.19"]])
        rc, _, err = self.run_cli()
        self.assertEqual(rc, 1)
        self.assertIn("use --resume", err)
        rc, _, _ = self.run_cli("--resume")
        self.assertEqual(rc, 0)
        self.assertEqual(len(self.gh.writes), 1)

    def test_consent_and_permission_refusals(self):
        cases = {
            "first line": lambda: self.gh.comment.update(body=f"/retarget release-1.19 {self.source}"),
            "not written by the PR author": lambda: self.gh.comment.update(user={"login": "mallory"}),
            "does not belong": lambda: self.gh.comment.update(issue_url=f"https://api.github.com/repos/{REPO}/issues/8"),
            "maintainerCanModify": lambda: self.gh.original.update(maintainer_can_modify=False),
            "maintainerCanModify ": lambda: self.gh.original.update(maintainer_can_modify=None),
        }
        for needle, mutate in cases.items():
            with self.subTest(needle):
                self.setUp()
                self.author()
                mutate()
                rc, _, err = self.run_cli()
                self.assertEqual(rc, 1)
                self.assertIn(needle.strip(), err)
                self.assertEqual(self.remote(FORK, "feature"), self.head)
                self.assertEqual(self.gh.writes, [])
        self.write_manifest(strategy="author", consent_comment_id=None)
        rc, _, err = self.run_cli()
        self.assertEqual(rc, 2)
        self.assertIn("consent_comment_id", err)

    def test_lease_protects_against_concurrent_push(self):
        self.author()
        real_git = retarget.git
        newer = []

        def racing_git(repo_path, args, **kw):
            if args[0] == "push" and not newer:
                newer.append(self.advance(FORK, "feature"))
            return real_git(repo_path, args, **kw)

        with patch.object(retarget, "git", racing_git):
            rc, _, err = self.run_cli()
        self.assertEqual(rc, 1)
        self.assertEqual(self.remote(FORK, "feature"), newer[0])
        self.assertEqual(self.states()[-1], "push_pending")
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertEqual(self.remote(FORK, "feature"), newer[0])
        self.assertEqual(self.gh.writes, [])

    def test_push_timeout_then_resume(self):
        self.author()
        real_git = retarget.git

        def timeout_after_push(repo_path, args, **kw):
            res = real_git(repo_path, args, **kw)
            if args[0] == "push":
                raise retarget.RetargetError("timed out, outcome unknown")
            return res

        with patch.object(retarget, "git", timeout_after_push):
            rc, _, _ = self.run_cli()
        self.assertEqual(rc, 1)
        self.assertEqual(self.states(), ["planned", "push_pending"])
        self.assertEqual(self.remote(FORK, "feature"), self.rebased)
        self.gh.original["updated_at"] = "2026-10-05T00:00:00Z"  # our push bumps updatedAt
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.states()[-3:], ["pushed", "base_pending", "complete"])

    def test_base_edit_timeout_then_resume_does_not_repeat(self):
        self.author()
        self.gh.fail_after.add("pr edit")
        rc, _, _ = self.run_cli()
        self.assertEqual(rc, 1)
        self.assertEqual(self.states()[-1], "base_pending")
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 0, err)
        self.assertEqual(len(self.gh.writes), 1)
        self.assertEqual(self.states()[-1], "complete")

    def test_resume_refuses_changed_manifest_identity(self):
        self.author()
        self.gh.fail_after.add("pr edit")
        self.run_cli()
        self.author(title="something else")
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertIn("identity", err)

    def test_complete_resume_never_reapplies_base(self):
        self.author()
        self.assertEqual(self.run_cli()[0], 0)
        self.gh.base = "main"
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertIn("refusing to re-apply", err)
        self.assertEqual(self.gh.base, "main")
        self.assertEqual(len(self.gh.writes), 1)
        self.assertEqual(self.states()[-1], "complete")

    def test_unowned_rebased_head_is_not_adopted(self):
        self.author()
        sh(self.work, "push", "-qf", str(self.root / f"{FORK}.git"), f"{self.rebased}:refs/heads/feature")
        self.gh.api_head = self.head
        rc, _, err = self.run_cli()
        self.assertEqual(rc, 1)
        self.assertIn("never recorded push intent", err)
        self.assertFalse(self.journal.exists())
        self.assertEqual(self.gh.writes, [])

    def test_rollback_refuses_foreign_base_change(self):
        self.author()
        with patch.object(retarget, "poll_pr", side_effect=retarget.RetargetError("simulated readback failure")):
            self.assertEqual(self.run_cli()[0], 1)
        self.assertEqual(self.states()[-1], "pushed")
        self.gh.base = "release-1.19"
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("cannot prove it made that change", err)
        self.assertEqual(self.remote(FORK, "feature"), self.rebased)
        self.assertEqual(self.gh.base, "release-1.19")
        self.assertEqual(self.gh.writes, [])

    def test_rollback_refuses_ambiguous_base_pending_with_original_head(self):
        self.author()
        self.gh.fail_before.add("pr edit")
        self.assertEqual(self.run_cli()[0], 1)
        self.assertEqual(self.states()[-1], "base_pending")
        self.assertEqual(self.gh.base, "main")
        # someone else changes the base and the author resets the head to the original commit
        self.gh.base = "release-1.19"
        sh(self.work, "push", "-qf", str(self.root / f"{FORK}.git"), f"{self.head}:refs/heads/feature")
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("cannot prove", err)
        self.assertEqual(self.gh.base, "release-1.19")
        self.assertEqual(self.gh.writes, [])
        self.assertEqual(self.states()[-1], "base_pending")

    def test_rollback_with_base_pending_and_prepared_head_is_owned(self):
        self.author()
        self.gh.fail_after.add("pr edit")
        self.assertEqual(self.run_cli()[0], 1)
        self.assertEqual(self.states()[-1], "base_pending")
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.remote(FORK, "feature"), self.head)
        self.assertEqual(self.gh.base, "main")

    def test_rollback_resume_after_head_restored_restores_base(self):
        self.author()
        self.assertEqual(self.run_cli()[0], 0)
        self.gh.fail_before.add("pr edit")
        rc, _, _ = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertEqual(self.remote(FORK, "feature"), self.head)
        self.assertEqual(self.gh.base, "release-1.19")
        self.assertEqual(self.states()[-1], "rollback_base_pending")
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.gh.base, "main")
        self.assertEqual(self.states()[-1], "rolled_back")

    def test_rollback_refuses_base_changed_by_others_mid_rollback(self):
        self.author()
        self.gh.fail_before.add("pr edit")
        self.assertEqual(self.run_cli()[0], 1)
        with patch.object(retarget, "poll_pr", side_effect=retarget.RetargetError("simulated readback failure")):
            self.assertEqual(self.run_cli("--rollback")[0], 1)
        self.assertFalse(json.loads(self.journal.read_text())["rollback_restore_base"])
        self.gh.base = "release-1.19"
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("by someone else during rollback", err)
        self.assertEqual(self.gh.base, "release-1.19")

    def test_live_title_drift_refused_on_resume(self):
        self.author()
        self.gh.fail_before.add("pr edit")
        self.assertEqual(self.run_cli()[0], 1)
        self.assertEqual(self.states()[-1], "base_pending")
        self.gh.original["title"] = "feat: now a feature"
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertIn("title", err)
        self.assertEqual(self.gh.base, "main")
        self.assertEqual(self.gh.writes, [])

    def test_rollback_refuses_complete_with_reset_head_and_target_base(self):
        self.author()
        self.assertEqual(self.run_cli()[0], 0)
        writes = len(self.gh.writes)
        sh(self.work, "push", "-qf", str(self.root / f"{FORK}.git"), f"{self.head}:refs/heads/feature")
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("cannot prove", err)
        self.assertEqual(self.gh.base, "release-1.19")
        self.assertEqual(len(self.gh.writes), writes)
        self.assertEqual(self.states()[-1], "complete")

    def test_rollback_restores_head_then_base(self):
        self.author()
        self.assertEqual(self.run_cli()[0], 0)
        writes = len(self.gh.writes)
        rc, out, _ = self.run_cli("--rollback", execute=False)
        self.assertEqual(rc, 0)
        self.assertIn(f"--force-with-lease=refs/heads/feature:{self.rebased}", out)
        self.assertEqual(len(self.gh.writes), writes)
        self.assertEqual(self.remote(FORK, "feature"), self.rebased)
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.remote(FORK, "feature"), self.head)
        self.assertEqual(self.gh.base, "main")
        self.assertEqual(self.states()[-5:], ["rollback_planned", "rollback_push_pending", "rollback_pushed",
                                              "rollback_base_pending", "rolled_back"])
        rc, _, _ = self.run_cli("--resume")
        self.assertEqual(rc, 1)

    def test_rollback_rejects_changed_head(self):
        self.author()
        self.assertEqual(self.run_cli()[0], 0)
        sh(self.work, "checkout", "-q", "--detach", self.rebased)
        (self.work / "more.txt").write_text("more\n")
        sh(self.work, "add", "-A")
        sh(self.work, "commit", "-qm", "author follow-up")
        newer = sh(self.work, "rev-parse", "HEAD")
        sh(self.work, "push", "-q", str(self.root / f"{FORK}.git"), "HEAD:refs/heads/feature")
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertEqual(self.remote(FORK, "feature"), newer)
        self.assertEqual(self.gh.base, "release-1.19")


class SiblingTest(Base):
    branch = property(lambda self: "retarget/legacy-7-" + self.head[:12])

    def test_resume_refuses_new_draft_after_partial_publication(self):
        self.gh.fail_before.add("pr create")
        rc, _, _ = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.gh.original["draft"] = True
        writes = list(self.gh.writes)
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("draft", err)
        self.assertEqual(self.gh.writes, writes)

    def test_happy_path_and_no_duplicate(self):
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.remote(REPO, self.branch), self.rebased)
        self.assertEqual(self.remote(FORK, "feature"), self.head)
        self.assertEqual(self.gh.base, "main")
        self.assertEqual(list(self.gh.siblings), [100])
        self.assertEqual(self.states(), ["planned", "branch_pending", "branch_pushed", "pr_pending", "pr_created", "complete"])
        rc, _, _ = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 0)
        self.assertEqual(list(self.gh.siblings), [100])

    def test_body_file_requirements(self):
        cases = {
            "requires --body-file": None,
            "must link": "## Explanation\n\n## Checklist\nsee #7\n",
            "every heading": f"## Explanation\nhttps://github.com/{REPO}/pull/7\n",
            "must link the original": f"## Explanation\n## Checklist\nhttps://github.com/{REPO}/pull/70\n",
        }
        for needle, text in cases.items():
            with self.subTest(needle):
                extra = ()
                if text is not None:
                    self.body.write_text(text)
                    extra = ("--body-file", str(self.body))
                rc, _, err = self.run_cli(*extra)
                self.assertEqual(rc, 1)
                self.assertIn(needle, err)
                self.assertFalse(self.journal.exists())
                self.assertIsNone(self.remote(REPO, self.branch))

    def test_create_timeout_then_resume_does_not_duplicate(self):
        self.gh.fail_after.add("pr create")
        rc, _, _ = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertEqual(self.states()[-1], "pr_pending")
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 0, err)
        self.assertEqual(list(self.gh.siblings), [100])
        self.assertEqual(json.loads(self.journal.read_text())["sibling_pr"], 100)

    def test_existing_branch_not_ours_is_refused(self):
        sh(self.work, "push", "-q", str(self.root / f"{REPO}.git"), f"{self.rebased}:refs/heads/{self.branch}")
        rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("never recorded", err)
        self.assertEqual(self.gh.writes, [])

    def test_resume_refuses_branch_with_other_sha(self):
        self.gh.fail_after.add("pr create")
        self.run_cli("--body-file", str(self.body))
        self.gh.siblings.clear()
        self.advance(REPO, self.branch)
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("not prepared", err)
        self.assertEqual(self.gh.siblings, {})

    def stop_at_branch_pushed(self):
        self.gh.fail_before.add("pr list")
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 1)
        self.assertEqual(self.states()[-1], "branch_pushed")
        self.gh.siblings[100] = {"branch": self.branch, "base": "release-1.19", "state": "open", "merged": False}

    def test_reviewed_title_and_body_bound_before_first_push(self):
        self.assertEqual(json.loads(self.journal.read_text()) if self.journal.exists() else None, None)
        self.gh.fail_before.add("pr list")
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 1)
        j = json.loads(self.journal.read_text())
        self.assertEqual(j["state"], "branch_pushed")
        self.assertEqual(j["events"][0]["state"], "planned")
        self.assertEqual(j["sibling_title"], "[release-1.19] fix legacy")
        self.assertIsNotNone(j["body_sha256"])
        original = self.body.read_text()
        for name, extra, text in (("title", ("--title", "other"), original),
                                  ("body", (), original + "swapped\n")):
            with self.subTest(name):
                self.body.write_text(text)
                rc, _, err = self.run_cli("--resume", "--body-file", str(self.body), *extra)
                self.assertEqual(rc, 1)
                self.assertIn("bound in the journal before the first push", err)
                self.assertEqual(self.gh.siblings, {})
                self.assertEqual(self.states()[-1], "branch_pushed")
        self.body.write_text(original)
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 0, err)
        self.assertEqual(list(self.gh.siblings), [100])

    def test_unowned_sibling_pr_is_not_adopted(self):
        self.stop_at_branch_pushed()
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("reconcile manually", err)
        self.assertEqual(self.states()[-1], "branch_pushed")
        self.assertEqual(self.gh.writes, [])

    def test_rollback_without_pr_intent_leaves_everything(self):
        self.stop_at_branch_pushed()
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.gh.siblings[100]["state"], "open")
        self.assertEqual(self.remote(REPO, self.branch), self.rebased)
        self.assertEqual(self.states()[-1], "rolled_back")
        self.assertEqual(self.gh.writes, [])

    def test_pr_pending_resume_requires_same_body_and_title(self):
        self.gh.fail_before.add("pr create")
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 1)
        self.assertEqual(self.states()[-1], "pr_pending")
        self.body.write_text(self.body.read_text() + "edited\n")
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("bound in the journal before the first push", err)
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body), "--title", "other")
        self.assertEqual(rc, 1)
        self.assertEqual(self.gh.siblings, {})

    def test_complete_resume_enforces_recorded_sibling_identity(self):
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
        self.gh.siblings[101] = {"branch": self.branch, "base": "release-1.19", "state": "open", "merged": False}
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertIn("not provably created by this journal", err)

    def test_unknown_create_outcome_adopts_only_marked_pr(self):
        self.gh.fail_after.add("pr create")
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 1)
        j = json.loads(self.journal.read_text())
        self.assertEqual(j["state"], "pr_pending")
        self.assertNotIn("sibling_pr", j)
        self.assertIn(f"kyverno-retarget-run: {j['run_id']}", self.gh.siblings[100]["body"])
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 0, err)
        self.assertEqual(json.loads(self.journal.read_text())["sibling_pr"], 100)
        self.assertEqual(list(self.gh.siblings), [100])

    def test_unknown_create_outcome_refuses_foreign_matching_pr(self):
        for name, sib in (
            ("same body other actor", lambda b: {"author": "mallory", "body": b}),
            ("same actor no marker", lambda b: {"author": "maint", "body": self.body.read_text()}),
            ("same actor other run", lambda b: {"author": "maint", "body": b.replace("run: ", "run: x")}),
        ):
            with self.subTest(name):
                self.setUp()
                self.gh.fail_before.add("pr create")
                self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 1)
                run_id = json.loads(self.journal.read_text())["run_id"]
                marked = self.body.read_text().rstrip("\n") + f"\n\n<!-- kyverno-retarget-run: {run_id} -->\n"
                self.gh.siblings[100] = dict({"branch": self.branch, "base": "release-1.19", "state": "open",
                                              "merged": False}, **sib(marked))
                rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
                self.assertEqual(rc, 1)
                self.assertIn("reconcile manually", err)
                rc, _, err = self.run_cli("--rollback")
                self.assertEqual(rc, 1)
                self.assertIn("reconcile manually", err)
                self.assertEqual(self.gh.siblings[100]["state"], "open")
                self.assertEqual([w for w in self.gh.writes if w[:2] != ["pr", "create"]], [])

    def test_original_rechecked_immediately_before_create(self):
        real_list = retarget.find_siblings

        def list_then_original_moves(ctx):
            rows = real_list(ctx)
            self.gh.original["state"] = "closed"
            return rows

        with patch.object(retarget, "find_siblings", list_then_original_moves):
            rc, _, err = self.run_cli("--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertIn("state=closed", err)
        self.assertEqual(self.gh.siblings, {})
        self.assertEqual(self.states()[-1], "branch_pushed")
        self.gh.original["state"] = "open"
        self.advance(FORK, "feature")
        rc, _, err = self.run_cli("--resume", "--body-file", str(self.body))
        self.assertEqual(rc, 1)
        self.assertEqual(self.gh.siblings, {})

    def test_owned_sibling_retargeted_elsewhere_is_detected(self):
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
        self.gh.siblings[100]["base"] = "main"
        rc, _, err = self.run_cli("--resume")
        self.assertEqual(rc, 1)
        self.assertIn("no longer target release-1.19", err)
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("head/base changed", err)
        self.assertEqual(self.gh.siblings[100]["state"], "open")
        self.assertNotIn("rolled_back", self.states())
        self.assertNotIn(["pr", "close", "100", "--repo", REPO], self.gh.writes)
        lists = [c for c in self.gh.calls if c[:2] == ["pr", "list"]]
        self.assertTrue(lists)
        self.assertTrue(all("--base" not in c for c in lists))

    def test_rollback_refuses_when_recorded_sibling_missing(self):
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
        self.gh.siblings.clear()
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("not found", err)
        self.assertNotIn("rolled_back", self.states())

    def test_rollback_checks_live_branch_before_close(self):
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
        self.advance(REPO, self.branch)
        self.gh.siblings[100]["api_head"] = self.rebased
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 1)
        self.assertIn("refusing to close", err)
        self.assertEqual(self.gh.siblings[100]["state"], "open")

    def test_rollback_closes_only_sibling_and_keeps_branch(self):
        self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
        rc, _, err = self.run_cli("--rollback")
        self.assertEqual(rc, 0, err)
        self.assertEqual(self.gh.siblings[100]["state"], "closed")
        self.assertEqual(self.gh.original["state"], "open")
        self.assertEqual(self.remote(REPO, self.branch), self.rebased)
        self.assertEqual(self.gh.writes[-1], ["pr", "close", "100", "--repo", REPO])

    def test_rollback_refuses_merged_or_changed(self):
        for name in ("merged", "original-moved"):
            with self.subTest(name):
                self.setUp()
                self.assertEqual(self.run_cli("--body-file", str(self.body))[0], 0)
                if name == "merged":
                    self.gh.siblings[100]["merged"] = True
                else:
                    self.advance(FORK, "feature")
                rc, _, _ = self.run_cli("--rollback")
                self.assertEqual(rc, 1)
                self.assertNotIn("pr close", [" ".join(w[:2]) for w in self.gh.writes])


if __name__ == "__main__":
    unittest.main()
