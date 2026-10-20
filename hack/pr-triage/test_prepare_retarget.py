import importlib.util
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SPEC = importlib.util.spec_from_file_location(
    "prepare_retarget", Path(__file__).with_name("prepare-retarget.py")
)
prepare = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(prepare)


class ManifestTest(unittest.TestCase):
    def test_preparation_metadata_is_pinned_to_github_host(self):
        result = subprocess.CompletedProcess(["gh"], 0, stdout='{"ok": true}')
        with patch.dict(os.environ, {"GH_HOST": "other.invalid", "GH_CONFIG_DIR": "/untrusted",
                                     "GH_REPO": "other/repo", "GH_TOKEN": "test-only"}), \
                patch.object(prepare, "run", return_value=result) as command:
            self.assertEqual(prepare.gh("api", "user"), {"ok": True})
        env = command.call_args.kwargs["env"]
        self.assertEqual(env["GH_HOST"], "github.com")
        self.assertEqual(env["GH_TOKEN"], "test-only")
        self.assertNotIn("GH_CONFIG_DIR", env)
        self.assertNotIn("GH_REPO", env)

    def row(self, **changes):
        return {
            "probe": "OK", "draft": False, "title": "fix: legacy bug",
            "shared_files": [], "overlap_prs": [], "category": "LEGACY_ONLY",
            **changes,
        }

    def test_recommendation_is_not_approval(self):
        self.assertEqual(prepare.recommend(self.row())[0], "RETARGET")

    def test_pinned_routing_is_recomputed_before_probe(self):
        files = ["pkg/engine/probe.txt"]
        snapshot = {"category": "LEGACY_ONLY", "content_signal": "NEUTRAL", "migration_flag": False}
        prepare.verify_routing(snapshot, {}, files, "+neutral\n")
        for stale, live_files, diff in (
            ({**snapshot, "content_signal": "LEGACY_CONTENT"}, files, "+neutral\n"),
            (snapshot, ["pkg/cel/probe.go"], "+neutral\n"),
            (snapshot, files + ["pkg/cel/probe.go"], "+neutral\n"),
            ({**snapshot, "migration_flag": True}, files, "+neutral\n"),
            (snapshot, files, "+ValidatingPolicy\n"),
        ):
            with self.subTest(stale=stale, files=live_files, diff=diff):
                with self.assertRaisesRegex(ValueError, "refresh report"):
                    prepare.verify_routing(stale, {}, live_files, diff)
        self.assertEqual(
            prepare.recommend(self.row(migration_flag=True))[0], "NEEDS_AUTHOR"
        )

    def test_unsafe_or_unreviewed_candidates_are_held(self):
        for changes in (
            {"draft": True}, {"title": "Feat/background metrics"},
            {"title": " feat add metrics"},
            {"shared_files": ["pkg/config/config.go"]},
            {"overlap_prs": [{"number": 123}]}, {"probe": "CONFLICT"},
            {"overlap_candidates": [{"number": 456}]},
            {"probe": "ERROR:stale"}, {"category": "MIXED"},
            {"labels": ["type_shared"]},
        ):
            with self.subTest(changes=changes):
                self.assertEqual(prepare.recommend(self.row(**changes))[0], "NEEDS_AUTHOR")

    def test_explicit_scope_overrides_are_preserved(self):
        for override in ("KEEP_MAIN", "SPLIT"):
            with self.subTest(override=override):
                self.assertEqual(prepare.recommend(self.row(override=override))[0], override)

    def test_already_landed_requires_manual_disposition_not_retarget(self):
        self.assertEqual(prepare.recommend(self.row(probe="EMPTY"))[0], "KEEP_MAIN")

    def test_overlap_is_based_on_paths_not_title(self):
        prs = [
            {"number": 1, "title": "different fix", "files": ["pkg/engine/a.go"]},
            {"number": 2, "title": "same title", "files": ["pkg/cel/b.go"]},
        ]
        self.assertEqual(
            prepare.overlap(["pkg/engine/a.go"], prs),
            [{"number": 1, "title": "different fix", "files": ["pkg/engine/a.go"]}],
        )

    def test_identity_checks_metadata_and_state(self):
        row = {"head_sha": "abc", "base_ref": "main", "updated_at": "now"}
        pr = {
            "state": "open", "head": {"sha": "abc"}, "base": {"ref": "main"},
            "updated_at": "now",
        }
        self.assertTrue(prepare.same_identity(row, pr))
        self.assertFalse(prepare.same_identity(row, {**pr, "updated_at": "later"}))
        self.assertFalse(prepare.same_identity(row, {**pr, "state": "closed"}))

    def test_dated_snapshot_is_unapproved_and_conservative(self):
        snapshot = Path(__file__).with_name("snapshots") / "2026-10-20.json"
        manifest = json.loads(snapshot.read_text())
        rows = manifest["candidates"]
        self.assertEqual(len(rows), len({r["number"] for r in rows}))
        for row in rows:
            with self.subTest(number=row["number"]):
                self.assertIsNone(row["approval"])
                self.assertEqual(row["strategy"], "sibling")
                self.assertEqual(row["recommendation"], prepare.recommend(row)[0])
                self.assertTrue(prepare.SHA.fullmatch(row["head_sha"]))
                self.assertEqual(set(row["evidence_sha256"]), set(row["evidence"]))
                for digest in row["evidence_sha256"].values():
                    self.assertRegex(digest, r"^[0-9a-f]{64}$")
                if row["probe"] == "CONFLICT":
                    self.assertTrue(row["conflict_files"])
                if row["probe"] == "OK":
                    self.assertTrue(prepare.SHA.fullmatch(row["rebased_sha"]))
                    self.assertIn("range_diff", row["evidence"])
                    self.assertIn("final_diff", row["evidence"])


class ProbeTest(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix="retarget-probe-test-")
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.remote = self.root / "upstream.git"
        self.clone = self.root / "clone"
        self.output = self.root / "evidence"
        self.output.mkdir()
        self.environment = {
            **{k: v for k, v in os.environ.items() if not k.startswith("GIT_")},
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "Test", "GIT_AUTHOR_EMAIL": "test@example.invalid",
            "GIT_COMMITTER_NAME": "Test", "GIT_COMMITTER_EMAIL": "test@example.invalid",
        }
        self.git("init", "--bare", "-q", str(self.remote), cwd=self.root)
        self.git("clone", "-q", str(self.remote), str(self.clone), cwd=self.root)
        self.path = self.clone / "pkg/engine/probe.txt"
        self.path.parent.mkdir(parents=True)
        self.path.write_text("base\n")
        (self.clone / ".gitattributes").write_text("pkg/engine/probe.txt filter=untrusted merge=untrusted\n")
        self.git("add", ".")
        self.git("commit", "-qm", "base")
        self.base = self.git("rev-parse", "HEAD")
        self.git("update-ref", "refs/test/source", self.base)
        self.path.write_text("contributor\n")
        self.git("commit", "-qam", "legacy fix")
        self.head = self.git("rev-parse", "HEAD")
        self.git("push", "-q", str(self.remote), "HEAD:refs/pull/42/head")

    def git(self, *args, cwd=None):
        return subprocess.run(
            ["git", "-c", "commit.gpgSign=false", *args], check=True,
            capture_output=True, text=True, cwd=cwd or self.clone,
            env=self.environment,
        ).stdout.strip()

    def probe(self, outcome):
        if outcome == "EMPTY":
            target = self.head
        else:
            self.git("checkout", "-q", "--detach", self.base)
            if outcome == "CONFLICT":
                self.path.write_text("release\n")
            else:
                (self.clone / "release.txt").write_text("unrelated release change\n")
            self.git("add", ".")
            self.git("commit", "-qm", "release change")
            target = self.git("rev-parse", "HEAD")
        self.git("update-ref", "refs/test/target", target)
        marker = self.root / "hook-executed"
        hook_body = "#!/bin/sh\nprintf executed >> " + shlex.quote(str(marker)) + "\n"
        for name in ("post-checkout", "post-rewrite"):
            hook = self.clone / ".git/hooks" / name
            hook.write_text(hook_body)
            hook.chmod(0o755)
        for config in ("filter.untrusted.smudge", "filter.untrusted.clean",
                       "filter.untrusted.process", "merge.untrusted.driver"):
            self.git("config", config, "touch " + shlex.quote(str(marker)))
        attributes = self.clone / ".git/info/attributes"
        attributes.write_text("* filter=untrusted merge=untrusted\n")
        snapshot = {
            "number": 42, "title": "fix: legacy probe", "head_sha": self.head,
            "base_ref": "main", "updated_at": "captured", "draft": False,
            "files": ["pkg/engine/probe.txt"], "category": "LEGACY_ONLY",
            "content_signal": "NEUTRAL", "migration_flag": False,
        }
        pr = {
            "state": "open", "base": {"ref": "main"}, "updated_at": "captured",
            "head": {"sha": self.head, "repo": {"full_name": "author/kyverno"}, "ref": "fix"},
            "draft": False, "user": {"login": "author"}, "maintainer_can_modify": True,
            "changed_files": 1,
        }
        commands = []
        original_run = prepare.run

        def offline_run(*args, **kwargs):
            self.assertEqual(args[0], "git", "probe must never execute contributor code")
            commands.append(args)
            args = tuple(
                str(self.remote) if value == "https://github.com/kyverno/kyverno.git"
                else value for value in args
            )
            return original_run(
                *args, **{**kwargs, "cwd": self.clone}
            )

        with patch.dict(os.environ, self.environment, clear=True), \
                patch.object(prepare, "gh", return_value=pr), \
                patch.object(prepare, "run", side_effect=offline_run):
            row = prepare.prepare_one(
                snapshot, "kyverno/kyverno", "refs/test/source", "refs/test/target",
                "refs/retarget/prepared/test", self.output, [],
            )
        self.assertEqual(row["probe"], outcome)
        self.assertFalse(marker.exists(), "untrusted worktree hooks executed")
        self.assertEqual(self.git("worktree", "list", "--porcelain").count("worktree "), 1)
        removed = [command[-1] for command in commands if "remove" in command and "worktree" in command]
        self.assertEqual(len(removed), 1)
        self.assertFalse(Path(removed[0]).exists())
        self.assertTrue((self.output / "42/candidate.json").is_file())
        for key, relative in row["evidence"].items():
            self.assertEqual(
                row["evidence_sha256"][key],
                hashlib.sha256((self.output / relative).read_bytes()).hexdigest(),
            )
        for command in commands:
            self.assertIn("core.hooksPath=/dev/null", command)
            self.assertIn("core.fsmonitor=false", command)
            if "rebase" in command:
                self.assertIn("rebase.updateRefs=false", command)
                self.assertIn("rerere.enabled=false", command)
                self.assertNotEqual(command[command.index("-C") + 1], str(self.clone))
            if "diff" in command:
                self.assertIn("--no-ext-diff", command)
                self.assertIn("--no-textconv", command)
            if "update-ref" in command:
                self.assertEqual(command[-1], "0" * 40)
        self.assertIn("rebase_log", row["evidence"])
        if outcome == "CONFLICT":
            self.assertEqual(row["conflict_files"], snapshot["files"])
            self.assertIsNone(row["prepared_ref"])
        else:
            self.assertIn("range_diff", row["evidence"])
            self.assertIn("final_diff", row["evidence"])
            self.assertEqual(self.git("rev-parse", row["prepared_ref"]), row["rebased_sha"])
        return row

    def test_clean_probe_disables_hooks_and_cleans_worktree(self):
        self.probe("OK")

    def test_conflicting_probe_disables_hooks_and_cleans_worktree(self):
        self.probe("CONFLICT")

    def test_empty_probe_records_already_landed_evidence(self):
        row = self.probe("EMPTY")
        self.assertEqual(row["recommendation"], "KEEP_MAIN")


if __name__ == "__main__":
    unittest.main()
