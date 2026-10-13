#!/usr/bin/env python3
"""Credential-free regression tests for the cherry-pick workflows (requires PyYAML)."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[1]
ACTION = yaml.safe_load(
    (ROOT / ".github/actions/git/cherry-pick/action.yaml").read_text()
)
COMMENT = yaml.safe_load(
    (ROOT / ".github/workflows/comment-cherry-pick.yaml").read_text()
)
MERGE = yaml.safe_load(
    (ROOT / ".github/workflows/cherry-pick-on-merge.yaml").read_text()
)
ACTION_STEPS = ACTION["runs"]["steps"]
COMMENT_STEPS = COMMENT["jobs"]["execute"]["steps"]
MERGE_STEPS = MERGE["jobs"]["cherry-pick"]["steps"]

MOCK_COMMAND = r"""
import json
import os
import subprocess
import sys

args = sys.argv[1:]
tool = os.path.basename(sys.argv[0])
with open(os.environ["CALLS"], "a") as log:
    log.write(json.dumps([tool, *args]) + "\n")
if tool == "git":
    if os.environ.get("REAL_GIT"):
        sys.exit(subprocess.run(["/usr/bin/git", *args]).returncode)
    if args[0] == "check-ref-format":
        sys.exit(subprocess.run(["/usr/bin/git", *args]).returncode)
    if args[0] == "ls-remote" and os.environ.get("MISSING_BRANCH"):
        sys.exit(2)
    if args[0] == "cherry-pick" and args[1] != "--abort":
        if os.environ.get("CONFLICT"):
            print("CONFLICT: first line\nsecond line")
            sys.exit(1)
elif args[:2] == ["pr", "list"]:
    if os.environ.get("API_FAILURE"):
        sys.exit(1)
    print(os.environ.get("EXISTING_PR", ""))
"""

JS_HARNESS = r"""
const fs = require('fs');
const input = JSON.parse(fs.readFileSync(0, 'utf8'));
const calls = [];
const outputs = {};
const warnings = [];
const context = {
  repo: {owner: 'test', repo: 'test'},
  payload: {comment: input.comment, pull_request: {number: 42}}
};
const github = {
  rest: {
    issues: {listComments() {}},
    repos: {
      async getCollaboratorPermissionLevel({username}) {
        calls.push(['permission', username]);
        const permission = input.permissions[username];
        if (typeof permission === 'number') {
          throw Object.assign(new Error('Permission API failed'), {status: permission});
        }
        return {data: {permission}};
      },
      async getBranch({branch}) {
        calls.push(['branch', branch]);
        if (input.missingBranch) {
          throw Object.assign(new Error('Branch not found'), {status: 404});
        }
        return {data: {name: branch}};
      }
    }
  },
  async paginate(method, options) {
    calls.push(['paginate', options.per_page]);
    return input.comments;
  }
};
const core = {
  setOutput(name, value) { outputs[name] = value; },
  warning(message) { warnings.push(message); }
};
const AsyncFunction = Object.getPrototypeOf(async function() {}).constructor;
(async () => {
  try {
    const result = await new AsyncFunction('github', 'context', 'core', input.script)(
      github, context, core
    );
    console.log(JSON.stringify({result, calls, outputs, warnings}));
  } catch (error) {
    console.log(JSON.stringify({error: error.message, calls, outputs, warnings}));
  }
})();
"""


class CherryPickTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        for command in ("git", "gh"):
            path = self.directory / command
            path.write_text(f"#!{sys.executable}\n{MOCK_COMMAND}")
            path.chmod(0o755)
        self.env = {
            **os.environ,
            "PATH": f"{self.directory}:{os.environ['PATH']}",
            "CALLS": str(self.directory / "calls"),
            "GITHUB_OUTPUT": str(self.directory / "outputs"),
            "SOURCE_PR": "42",
            "SOURCE_PR_TITLE": "A normal PR",
            "MERGE_COMMIT": "a" * 40,
            "TARGET_BRANCHES": "release-1.19",
            "GH_TOKEN": "unused-test-token",
            "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_TERMINAL_PROMPT": "0",
        }

    def shell(self, step, **env):
        return subprocess.run(
            ["bash", "-e", "-o", "pipefail", "-c", step["run"]],
            env={**self.env, **env},
            text=True,
            capture_output=True,
            cwd=self.directory,
            timeout=10,
        )

    def calls(self):
        path = self.directory / "calls"
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def javascript(self, steps, **values):
        data = {"script": steps[0]["with"]["script"], "permissions": {}, **values}
        result = subprocess.run(
            ["node", "-e", JS_HARNESS],
            input=json.dumps(data),
            text=True,
            capture_output=True,
            check=True,
            timeout=10,
        )
        return json.loads(result.stdout)

    @staticmethod
    def comment(body, author="maintainer", number=1):
        return {"body": body, "user": {"login": author}, "id": number}

    def test_no_input_interpolation_in_shell(self):
        for step in ACTION_STEPS:
            self.assertNotIn("${{", step.get("run", ""))
        for key in ("SOURCE_PR", "SOURCE_PR_TITLE", "MERGE_COMMIT", "TARGET_BRANCHES"):
            self.assertIn(key, ACTION_STEPS[-1]["env"])
        self.assertNotIn("continue-on-error", ACTION_STEPS[-1])

    def test_validation_before_credentials(self):
        self.assertEqual(ACTION_STEPS[0]["name"], "Validate cherry-pick inputs")
        for steps in (COMMENT_STEPS, MERGE_STEPS):
            self.assertIn("getCollaboratorPermissionLevel", steps[0]["with"]["script"])
            checkout = next(step for step in steps if step.get("uses", "").startswith("actions/checkout@"))
            self.assertEqual(checkout["with"]["ref"], "${{ github.event.repository.default_branch }}")
            self.assertFalse(checkout["with"]["persist-credentials"])

    def test_rejects_invalid_action_inputs(self):
        invalid_branches = (
            "", "-option", "release..bad", "release.lock", "release/../main",
            "release-1.19,", ",release-1.19", "release-1.19,,main",
            "release-1.19\nmain", "release-1.19\rmain", "$(touch marker)",
            "`touch marker`", 'branch"quoted', "branch*", "branch:main",
        )
        for branch in invalid_branches:
            with self.subTest(branch=branch):
                self.assertNotEqual(self.shell(ACTION_STEPS[0], TARGET_BRANCHES=branch).returncode, 0)
        for key in ("SOURCE_PR", "MERGE_COMMIT"):
            for value in ("", "-1", "42\n", "$(touch marker)"):
                with self.subTest(key=key, value=value):
                    self.assertNotEqual(self.shell(ACTION_STEPS[0], **{key: value}).returncode, 0)
        self.assertFalse((self.directory / "marker").exists())
        self.assertTrue(all(call[1] == "check-ref-format" for call in self.calls()))

    def test_title_is_literal_and_all_branches_processed(self):
        title = 'Quotes " and $(touch marker) and `touch marker`\nsecond line'
        branches = "release-1.19,release-1.20"
        self.assertEqual(self.shell(ACTION_STEPS[0], TARGET_BRANCHES=branches).returncode, 0)
        result = self.shell(ACTION_STEPS[-1], SOURCE_PR_TITLE=title, TARGET_BRANCHES=branches)
        self.assertEqual(result.returncode, 0, result.stderr)
        creates = [call for call in self.calls() if call[:3] == ["gh", "pr", "create"]]
        self.assertEqual(len(creates), 2)
        for call in creates:
            self.assertEqual(call[call.index("--title") + 1], f"{title} (Cherry-pick #42)")
        self.assertFalse((self.directory / "marker").exists())
        self.assertIn("success=true\n", (self.directory / "outputs").read_text())
        fetches = [call for call in self.calls() if call[1] == "fetch"]
        self.assertEqual(fetches[0][-1], "+refs/heads/release-1.19:refs/remotes/origin/release-1.19")

    def test_existing_pr(self):
        result = self.shell(ACTION_STEPS[-1], EXISTING_PR="123")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(call[:3] == ["gh", "pr", "create"] for call in self.calls()))

    def test_conflict_captured_once_and_reported(self):
        result = self.shell(ACTION_STEPS[-1], CONFLICT="1")
        self.assertNotEqual(result.returncode, 0)
        picks = [call for call in self.calls() if call[1] == "cherry-pick"]
        self.assertEqual(len(picks), 2)
        self.assertEqual(picks[-1], ["git", "cherry-pick", "--abort"])
        outputs = (self.directory / "outputs").read_text()
        self.assertNotIn("success=true", outputs)
        self.assertIn("CONFLICT: first line\nsecond line\n", outputs)
        self.assertIn("git-logs<<CHERRY_PICK_LOGS_", outputs)

    def test_missing_remote_branch_prevents_mutations(self):
        result = self.shell(ACTION_STEPS[-1], MISSING_BRANCH="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual([call[1] for call in self.calls()], ["ls-remote"])

    def test_pr_list_error_does_not_create_pr(self):
        result = self.shell(ACTION_STEPS[-1], API_FAILURE="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[:3] == ["gh", "pr", "create"] for call in self.calls()))

    def test_comment_permission_checks(self):
        for permission in ("read", "triage", "none", 404, 403):
            with self.subTest(permission=permission):
                result = self.javascript(
                    COMMENT_STEPS, comment=self.comment("/cherry-pick release-1.19"),
                    permissions={"maintainer": permission},
                )
                self.assertIn("error", result)
                self.assertEqual(result["outputs"], {})
                self.assertEqual(result["calls"], [["permission", "maintainer"]])
        for permission in ("write", "maintain", "admin"):
            result = self.javascript(
                COMMENT_STEPS, comment=self.comment("/cherry-pick release-1.19\r\nDetails"),
                permissions={"maintainer": permission},
            )
            self.assertEqual(result["outputs"], {"branch": "release-1.19"})

    def test_exact_command_and_branch_existence(self):
        for body in (
            "/cherry-pick-extra release-1.19", "/cherry-pick",
            "/cherry-pick release-1.19 extra", "/cherry-pick $(touch marker)",
            "/cherry-pick -option", "/cherry-pick release-1.19,main",
        ):
            with self.subTest(body=body):
                result = self.javascript(
                    COMMENT_STEPS, comment=self.comment(body),
                    permissions={"maintainer": "write"},
                )
                self.assertIn("error", result)
                self.assertEqual(result["outputs"], {})
        result = self.javascript(
            COMMENT_STEPS, comment=self.comment("/cherry-pick missing"),
            permissions={"maintainer": "write"}, missingBranch=True,
        )
        self.assertIn("error", result)
        self.assertEqual(result["outputs"], {})

    def test_merge_authorizes_each_author_and_deduplicates(self):
        comments = [
            self.comment("/cherry-pick main", "outsider"),
            self.comment("/cherry-pick main", "deleted"),
            self.comment("/cherry-pick release-1.19"),
            self.comment("/cherry-pick release-1.20"),
            self.comment("/cherry-pick release-1.19"),
            self.comment("/cherry-pick $(touch marker)"),
            self.comment("/cherry-pick-extra main"),
        ]
        result = self.javascript(
            MERGE_STEPS, comments=comments,
            permissions={"maintainer": "write", "outsider": "read", "deleted": 404},
        )
        self.assertEqual(result["result"], ["release-1.19", "release-1.20"])
        self.assertEqual(len(result["warnings"]), 2)
        self.assertEqual(result["calls"][0], ["paginate", 100])
        self.assertNotIn(["branch", "main"], result["calls"])
        action = MERGE_STEPS[-1]
        self.assertEqual(action["with"]["target-branches"], "${{ join(fromJSON(steps.cherry.outputs.result), ',') }}")

    def test_merge_api_errors_fail_closed(self):
        for permission in (403, 500):
            result = self.javascript(
                MERGE_STEPS, comments=[self.comment("/cherry-pick release-1.19")],
                permissions={"maintainer": permission},
            )
            self.assertIn("error", result)
            self.assertNotIn("result", result)

    def local_repository(self, conflict=False):
        def git(*args):
            return subprocess.run(
                ["/usr/bin/git", *args], cwd=self.directory, env=self.env,
                text=True, capture_output=True, check=True,
            ).stdout.strip()

        git("init", "-q", "--bare", "-b", "main", str(self.directory / "origin.git"))
        git("init", "-q", "-b", "main")
        git("config", "user.name", "Test Maintainer")
        git("config", "user.email", "maintainer@example.invalid")
        git("config", "commit.gpgsign", "false")
        git("config", "core.hooksPath", str(self.directory / "no-hooks"))
        resource = self.directory / "resource"
        resource.write_text("base\n")
        git("add", "resource")
        git("commit", "-qm", "Base")
        git("branch", "release-1.19")
        git("branch", "release-1.20")
        if conflict:
            git("checkout", "-q", "release-1.19")
            resource.write_text("conflicting release change\n")
            git("commit", "-qam", "Release change")
            git("checkout", "-q", "main")
        git("checkout", "-qb", "feature")
        resource.write_text("source change\n")
        git("commit", "-qam", "Feature")
        git("checkout", "-q", "main")
        git("merge", "--no-ff", "-qm", "Merge feature", "feature")
        merge_commit = git("rev-parse", "HEAD")
        git("remote", "add", "origin", str(self.directory / "origin.git"))
        git("push", "-q", "origin", "main", "release-1.19", "release-1.20")
        return git, merge_commit

    def test_real_git_success(self):
        git, merge_commit = self.local_repository()
        result = self.shell(
            ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit,
            TARGET_BRANCHES="release-1.19,release-1.20",
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        for branch in ("release-1.19", "release-1.20"):
            ref = f"refs/heads/cherry-pick-42-{branch}"
            self.assertEqual(
                git("--git-dir=origin.git", "show", f"{ref}:resource"), "source change",
            )
            self.assertIn(
                "Signed-off-by: Test Maintainer",
                git("--git-dir=origin.git", "log", "-1", "--format=%B", ref),
            )

    def test_real_git_conflict_aborts_and_continues(self):
        git, merge_commit = self.local_repository(conflict=True)
        result = self.shell(
            ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit,
            TARGET_BRANCHES="release-1.19,release-1.20",
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.directory / ".git/CHERRY_PICK_HEAD").exists())
        self.assertEqual(
            git("--git-dir=origin.git", "show", "cherry-pick-42-release-1.20:resource"),
            "source change",
        )
        self.assertEqual(
            git("--git-dir=origin.git", "show", "release-1.19:resource"),
            "conflicting release change",
        )
        outputs = (self.directory / "outputs").read_text()
        self.assertIn("CONFLICT", outputs)
        self.assertNotIn("success=true", outputs)


if __name__ == "__main__":
    unittest.main()
