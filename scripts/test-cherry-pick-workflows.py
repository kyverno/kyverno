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
CONFLICT_CHECK = yaml.safe_load(
    (ROOT / ".github/workflows/check-cherry-pick-conflicts.yaml").read_text()
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
        if args[0] == "push" and os.environ.get("RACE_BRANCH"):
            subprocess.run(
                ["/usr/bin/git", "--git-dir=origin.git", "update-ref",
                 args[-1].split(":")[1], "refs/heads/release-1.19"],
                check=True,
            )
        sys.exit(subprocess.run(["/usr/bin/git", *args]).returncode)
    if args[0] == "check-ref-format":
        sys.exit(subprocess.run(["/usr/bin/git", *args]).returncode)
    if args[0] == "ls-remote" and os.environ.get("MISSING_BRANCH"):
        sys.exit(2)
    if args[:2] == ["ls-remote", "--heads"] and os.environ.get("EXISTING_BRANCH"):
        print("a" * 40 + "\t" + args[-1])
    if args[:3] == ["diff", "--name-only", "--diff-filter=U"] and os.environ.get("CONFLICT"):
        print("resource")
    if args[0] == "cherry-pick" and args[1] != "--abort":
        if os.environ.get("CONFLICT"):
            print("CONFLICT: first line\nsecond line")
            sys.exit(1)
        if os.environ.get("PICK_ERROR"):
            print("error: invalid object")
            sys.exit(1)
    if args[0] == "push" and os.environ.get("PUSH_ERROR"):
        print("remote rejected: workflows permission required", file=sys.stderr)
        sys.exit(1)
elif args[:2] == ["pr", "list"]:
    if os.environ.get("API_FAILURE"):
        sys.exit(1)
    print(os.environ.get("EXISTING_PR", ""))
elif args[:2] == ["pr", "create"]:
    with open(os.environ["PR_BODIES"], "a") as bodies:
        with open(args[args.index("--body-file") + 1]) as body:
            bodies.write(json.dumps(body.read()) + "\n")
    if os.environ.get("CREATE_ERROR"):
        sys.exit(1)
    print("https://github.com/test/test/pull/123")
"""

JS_HARNESS = r"""
const fs = require('fs');
const input = JSON.parse(fs.readFileSync(0, 'utf8'));
const calls = [];
const outputs = {};
const warnings = [];
const context = {
  repo: {owner: 'test', repo: 'test'},
  serverUrl: 'https://github.com',
  runId: 1234,
  issue: {number: 42},
  payload: {comment: input.comment, pull_request: {number: 42}}
};
const github = {
  rest: {
    issues: {
      listComments() {},
      async createComment(data) { calls.push(['comment', data.body]); }
    },
    reactions: {
      async createForIssueComment() {
        calls.push(['reaction']);
        if (input.reactionError) {
          throw Object.assign(new Error('Reaction failed'), {status: input.reactionError});
        }
      }
    },
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
        if (input.branchErrors && input.branchErrors[branch]) {
          throw Object.assign(new Error('Branch API failed'), {status: input.branchErrors[branch]});
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
  warning(message) { warnings.push(message); },
  summary: {
    addHeading() { return this; },
    addRaw(body) { calls.push(['summary', body]); return this; },
    async write() {}
  }
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
        template = self.directory / ".github/PULL_REQUEST_TEMPLATE.md"
        template.parent.mkdir()
        template.write_text((ROOT / ".github/PULL_REQUEST_TEMPLATE.md").read_text())
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
            "GITHUB_SERVER_URL": "https://github.com",
            "GITHUB_REPOSITORY": "test/test",
            "GITHUB_RUN_ID": "1234",
            "GITHUB_STEP_SUMMARY": str(self.directory / "summary"),
            "PR_BODIES": str(self.directory / "bodies"),
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

    def javascript(self, steps, environment=None, **values):
        data = {"script": steps[0]["with"]["script"], "permissions": {}, **values}
        result = subprocess.run(
            ["node", "-e", JS_HARNESS],
            input=json.dumps(data),
            text=True,
            capture_output=True,
            check=True,
            timeout=10,
            env={**os.environ, **(environment or {})},
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

    def test_comment_reaction_permissions(self):
        self.assertEqual(COMMENT["jobs"]["execute"]["permissions"]["issues"], "write")

    def test_workflow_token_permissions(self):
        for steps in (COMMENT_STEPS, MERGE_STEPS):
            token = next(step for step in steps if step.get("id") == "app-token")
            self.assertEqual(token["with"]["permission-workflows"], "write")
            report = next(step for step in steps if step["name"] in (
                "Comment Cherry-pick Fail", "Report cherry-pick failure",
            ))
            self.assertIn("failure()", report["if"])
            self.assertNotIn("github-token", report["with"])
            self.assertIn("core.summary", report["with"]["script"])
            self.assertIn("issues.createComment", report["with"]["script"])

    def test_deleted_comment_reaction_is_nonfatal(self):
        reaction = next(step for step in COMMENT_STEPS if step["name"] == "React to comment")
        result = self.javascript([reaction], comment=self.comment("/cherry-pick main"), reactionError=404)
        self.assertNotIn("error", result)
        self.assertIn("no longer exists", result["warnings"][0])
        for status in (403, 500):
            result = self.javascript([reaction], comment=self.comment("/cherry-pick main"), reactionError=status)
            self.assertIn("error", result)

    def test_setup_failures_are_reported_without_app_token(self):
        for steps, name in ((COMMENT_STEPS, "Comment Cherry-pick Fail"), (MERGE_STEPS, "Report cherry-pick failure")):
            report = next(step for step in steps if step["name"] == name)
            result = self.javascript(
                [report], environment={
                    "FAILURE_STAGE": "private-key validation",
                    "TARGET_BRANCH": "release-1.19",
                },
            )
            self.assertNotIn("error", result)
            body = next(call[1] for call in result["calls"] if call[0] == "comment")
            self.assertIn("private-key validation", body)
            self.assertIn("https://github.com/test/test/actions/runs/1234", body)

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
        result = self.shell(ACTION_STEPS[-1], EXISTING_PR="123 OPEN")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(any(call[:3] == ["gh", "pr", "create"] for call in self.calls()))
        self.assertFalse(any(call[1] in ("checkout", "cherry-pick", "push") for call in self.calls()))
        query = next(call for call in self.calls() if call[:3] == ["gh", "pr", "list"])
        self.assertEqual(query[query.index("--state") + 1], "all")

    def test_merged_backport_is_skipped_but_closed_unmerged_is_reported(self):
        for state, expected in (("MERGED", 0), ("CLOSED", 1)):
            with self.subTest(state=state):
                result = self.shell(ACTION_STEPS[-1], EXISTING_PR=f"123 {state}")
                self.assertEqual(result.returncode, expected, result.stderr)
                self.assertFalse(any(call[1] == "push" for call in self.calls()))

    def test_conflict_creates_draft_and_reports_manual_resolution(self):
        result = self.shell(ACTION_STEPS[-1], CONFLICT="1")
        self.assertEqual(result.returncode, 0, result.stderr)
        picks = [call for call in self.calls() if call[1] == "cherry-pick"]
        self.assertEqual(len(picks), 1)
        create = next(call for call in self.calls() if call[:3] == ["gh", "pr", "create"])
        self.assertIn("--draft", create)
        body = json.loads((self.directory / "bodies").read_text())
        self.assertIn("Manual conflict resolution required", body)
        self.assertIn("resource", body)
        self.assertIn("do not run `git cherry-pick --continue`", body)
        self.assertIn("https://github.com/test/test/actions/runs/1234", body)
        outputs = (self.directory / "outputs").read_text()
        self.assertIn("success=true", outputs)
        self.assertIn("CONFLICT: first line\nsecond line\n", outputs)
        self.assertIn("git-logs<<CHERRY_PICK_LOGS_", outputs)

    def test_existing_branch_is_never_overwritten(self):
        result = self.shell(ACTION_STEPS[-1], EXISTING_BRANCH="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[1] in ("checkout", "cherry-pick", "push") for call in self.calls()))
        self.assertIn("open its PR manually", (self.directory / "summary").read_text())

    def test_non_conflict_failure_is_not_a_draft(self):
        result = self.shell(ACTION_STEPS[-1], PICK_ERROR="1")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[:3] == ["gh", "pr", "create"] for call in self.calls()))
        self.assertIn("invalid object", (self.directory / "outputs").read_text())

    def test_infrastructure_errors_preserve_failure_summary(self):
        for failure, stage in (("PUSH_ERROR", "pushing"), ("CREATE_ERROR", "creating PR")):
            with self.subTest(failure=failure):
                result = self.shell(ACTION_STEPS[-1], **{failure: "1"})
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("success=false", (self.directory / "outputs").read_text())
                self.assertIn(stage, (self.directory / "summary").read_text())

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
        action = next(step for step in MERGE_STEPS if step.get("id") == "cherry-pick")
        self.assertEqual(action["with"]["target-branches"], "${{ join(fromJSON(steps.cherry.outputs.result), ',') }}")

    def test_merge_api_errors_fail_closed(self):
        for permission in (403, 500):
            result = self.javascript(
                MERGE_STEPS, comments=[self.comment("/cherry-pick release-1.19")],
                permissions={"maintainer": permission},
            )
            self.assertIn("error", result)
            self.assertNotIn("result", result)

    def test_merge_skips_comments_without_authors(self):
        for user in (None, {}):
            with self.subTest(user=user):
                authorless = self.comment("/cherry-pick main", number=10)
                authorless["user"] = user
                result = self.javascript(
                    MERGE_STEPS,
                    comments=[authorless, self.comment("/cherry-pick release-1.19")],
                    permissions={"maintainer": "write"},
                )
                self.assertEqual(result["result"], ["release-1.19"])
                self.assertEqual(len(result["warnings"]), 1)
                self.assertIn("without an author", result["warnings"][0])
                self.assertEqual(
                    result["calls"],
                    [["paginate", 100], ["permission", "maintainer"], ["branch", "release-1.19"]],
                )

    def test_merge_skips_missing_target_and_keeps_valid_targets(self):
        result = self.javascript(
            MERGE_STEPS, comments=[
                self.comment("/cherry-pick deleted-target"),
                self.comment("/cherry-pick release-1.19"),
            ],
            permissions={"maintainer": "write"},
            branchErrors={"deleted-target": 404},
        )
        self.assertEqual(result["result"], ["release-1.19"])
        self.assertEqual(len(result["warnings"]), 1)
        self.assertIn("deleted-target", result["warnings"][0])

    def test_merge_branch_api_errors_fail_closed(self):
        for status in (403, 500):
            with self.subTest(status=status):
                result = self.javascript(
                    MERGE_STEPS, comments=[self.comment("/cherry-pick release-1.19")],
                    permissions={"maintainer": "write"},
                    branchErrors={"release-1.19": status},
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
        (self.directory / ".git/info/exclude").write_text(
            "/git\n/gh\n/calls\n/outputs\n/summary\n/bodies\n/.github/PULL_REQUEST_TEMPLATE.md\n/origin.git/\n"
        )
        resource = self.directory / "resource"
        resource.write_bytes(b"base\0\n" if conflict == "binary" else b"base\n")
        git("add", "resource")
        git("commit", "-qm", "Base")
        git("branch", "release-1.19")
        git("branch", "release-1.20")
        if conflict:
            git("checkout", "-q", "release-1.19")
            if conflict == "modify-delete":
                resource.unlink()
            else:
                resource.write_bytes(
                    b"release\0\n" if conflict == "binary" else b"conflicting release change\n"
                )
            git("commit", "-qam", "Release change")
            git("checkout", "-q", "main")
        git("checkout", "-qb", "feature")
        resource.write_bytes(b"source\0\n" if conflict == "binary" else b"source change\n")
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

    def test_real_git_conflict_creates_draft_and_continues(self):
        git, merge_commit = self.local_repository(conflict=True)
        result = self.shell(
            ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit,
            TARGET_BRANCHES="release-1.19,release-1.20",
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse((self.directory / ".git/CHERRY_PICK_HEAD").exists())
        self.assertEqual(
            git("--git-dir=origin.git", "show", "cherry-pick-42-release-1.20:resource"),
            "source change",
        )
        self.assertEqual(
            git("--git-dir=origin.git", "show", "release-1.19:resource"),
            "conflicting release change",
        )
        conflict = git("--git-dir=origin.git", "show", "cherry-pick-42-release-1.19:resource")
        self.assertIn("<<<<<<< HEAD", conflict)
        self.assertIn("source change", conflict)
        creates = [call for call in self.calls() if call[:3] == ["gh", "pr", "create"]]
        self.assertIn("--draft", creates[0])
        self.assertNotIn("--draft", creates[1])
        outputs = (self.directory / "outputs").read_text()
        self.assertIn("CONFLICT", outputs)
        self.assertIn("success=true", outputs)

    def test_real_git_already_applied_is_success(self):
        git, merge_commit = self.local_repository()
        git("checkout", "-q", "release-1.19")
        git("cherry-pick", "-m", "1", merge_commit)
        git("push", "-q", "origin", "release-1.19")
        result = self.shell(ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("empty/already applied", (self.directory / "summary").read_text())
        self.assertFalse(any(call[:3] == ["gh", "pr", "create"] for call in self.calls()))
        self.assertFalse((self.directory / ".git/CHERRY_PICK_HEAD").exists())

    def test_real_git_binary_conflict_has_explicit_resolution_gate(self):
        self.check_non_text_conflict("binary")

    def test_real_git_modify_delete_conflict_has_explicit_resolution_gate(self):
        self.check_non_text_conflict("modify-delete")

    def check_non_text_conflict(self, kind):
        git, merge_commit = self.local_repository(conflict=kind)
        result = self.shell(ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit)
        self.assertEqual(result.returncode, 0, result.stderr)
        create = next(call for call in self.calls() if call[:3] == ["gh", "pr", "create"])
        self.assertIn("--draft", create)
        manifest = git(
            "--git-dir=origin.git", "show",
            "cherry-pick-42-release-1.19:.github/cherry-pick-conflicts",
        )
        self.assertIn("resource", manifest)
        guard = CONFLICT_CHECK["jobs"]["check"]["steps"][-1]
        result = self.shell(
            guard, REAL_GIT="1", BASE_SHA=git("rev-parse", "release-1.19"),
            HEAD_SHA=git("rev-parse", "HEAD"),
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(".github/cherry-pick-conflicts", result.stdout)

    def test_real_git_rerun_preserves_maintainer_work(self):
        git, merge_commit = self.local_repository()
        result = self.shell(ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit)
        self.assertEqual(result.returncode, 0, result.stderr)
        (self.directory / "resource").write_text("maintainer resolution\n")
        git("commit", "-qam", "Maintainer resolution")
        git("push", "-q", "origin", "cherry-pick-42-release-1.19")
        before = git("--git-dir=origin.git", "rev-parse", "cherry-pick-42-release-1.19")
        result = self.shell(
            ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit, EXISTING_PR="123 OPEN",
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(before, git("--git-dir=origin.git", "rev-parse", "cherry-pick-42-release-1.19"))
        # Even if PR creation failed, the existing remote branch must survive.
        result = self.shell(ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(before, git("--git-dir=origin.git", "rev-parse", "cherry-pick-42-release-1.19"))

    def test_real_git_concurrent_branch_creation_is_rejected(self):
        git, merge_commit = self.local_repository()
        result = self.shell(
            ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit, RACE_BRANCH="1",
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(
            git("--git-dir=origin.git", "rev-parse", "cherry-pick-42-release-1.19"),
            git("rev-parse", "release-1.19"),
        )
        self.assertFalse(any(call[:3] == ["gh", "pr", "create"] for call in self.calls()))

    def test_conflict_guard_uses_only_trusted_base_and_no_pr_execution(self):
        job = CONFLICT_CHECK["jobs"]["check"]
        self.assertEqual(job["permissions"], {"contents": "read"})
        checkout, guard = job["steps"]
        self.assertEqual(checkout["with"]["ref"], "${{ github.event.pull_request.base.sha }}")
        self.assertFalse(checkout["with"]["persist-credentials"])
        self.assertNotIn("${{", guard["run"])
        git, merge_commit = self.local_repository(conflict=True)
        result = self.shell(ACTION_STEPS[-1], REAL_GIT="1", MERGE_COMMIT=merge_commit)
        self.assertEqual(result.returncode, 0, result.stderr)
        head = git("rev-parse", "cherry-pick-42-release-1.19")
        base = git("rev-parse", "release-1.19")
        result = self.shell(guard, REAL_GIT="1", BASE_SHA=base, HEAD_SHA=head)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(".github/cherry-pick-conflicts", result.stdout)
        git("rm", ".github/cherry-pick-conflicts")
        git("commit", "-qm", "Remove resolution checklist but leave markers")
        head = git("rev-parse", "HEAD")
        git("push", "-q", "origin", "cherry-pick-42-release-1.19")
        result = self.shell(guard, REAL_GIT="1", BASE_SHA=base, HEAD_SHA=head)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Unresolved conflict markers", result.stdout)
        (self.directory / "resource").write_text("resolved\n")
        git("commit", "-qam", "Resolve conflicts")
        head = git("rev-parse", "HEAD")
        git("push", "-q", "origin", "cherry-pick-42-release-1.19")
        result = self.shell(guard, REAL_GIT="1", BASE_SHA=base, HEAD_SHA=head)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main()
