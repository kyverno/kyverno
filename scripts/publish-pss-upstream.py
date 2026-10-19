#!/usr/bin/env python3
"""Publishes a detected upstream PSA change as a single pull request.

Reads the output directory of scripts/sync-pss-upstream.py (result.json,
upstream.yaml, report.md) and creates or updates one PR that moves the pin in
scripts/pss-upstream/upstream.yaml:

- A fixtures-only change whose candidate commit passes the fixture check opens
  a ready PR.
- A manual change, or any change whose candidate commit fails the fixture check,
  opens a draft PR with the needs-manual-cel label, and turns an existing ready
  bot PR back into a draft.

Safety rules:

- The bot branch only moves forward. Each change is a new commit on top of the
  bot's own tip, or on top of the base branch when the branch doesn't exist or
  is already merged. Pushes are never forced, so a concurrent change makes Git
  reject the push.
- The branch is owned only when every commit it adds on top of the base is a
  single-parent commit whose author and committer are the bot. Otherwise the
  script doesn't push, posts one comment on the open PR, and exits with 3.
- It never merges, approves, closes, or deletes anything, and never marks a
  draft ready for review. request() refuses those calls.

Usage: scripts/publish-pss-upstream.py --result DIR --repo OWNER/NAME
                                       [--base BRANCH] [--branch BRANCH]
                                       [--remote-url URL] [--dry-run]

Environment:
  GH_TOKEN             token for gh and for git push (not needed for a local remote)
  APP_SLUG             GitHub App slug; the bot is <slug>[bot]
  BOT_NAME, BOT_EMAIL  explicit bot identity instead of APP_SLUG

Exit codes: 0 done or nothing to do, 3 paused because the branch has commits
from someone else, 1 error. Nothing is pushed on 1 or 3.
"""

import argparse
import base64
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[1]
VERIFIER = ROOT / "scripts/verify-policies-pss-fixtures.py"
PIN_PATH = "scripts/pss-upstream/upstream.yaml"
BOT_BRANCH = "bot/pss-upstream-sync"
LABEL = "needs-manual-cel"
LABEL_COLOR = "d93f0b"
LABEL_DESCRIPTION = "Upstream PSA change that needs CEL policy changes by hand"
BODY_MARKER = "<!-- pss-upstream-sync -->"
PAUSE_MARKER = "<!-- pss-upstream-sync-paused -->"
MAX_BODY = 60000
TRUNCATED_NOTE = ("\n**The report is cut here, because it's longer than a PR description can be.** The full "
                  "report is in the `pss-upstream-sync` artifact of the workflow run that opened or updated this PR.\n")
EXIT_DONE, EXIT_ERROR, EXIT_PAUSED = 0, 1, 3

sys.dont_write_bytecode = True
_spec = importlib.util.spec_from_file_location("verify_pss_fixtures", VERIFIER)
verifier = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(verifier)
PublishError = verifier.VerifyError


def log(message):
    print(f"[publish-pss-upstream] {message}", file=sys.stderr)


class GitHub:
    """A small GitHub REST and GraphQL client on top of `gh api`.

    Every call goes through request(), which refuses anything that could merge,
    approve, close, delete, or mark a PR ready.
    """

    def __init__(self, repo, dry_run=False):
        self.repo = repo
        self.owner = repo.split("/", 1)[0]
        self.dry_run = dry_run
        self.planned = []

    @staticmethod
    def check_allowed(method, path, body):
        if method == "DELETE":
            raise PublishError(f"refusing DELETE {path}")
        if re.search(r"/pulls/\d+/(merge|reviews)$", path):
            raise PublishError(f"refusing {method} {path}: the bot never merges or approves")
        if method == "PATCH" and re.search(r"/(pulls|issues)/\d+$", path) and "state" in (body or {}):
            raise PublishError(f"refusing to change the state of {path}")
        query = (body or {}).get("query", "") if path == "graphql" else ""
        if re.search(r"markPullRequestReadyForReview|mergePullRequest|closePullRequest|addPullRequestReview", query):
            raise PublishError("refusing a GraphQL mutation that marks ready, merges, closes, or reviews")

    def request(self, method, path, body=None):
        self.check_allowed(method, path, body)
        if self.dry_run and method != "GET" and not (path == "graphql" and body.get("query", "").startswith("query")):
            self.planned.append(f"{method} {path}")
            log(f"dry run: would {method} {path}")
            return {}
        command = ["gh", "api", "-X", method, path, "-H", "Accept: application/vnd.github+json"]
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False, encoding="utf-8") as handle:
            json.dump(body or {}, handle)
            body_file = handle.name
        try:
            if body is not None:
                command += ["--input", body_file]
            result = verifier.run_command(command)
        finally:
            os.unlink(body_file)
        if result.returncode != 0:
            if method == "GET" and ('"status":"404"' in result.stdout.replace(" ", "") or "(HTTP 404)" in result.stderr):
                return None
            raise PublishError(f"gh api {method} {path} failed: {(result.stderr or result.stdout).strip()[-500:]}")
        return json.loads(result.stdout) if result.stdout.strip() else {}

    def open_pr(self, branch, base):
        prs = self.request("GET", f"repos/{self.repo}/pulls?state=open&base={base}&head={self.owner}:{branch}")
        return prs[0] if prs else None

    def create_pr(self, title, body, branch, base, draft):
        return self.request("POST", f"repos/{self.repo}/pulls",
                            {"title": title, "body": body, "head": branch, "base": base, "draft": draft})

    def update_pr(self, number, title, body):
        return self.request("PATCH", f"repos/{self.repo}/pulls/{number}", {"title": title, "body": body})

    def convert_to_draft(self, node_id):
        return self.request("POST", "graphql", {
            "query": "mutation($id: ID!) { convertPullRequestToDraft(input: {pullRequestId: $id}) "
                     "{ pullRequest { isDraft } } }",
            "variables": {"id": node_id}})

    def ensure_label(self):
        if self.request("GET", f"repos/{self.repo}/labels/{LABEL}") is None:
            self.request("POST", f"repos/{self.repo}/labels",
                         {"name": LABEL, "color": LABEL_COLOR, "description": LABEL_DESCRIPTION})

    def add_label(self, number):
        return self.request("POST", f"repos/{self.repo}/issues/{number}/labels", {"labels": [LABEL]})

    def comments(self, number):
        return self.request("GET", f"repos/{self.repo}/issues/{number}/comments?per_page=100") or []

    def comment(self, number, body):
        return self.request("POST", f"repos/{self.repo}/issues/{number}/comments", {"body": body})

    def bot_user_id(self, slug):
        user = self.request("GET", f"users/{slug}%5Bbot%5D")
        if not user:
            raise PublishError(f"can't find the GitHub App bot user {slug}[bot]")
        return user["id"]


class Git:
    """Runs git in a scratch repository that tracks only the commit graph."""

    def __init__(self, git_bin, work, remote_url, token=None):
        self.git_bin = git_bin
        self.work = work
        self.env = dict(os.environ, GIT_TERMINAL_PROMPT="0")
        if token and remote_url.startswith("https://"):
            # Pass the credential through the environment, never as an argument.
            basic = base64.b64encode(f"x-access-token:{token}".encode()).decode()
            self.env.update(GIT_CONFIG_COUNT="1", GIT_CONFIG_KEY_0="http.https://github.com/.extraheader",
                            GIT_CONFIG_VALUE_0=f"AUTHORIZATION: basic {basic}")
        self.run("init", "-q")
        self.run("remote", "add", "origin", remote_url)

    def run(self, *args, env=None, check=True):
        try:
            result = subprocess.run([self.git_bin, "-C", str(self.work), *args], capture_output=True, text=True,
                                    env=dict(self.env, **(env or {})))
        except OSError as err:
            raise PublishError(f"can't run {self.git_bin}: {err}") from err
        if check and result.returncode != 0:
            raise PublishError(f"git {' '.join(args[:3])} failed: {result.stderr.strip()[-500:]}")
        return result

    def out(self, *args, **kwargs):
        return self.run(*args, **kwargs).stdout.strip()

    def remote_heads(self, *branches):
        lines = self.out("ls-remote", "--heads", "origin", *[f"refs/heads/{b}" for b in branches])
        heads = {}
        for line in lines.splitlines():
            sha, ref = line.split("\t")
            heads[ref[len("refs/heads/"):]] = sha
        return heads

    def fetch(self, *branches):
        refspecs = [f"+refs/heads/{b}:refs/remotes/origin/{b}" for b in branches]
        self.run("fetch", "-q", "--filter=tree:0", "origin", *refspecs)

    def is_ancestor(self, older, newer):
        return self.run("merge-base", "--is-ancestor", older, newer, check=False).returncode == 0


def owned(git, base_sha, branch_sha, bot_email):
    """Returns (owned, reason) for the commits the branch adds on top of the base."""
    log_out = git.out("log", "--format=%H %P%x1f%ae%x1f%ce", f"{base_sha}..{branch_sha}")
    for line in filter(None, log_out.splitlines()):
        shas, author, committer = line.split("\x1f")
        parts = shas.split()
        if len(parts) != 2:
            return False, f"commit {parts[0][:12]} is a merge commit"
        if author != bot_email or committer != bot_email:
            return False, f"commit {parts[0][:12]} is by {author}, not the bot"
    return True, ""


def pin_text(original, candidate):
    """Returns the pin file with the candidate values and the original header comments."""
    header = "".join(line for line in original.splitlines(keepends=True) if line.startswith("#"))
    values = {key: candidate[key] for key in ("repository", "branch", "commit")}
    return header + yaml.safe_dump(values, sort_keys=False)


def build_commit(git, start, text, message, bot_name, bot_email):
    """Creates a commit on start that sets the pin file to text, without a checkout."""
    entry = git.out("ls-tree", start, "--", PIN_PATH)
    if not entry:
        raise PublishError(f"{PIN_PATH} doesn't exist on the starting commit {start[:12]}")
    mode = entry.split()[0]
    path = git.work / ".pss-pin"
    path.write_text(text, encoding="utf-8", newline="\n")
    blob = git.out("hash-object", "-w", str(path))
    git.run("read-tree", start)
    git.run("update-index", "--cacheinfo", f"{mode},{blob},{PIN_PATH}")
    tree = git.out("write-tree")
    identity = {"GIT_AUTHOR_NAME": bot_name, "GIT_AUTHOR_EMAIL": bot_email,
                "GIT_COMMITTER_NAME": bot_name, "GIT_COMMITTER_EMAIL": bot_email}
    full = f"{message}\n\nSigned-off-by: {bot_name} <{bot_email}>\n"
    return git.out("commit-tree", tree, "-p", start, "-m", full, env=identity)


def needs_manual_work(result):
    """Returns True when the PR must be a labeled draft.

    A Go check change needs review even if the chart still matches. A fixtures-only
    change whose candidate fails the fixture check needs CEL changes too, for
    example after a manual bot PR was merged without its CEL fix.
    """
    return result["classification"] == "manual" or result.get("preview") == "fail"


def pr_title(result):
    return f"chore(pss): sync with upstream Pod Security Admission {result['head'][:7]}"


def pr_body(result, report):
    lines = [BODY_MARKER, ""]
    if needs_manual_work(result):
        reason = ("A Go check changed upstream, so the CEL policies may need changes by hand."
                  if result["classification"] == "manual" else
                  "The candidate commit fails the PSS fixture check, so the CEL policies need changes by hand.")
        lines += ["> [!IMPORTANT]",
                  f"> {reason} This PR stays a draft until a maintainer finishes that work and marks it ready.", ""]
    preview = {"pass": "passes", "fail": "**fails**"}.get(result.get("preview"), "wasn't checked against")
    lines += [f"This PR moves `{PIN_PATH}` to `{result['head']}`. The candidate commit {preview} the PSS "
              "fixture check" + (f": `{result['preview_summary']}`" if result.get("preview_summary") else "") + ".",
              "",
              "It was opened by `scripts/publish-pss-upstream.py`. The bot never merges, approves, or marks this "
              "PR ready. If you push your own commits to this branch, the bot stops updating it.",
              "", "---", ""]
    head = "\n".join(lines)
    report = report.rstrip("\n")
    # GitHub rejects a PR body over 65,536 characters, and a large upstream Go
    # diff can produce a longer report, so cut the report and point to the artifact.
    room = MAX_BODY - len(head) - len(TRUNCATED_NOTE) - 1
    if len(report) > room:
        report = report[:max(room, 0)].rsplit("\n", 1)[0] + "\n" + TRUNCATED_NOTE
    return head + report + "\n"


def publish(args, github, git, result, candidate, report):
    """Runs the publish flow. Returns an exit code."""
    branch, base = args.branch, args.base
    heads = git.remote_heads(base, branch)
    if base not in heads:
        raise PublishError(f"the base branch {base} doesn't exist in {args.repo}")
    git.fetch(*heads)
    base_sha = heads[base]
    branch_sha = heads.get(branch)
    pr = github.open_pr(branch, base)

    if branch_sha:
        is_owned, reason = owned(git, base_sha, branch_sha, args.bot_email)
        if not is_owned:
            log(f"paused: {reason}")
            if pr:
                if not any(PAUSE_MARKER in (c.get("body") or "") for c in github.comments(pr["number"])):
                    github.comment(pr["number"], f"{PAUSE_MARKER}\nThe PSS upstream sync bot paused updates to "
                                   f"`{branch}` because {reason}. It won't push to this branch until the extra "
                                   f"commits are gone. The detected upstream head is `{result['head']}`.")
            return EXIT_PAUSED
        # A branch that's already merged restarts from the base. Both are fast-forwards.
        start = base_sha if git.is_ancestor(branch_sha, base_sha) else branch_sha
    else:
        start = base_sha

    original = git.out("show", f"{start}:{PIN_PATH}", check=False) + "\n"
    text = pin_text(original, candidate)
    if git.run("show", f"{start}:{PIN_PATH}", check=False).stdout == text:
        new_sha = None
        log(f"{branch} already pins {result['head'][:12]}")
    else:
        message = pr_title(result) + f"\n\nMove the upstream Pod Security Admission pin to {result['head']}.\n" \
                  f"Classification: {result['classification']}."
        new_sha = build_commit(git, start, text, message, args.bot_name, args.bot_email)
        if new_sha and branch_sha and not git.is_ancestor(branch_sha, new_sha):
            raise PublishError("internal error: the new commit doesn't fast-forward the bot branch")
        if args.dry_run:
            github.planned.append(f"push {new_sha[:12]} to {branch}")
            log(f"dry run: would push {new_sha[:12]} to {branch}")
        else:
            # Never forced: Git rejects the push if the branch moved since it was read.
            git.run("push", "-q", "origin", f"{new_sha}:refs/heads/{branch}")
            log(f"pushed {new_sha[:12]} to {branch}")

    title, body = pr_title(result), pr_body(result, report)
    manual = needs_manual_work(result)
    if pr is None:
        pr = github.create_pr(title, body, branch, base, draft=manual)
        verb = 'would create' if args.dry_run else 'created'
        log(f"{verb} {'a draft' if manual else 'a ready'} PR" + (f" #{pr['number']}" if pr.get('number') else ''))
    else:
        github.update_pr(pr["number"], title, body)
        if manual and not pr.get("draft"):
            github.convert_to_draft(pr["node_id"])
            log(f"converted PR #{pr['number']} to a draft")
        log(f"updated PR #{pr['number']}")
    if manual:
        github.ensure_label()
        if pr.get("number"):
            github.add_label(pr["number"])
        else:
            github.planned.append(f"add the {LABEL} label to the new PR")
    return EXIT_DONE


def load_inputs(result_dir):
    result_dir = Path(result_dir)
    result = json.loads((result_dir / "result.json").read_text(encoding="utf-8"))
    if result.get("classification") not in ("none", "fixtures-only", "manual"):
        raise PublishError(f"unknown classification in result.json: {result.get('classification')}")
    if result["classification"] == "none":
        return result, None, ""
    candidate = verifier.load_pin(yaml.safe_load((result_dir / "upstream.yaml").read_text(encoding="utf-8")) or {})
    if candidate["commit"] != result["head"]:
        raise PublishError("upstream.yaml and result.json name different commits")
    return result, candidate, (result_dir / "report.md").read_text(encoding="utf-8")


def bot_identity(args, github):
    if os.environ.get("BOT_NAME") and os.environ.get("BOT_EMAIL"):
        return os.environ["BOT_NAME"], os.environ["BOT_EMAIL"]
    slug = os.environ.get("APP_SLUG")
    if not slug:
        raise PublishError("set APP_SLUG, or BOT_NAME and BOT_EMAIL, for the bot identity")
    name = f"{slug}[bot]"
    return name, f"{github.bot_user_id(slug)}+{name}@users.noreply.github.com"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--result", required=True, help="output directory of scripts/sync-pss-upstream.py")
    parser.add_argument("--repo", required=True, help="target repository, OWNER/NAME")
    parser.add_argument("--base", default="main", help="base branch for the PR")
    parser.add_argument("--branch", default=BOT_BRANCH, help="bot branch")
    parser.add_argument("--remote-url", help="git remote (default: https://github.com/OWNER/NAME.git)")
    parser.add_argument("--git", default="git", help="path to git")
    parser.add_argument("--dry-run", action="store_true", help="read state and print the plan, write nothing")
    args = parser.parse_args(argv)
    try:
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repo):
            raise PublishError(f"invalid repository: {args.repo}")
        for name in (args.base, args.branch):
            if not re.fullmatch(r"[A-Za-z0-9._/-]+", name) or ".." in name:
                raise PublishError(f"invalid branch name: {name}")
        result, candidate, report = load_inputs(args.result)
        if result["classification"] == "none":
            log("no upstream change, nothing to publish")
            return EXIT_DONE
        github = GitHub(args.repo, dry_run=args.dry_run)
        args.bot_name, args.bot_email = bot_identity(args, github)
        remote = args.remote_url or f"https://github.com/{args.repo}.git"
        with tempfile.TemporaryDirectory(prefix="pss-publish-") as tmp:
            git = Git(args.git, Path(tmp), remote, token=os.environ.get("GH_TOKEN"))
            code = publish(args, github, git, result, candidate, report)
        if args.dry_run:
            print("Planned actions:" if github.planned else "Planned actions: none")
            for action in github.planned:
                print(f"- {action}")
        return code
    except (PublishError, OSError, ValueError, KeyError, yaml.YAMLError) as err:
        print(f"ERROR: {err}")
        return EXIT_ERROR


if __name__ == "__main__":
    sys.exit(main())
