#!/usr/bin/env python3
"""Offline tests for scripts/sync-pss-upstream.py (requires PyYAML and git).

Each test builds a small local git repository shaped like
kubernetes/pod-security-admission and runs the detector against it. No network
is used: the verifier preview is faked by replacing run_command.
"""

import importlib.util
import io
import os
from pathlib import Path
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
SPEC = importlib.util.spec_from_file_location("sync_pss_upstream", ROOT / "scripts/sync-pss-upstream.py")
sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(sync)

CHECK_SYSCTLS = 'package policy\n\nfunc CheckSysctls() Check {\n\treturn Check{\n\t\tID: "sysctls",\n\t}\n}\n'
CHECK_CAPS = ('package policy\n\nconst checkCapabilitiesBaselineID CheckID = "capabilities_baseline"\n\n'
              'func CheckCapabilitiesBaseline() Check { return Check{ID: checkCapabilitiesBaselineID} }\n')
FIXTURE = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: {name}\nspec: {{}}\n"
K8S_SHA = "a" * 40


class Upstream:
    """A local repository shaped like the PSA mirror, with dated commits."""

    def __init__(self, path):
        self.path = path
        self.when = 1_700_000_000
        path.mkdir(parents=True)
        self.run("init", "-q", "-b", "master")
        self.write({
            "policy/check_sysctls.go": CHECK_SYSCTLS,
            "policy/check_capabilities_baseline.go": CHECK_CAPS,
            "policy/helpers.go": "package policy\n",
            "policy/check_sysctls_test.go": "package policy\n",
            "test/run.go": "package test\n",
            "test/testdata/README.md": "fixtures\n",
            "test/testdata/baseline/v1.37/pass/sysctls1.yaml": FIXTURE.format(name="sysctls1"),
            "test/testdata/restricted/v1.37/pass/sysctls1.yaml": FIXTURE.format(name="sysctls1"),
        })
        self.base = self.commit("Initial")

    def env(self):
        stamp = f"{self.when} +0000"
        return dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@example.com", GIT_COMMITTER_NAME="t",
                    GIT_COMMITTER_EMAIL="t@example.com", GIT_AUTHOR_DATE=stamp, GIT_COMMITTER_DATE=stamp)

    def run(self, *args):
        result = subprocess.run(["git", "-C", str(self.path), *args], capture_output=True, text=True, env=self.env())
        if result.returncode != 0:
            raise AssertionError(f"git {args}: {result.stderr}")
        return result.stdout.strip()

    def write(self, files):
        for name, content in files.items():
            path = self.path / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8", newline="\n")

    def commit(self, message, files=None, body=""):
        if files:
            self.write(files)
        self.when += 3600
        self.run("add", "-A")
        self.run("commit", "-q", "-m", message + (f"\n\n{body}" if body else ""))
        return self.run("rev-parse", "HEAD")

    def tag(self, name, commit, when):
        self.when = when
        self.run("tag", "-a", "-m", name, name, commit)


class DetectorTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.upstream = Upstream(Path(self.tmp.name) / "upstream")
        self.work = 0

    def tearDown(self):
        self.tmp.cleanup()

    def detect(self, pin, branch="master"):
        self.work += 1
        work = Path(self.tmp.name) / f"work{self.work}"
        work.mkdir()
        return sync.detect("git", str(self.upstream.path), branch, pin, work)

    def test_head_equals_pin(self):
        result = self.detect(self.upstream.base)
        self.assertEqual(result["classification"], "none")
        self.assertEqual(result["head"], self.upstream.base)

    def test_test_only_change_is_ignored(self):
        self.upstream.commit("Refactor tests", {"test/run.go": "package test // v2\n",
                                                "policy/check_sysctls_test.go": "package policy // v2\n"})
        result = self.detect(self.upstream.base)
        self.assertEqual(result["classification"], "none")
        self.assertEqual(result["files"], [])

    def test_fixtures_only_change(self):
        self.upstream.commit("Regenerate fixtures for v1.38", {
            "test/testdata/baseline/v1.38/pass/sysctls1.yaml": FIXTURE.format(name="sysctls1"),
            "test/testdata/restricted/v1.38/pass/sysctls1.yaml": FIXTURE.format(name="sysctls1"),
        })
        result = self.detect(self.upstream.base)
        self.assertEqual(result["classification"], "fixtures-only")
        self.assertEqual(result["new_versions"], ["baseline/v1.38", "restricted/v1.38"])
        self.assertEqual(list(result["groups"]), ["fixtures"])
        self.assertEqual(result["go_diff"], "")

    def test_go_check_change_is_manual(self):
        sha = self.upstream.commit("Allow tcp_fastopen", {"policy/check_sysctls.go": CHECK_SYSCTLS + "// v1.38\n"},
                                   body=f"Kubernetes-commit: {K8S_SHA}")
        result = self.detect(self.upstream.base)
        self.assertEqual(result["classification"], "manual")
        self.assertEqual(result["groups"], {"sysctls": ["policy/check_sysctls.go"]})
        self.assertEqual([(c[0], c[2]) for c in result["commits"]], [(sha, K8S_SHA)])
        self.assertIn("+// v1.38", result["go_diff"])

    def test_check_id_from_constant_and_shared_files(self):
        self.upstream.commit("Change caps and helpers", {"policy/check_capabilities_baseline.go": CHECK_CAPS + "// x\n",
                                                         "policy/helpers.go": "package policy // x\n"})
        result = self.detect(self.upstream.base)
        self.assertEqual(result["groups"], {"capabilities_baseline": ["policy/check_capabilities_baseline.go"],
                                            "shared": ["policy/helpers.go"]})

    def test_deleted_check_file_is_still_grouped(self):
        self.upstream.run("rm", "-q", "policy/check_capabilities_baseline.go")
        self.upstream.commit("Remove check")
        result = self.detect(self.upstream.base)
        self.assertEqual(result["groups"], {"capabilities_baseline": ["policy/check_capabilities_baseline.go"]})

    def test_first_release_is_earliest_by_date_not_version(self):
        sha = self.upstream.commit("Change sysctls", {"policy/check_sysctls.go": CHECK_SYSCTLS + "// y\n"})
        # The alpha is tagged first. sort -V would pick kubernetes-1.38.0.
        self.upstream.tag("kubernetes-1.38.0-alpha.1", sha, 1_800_000_000)
        self.upstream.tag("kubernetes-1.38.0", sha, 1_810_000_000)
        result = self.detect(self.upstream.base)
        self.assertEqual(result["commits"][0][3], "kubernetes-1.38.0-alpha.1")

    def test_unreleased_commit(self):
        self.upstream.commit("Change sysctls", {"policy/check_sysctls.go": CHECK_SYSCTLS + "// z\n"})
        self.assertEqual(self.detect(self.upstream.base)["commits"][0][3], "unreleased")

    def test_rewritten_history_is_an_error(self):
        old = self.upstream.commit("Will be dropped", {"policy/helpers.go": "package policy // old\n"})
        self.upstream.run("reset", "-q", "--hard", self.upstream.base)
        self.upstream.commit("Replacement", {"policy/helpers.go": "package policy // new\n"})
        # Keep the dropped commit reachable so only the ancestry check fails.
        self.upstream.run("tag", "dropped", old)
        with self.assertRaisesRegex(sync.VerifyError, "isn't an ancestor"):
            self.detect(old)

    def test_missing_pinned_commit_is_an_error(self):
        with self.assertRaisesRegex(sync.VerifyError, "isn't in"):
            self.detect("0" * 40)

    def test_missing_branch_is_an_error(self):
        with self.assertRaisesRegex(sync.VerifyError, "git fetch"):
            self.detect(self.upstream.base, branch="no-such-branch")


class PreviewTest(unittest.TestCase):
    args = mock.Mock(cli="kubectl-kyverno", helm="helm", jobs=2, mapping=None)

    def preview(self, returncode, stdout):
        completed = subprocess.CompletedProcess([], returncode, stdout, "")
        with mock.patch.object(sync.verifier, "run_command", return_value=completed) as run:
            return sync.run_preview(self.args, Path("pin.yaml")), run.call_args.args[0]

    def test_candidate_passes(self):
        (status, summary, _), command = self.preview(0, "checked 10 fixture results across 4 renders: 0 unexpected, 2 known, 0 stale\n")
        self.assertEqual(status, "pass")
        self.assertIn("--pin", command)

    def test_mismatch_is_a_failure_not_a_pass(self):
        out = ("FAIL: baseline v1.38 userns=false: pass/sysctls1 rejected by restrict-sysctls\n"
               "checked 10 fixture results across 4 renders: 1 unexpected, 2 known, 0 stale\n")
        (status, summary, output), _ = self.preview(1, out)
        self.assertEqual(status, "fail")
        self.assertIn("1 unexpected", summary)

    def test_stale_entry_is_a_failure(self):
        (status, _, _), _ = self.preview(1, "checked 10 fixture results across 4 renders: 0 unexpected, 1 known, 1 stale\n")
        self.assertEqual(status, "fail")

    def test_incomplete_check_is_an_error(self):
        with self.assertRaisesRegex(sync.VerifyError, "didn't complete"):
            self.preview(1, "ERROR: command not found: helm\n")


class ReportAndMainTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self.tmp.name)
        self.pin = self.dir / "upstream.yaml"
        self.pin.write_text(yaml.safe_dump({"repository": "https://github.com/kubernetes/pod-security-admission",
                                            "branch": "master", "commit": "1" * 40}))

    def tearDown(self):
        self.tmp.cleanup()

    def result(self, classification):
        return {"repository": "https://github.com/kubernetes/pod-security-admission", "branch": "master",
                "pin": "1" * 40, "head": "2" * 40, "classification": classification,
                "files": ["policy/check_sysctls.go"], "groups": {"sysctls": ["policy/check_sysctls.go"]},
                "new_versions": [], "commits": [("2" * 40, "Allow tcp_fastopen", K8S_SHA, "unreleased")],
                "go_diff": "+allowed\n"}

    def main(self, *argv, result=None, preview=None):
        out = io.StringIO()
        with mock.patch.object(sync, "detect", return_value=result), \
             mock.patch.object(sync, "run_preview", return_value=preview), redirect_stdout(out):
            code = sync.main(["--out", str(self.dir / "out"), "--pin", str(self.pin), *argv])
        return code, out.getvalue()

    def test_report_for_manual_change(self):
        report = sync.render_report(self.result("manual"), ("fail", "checked 1: 1 unexpected", "FAIL: x\nnoise\n"))
        self.assertIn("Classification: **manual**", report)
        self.assertIn("The candidate **fails** the fixture check", report)
        self.assertIn("FAIL: x", report)
        self.assertNotIn("noise", report)
        self.assertIn("check `sysctls`", report)
        self.assertIn("kubernetes/kubernetes@aaaaaaaaaaaa", report)
        self.assertIn("+allowed", report)

    def test_exit_codes(self):
        none = dict(self.result("manual"), classification="none")
        self.assertEqual(self.main(result=none)[0], 0)
        self.assertFalse((self.dir / "out/upstream.yaml").exists())
        self.assertEqual(self.main(result=self.result("manual"))[0], 2)
        candidate = yaml.safe_load((self.dir / "out/upstream.yaml").read_text())
        self.assertEqual(candidate["commit"], "2" * 40)
        passing = ("pass", "checked: 0 unexpected", "")
        self.assertEqual(self.main("--preview", "--cli", "k", result=self.result("manual"), preview=passing)[0], 2)
        failing = ("fail", "checked: 4 unexpected", "")
        self.assertEqual(self.main("--preview", "--cli", "k", result=self.result("manual"), preview=failing)[0], 3)

    def test_errors_exit_1(self):
        code, out = self.main("--repository", "file:///tmp/psa")
        self.assertEqual(code, 1)
        self.assertIn("https://", out)
        code, out = self.main("--branch", "a..b")
        self.assertEqual((code, "invalid branch name" in out), (1, True))
        self.pin.write_text("repository: https://github.com/x/y\ncommit: abc\n")
        code, out = self.main()
        self.assertEqual((code, "40-character" in out), (1, True))
        with mock.patch.object(sync, "detect", side_effect=sync.VerifyError("boom")), redirect_stdout(io.StringIO()) as out:
            self.pin.write_text(yaml.safe_dump({"repository": "https://github.com/x/y", "commit": "1" * 40}))
            self.assertEqual(sync.main(["--out", str(self.dir / "out"), "--pin", str(self.pin)]), 1)
        self.assertIn("ERROR: boom", out.getvalue())

    def test_preview_needs_cli(self):
        with self.assertRaises(SystemExit), redirect_stdout(io.StringIO()), mock.patch("sys.stderr", io.StringIO()):
            sync.main(["--out", "x", "--preview"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
