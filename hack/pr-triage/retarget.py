#!/usr/bin/env python3
"""
Publisher for approved legacy PR retargets (main -> release branch).

Consumes one candidate from a prepare-retarget manifest (schema_version 1) and
either previews (default) or publishes it. Preview never calls GitHub, never
creates or mutates the journal, and never writes refs.

Strategies:
  sibling (default) create upstream branch retarget/legacy-<N>-<oldsha12>
                    (create-only lease) and open a new PR against the target
                    branch with an operator-reviewed --body-file. The original
                    PR is never modified or closed.
  author            force-push the prepared commit onto the contributor's head
                    branch (lease on the original head SHA), then change the PR
                    base. Requires maintainerCanModify and a consent comment by
                    the PR author whose first line is exactly
                    `/retarget <target-branch> <original-head-sha>`.

Safety model:
  * --execute requires a valid manifest approval bound to head/target/rebased
    SHAs, --updater-paused operator attestation, pinned source and target branch
    SHAs equal to the live upstream branches, local prepared refs pointing at
    the recorded SHAs, and live PR metadata equal to the manifest.
  * Every remote write is preceded by a write-ahead journal record and followed
    by a completion record. The journal is exclusively flock()ed and replaced
    atomically with file and directory fsync.
  * --resume continues only when the candidate identity hash is unchanged and
    live state is either the original or what this tool wrote; newer work is
    never overwritten. Unknown outcomes (timeouts) are reconciled by readback.
  * --rollback previews by default; with --execute it restores the original
    head (lease on our pushed SHA) then the base (author), or closes only the
    created, unmerged sibling PR (sibling). Branches are never deleted.
  * Residual race: `gh pr edit --base` has no compare-and-swap. PR head/base are
    re-read immediately before and after the edit and a mismatch is reported.
  * No PR code, tests, or hooks are executed (pushes use --no-verify).

Usage:
  retarget.py --manifest PATH --pr N --journal PATH [--execute] [--resume]
              [--rollback] [--updater-paused] [--body-file PATH] [--title T]
              [--git-repo PATH]
"""
import argparse
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

try:
    import fcntl
except ImportError:  # pragma: no cover - non-POSIX
    fcntl = None

SCHEMA_VERSION = 1
JOURNAL_SCHEMA_VERSION = 1
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
REPO_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9._-]+$")
LOGIN_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9-]*(\[bot\])?$")
PREPARED_REF_RE = re.compile(r"^refs/retarget/prepared/([A-Za-z0-9._-]+)/(\d+)/(original|rebased)$")
FEATURE_TITLE_RE = re.compile(r"^\s*feat(ure)?\s*(\(|:|!|/|\[)", re.I)
REMOTE_URL_TEMPLATE = "https://github.com/{repo}.git"
TEMPLATE_PATHS = (
    ".github/pull_request_template.md",
    ".github/PULL_REQUEST_TEMPLATE.md",
    "docs/pull_request_template.md",
    "pull_request_template.md",
)
TEMPLATE_DIR = ".github/PULL_REQUEST_TEMPLATE"
GIT_TIMEOUT = 120
PUSH_TIMEOUT = 300
GH_TIMEOUT = 60
POLL_ATTEMPTS = 6
POLL_DELAY = 5

AUTHOR_FORWARD = ("planned", "push_pending", "pushed", "base_pending", "complete")
SIBLING_FORWARD = ("planned", "branch_pending", "branch_pushed", "pr_pending", "complete")
ROLLBACK_STATES = ("rollback_push_pending", "rollback_pushed", "rollback_base_pending",
                   "rollback_close_pending", "rolled_back")


class RetargetError(Exception):
    pass


def _env():
    env = dict(os.environ)
    env.update({"GIT_TERMINAL_PROMPT": "0", "GH_PROMPT_DISABLED": "1", "GH_NO_UPDATE_NOTIFIER": "1"})
    return env


def run_cmd(cmd, timeout):
    try:
        return subprocess.run(cmd, capture_output=True, text=True, timeout=timeout, check=False, env=_env())
    except subprocess.TimeoutExpired as exc:
        raise RetargetError(
            f"timed out after {timeout}s, outcome unknown; reconcile with --resume/--rollback: {shlex.join(cmd)}"
        ) from exc
    except OSError as exc:
        raise RetargetError(f"cannot run {cmd[0]}: {exc}") from exc


def git(repo_path, args, check=True, timeout=GIT_TIMEOUT):
    res = run_cmd(["git", "-C", str(repo_path)] + list(args), timeout)
    if check and res.returncode != 0:
        raise RetargetError(f"git {shlex.join(args)} failed ({res.returncode}): {res.stderr.strip()}")
    return res


def gh(args, timeout=GH_TIMEOUT):
    res = run_cmd(["gh"] + list(args), timeout)
    if res.returncode != 0:
        raise RetargetError(f"gh {shlex.join(args)} failed ({res.returncode}): {res.stderr.strip()}")
    return res.stdout


def gh_json(args):
    out = gh(args)
    try:
        return json.loads(out)
    except ValueError as exc:
        raise RetargetError(f"gh {shlex.join(args)} returned non-JSON output") from exc


def remote_url(repo):
    return REMOTE_URL_TEMPLATE.format(repo=repo)


def now():
    return datetime.now(timezone.utc).isoformat()


# ---------------------------------------------------------------------------
# Manifest


def _valid_branch(name):
    return (isinstance(name, str) and re.match(r"^[A-Za-z0-9._/-]+$", name) is not None
            and not name.startswith(("-", "/", ".")) and not name.endswith(("/", ".", ".lock"))
            and ".." not in name and "//" not in name and "/." not in name)


def _require(cond, msg):
    if not cond:
        raise RetargetError(f"invalid manifest: {msg}")


def load_manifest(path, number):
    try:
        data = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError) as exc:
        raise RetargetError(f"cannot read manifest {path}: {exc}") from exc
    _require(isinstance(data, dict), "top level must be an object")
    _require(data.get("schema_version") == SCHEMA_VERSION, f"schema_version must be {SCHEMA_VERSION}")
    _require(isinstance(data.get("repository"), str) and REPO_RE.match(data["repository"]), "repository")
    for key in ("source", "target"):
        ref = data.get(key)
        _require(isinstance(ref, dict) and _valid_branch(ref.get("branch")), f"{key}.branch")
        _require(isinstance(ref.get("sha"), str) and SHA_RE.match(ref["sha"]), f"{key}.sha")
    _require(data["source"]["branch"] != data["target"]["branch"], "source and target branch must differ")
    cands = data.get("candidates")
    _require(isinstance(cands, list), "candidates must be a list")
    matches = [c for c in cands if isinstance(c, dict) and c.get("number") == number
               and not isinstance(c.get("number"), bool)]
    _require(len(matches) == 1, f"expected exactly one candidate for PR #{number}, found {len(matches)}")
    cand = matches[0]
    _require(cand.get("base_ref") == data["source"]["branch"], "base_ref must equal source.branch")
    for key in ("head_sha",):
        _require(isinstance(cand.get(key), str) and SHA_RE.match(cand[key]), key)
    for key in ("rebased_sha", "merge_base"):
        _require(cand.get(key) is None or (isinstance(cand[key], str) and SHA_RE.match(cand[key])), key)
    _require(isinstance(cand.get("head_repository"), str) and REPO_RE.match(cand["head_repository"]),
             "head_repository")
    _require(_valid_branch(cand.get("head_ref")), "head_ref")
    _require(isinstance(cand.get("author"), str) and LOGIN_RE.match(cand["author"]), "author")
    _require(isinstance(cand.get("title"), str), "title")
    _require(isinstance(cand.get("updated_at"), str), "updated_at")
    _require(isinstance(cand.get("draft"), bool), "draft must be a boolean")
    for key, kind, required in (("backup_ref", "original", True), ("prepared_ref", "rebased", False)):
        val = cand.get(key)
        if val is None and not required:
            continue
        m = PREPARED_REF_RE.match(val) if isinstance(val, str) else None
        _require(m is not None and int(m.group(2)) == number and m.group(3) == kind, key)
    strategy = cand.get("strategy") or "sibling"
    _require(strategy in ("sibling", "author"), "strategy must be sibling or author")
    cid = cand.get("consent_comment_id")
    _require(cid is None or (isinstance(cid, int) and not isinstance(cid, bool) and cid > 0),
             "consent_comment_id")
    return data, cand


def identity_hash(manifest, cand):
    ident = {k: manifest[k] for k in ("repository", "source", "target")}
    ident["candidate"] = cand
    blob = json.dumps(ident, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(blob).hexdigest()


def strategy_of(cand):
    return cand.get("strategy") or "sibling"


def _evidence_path(manifest_dir, rel):
    if not isinstance(rel, str) or not rel or os.path.isabs(rel):
        return None
    p = (manifest_dir / rel).resolve()
    try:
        p.relative_to(manifest_dir.resolve())
    except ValueError:
        return None
    return p


def approval_blockers(manifest, cand, manifest_dir):
    """Reasons the candidate cannot be published. Empty list means approved."""
    b = []
    if cand.get("recommendation") != "RETARGET":
        b.append(f"recommendation is {cand.get('recommendation')!r}, not 'RETARGET'")
    if cand.get("draft") is not False:
        b.append("draft PRs are never published")
    if cand.get("probe") != "OK":
        b.append(f"probe is {cand.get('probe')!r}, not 'OK'")
    if not cand.get("rebased_sha") or not cand.get("prepared_ref") or not cand.get("merge_base"):
        b.append("rebased_sha, prepared_ref and merge_base must all be recorded")
    if strategy_of(cand) == "author" and cand.get("consent_comment_id") is None:
        b.append("author strategy requires consent_comment_id (author comment "
                 f"`/retarget {manifest['target']['branch']} {cand['head_sha']}`)")
    evidence = cand.get("evidence") if isinstance(cand.get("evidence"), dict) else {}
    for key in ("range_diff", "final_diff"):
        p = _evidence_path(manifest_dir, evidence.get(key))
        if p is None or not p.is_file():
            b.append(f"evidence.{key} missing or not a file inside the manifest directory")
    a = cand.get("approval")
    if not isinstance(a, dict):
        b.append("approval pending (approval is null)")
        return b
    reviewer = a.get("reviewer")
    if not isinstance(reviewer, str) or not LOGIN_RE.match(reviewer):
        b.append("approval.reviewer must be a GitHub login")
    elif reviewer.lower() == cand["author"].lower():
        b.append("approval.reviewer must not be the PR author")
    for key, expected in (("head_sha", cand["head_sha"]), ("target_sha", manifest["target"]["sha"]),
                          ("rebased_sha", cand.get("rebased_sha"))):
        if a.get(key) != expected:
            b.append(f"approval.{key} does not match manifest ({a.get(key)!r} != {expected!r})")
    for key in ("range_diff_reviewed", "final_diff_reviewed"):
        if a.get(key) is not True:
            b.append(f"approval.{key} must be true")
    if not isinstance(a.get("release_feature_approved"), bool):
        b.append("approval.release_feature_approved must be a boolean")
    elif FEATURE_TITLE_RE.match(cand["title"]) and a["release_feature_approved"] is not True:
        b.append("feature PR requires approval.release_feature_approved true")
    tests = a.get("tests")
    if not isinstance(tests, list) or not tests:
        b.append("approval.tests must list at least one human-run test")
    else:
        for i, t in enumerate(tests):
            if not (isinstance(t, dict) and isinstance(t.get("command"), str) and t["command"].strip()
                    and t.get("result") == "passed"):
                b.append(f"approval.tests[{i}] must have a command and result 'passed'")
    return b


# ---------------------------------------------------------------------------
# Journal


class Journal:
    def __init__(self, path):
        self.path = Path(path)
        self.data = None
        self._lock_fd = None

    def read(self):
        if not self.path.exists():
            return None
        try:
            data = json.loads(self.path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            raise RetargetError(f"cannot read journal {self.path}: {exc}") from exc
        if not isinstance(data, dict) or data.get("schema_version") != JOURNAL_SCHEMA_VERSION:
            raise RetargetError(f"journal {self.path} has unexpected format")
        self.data = data
        return data

    def intended(self, *states):
        """True when any of the given write-ahead states was ever recorded."""
        return bool(self.data) and any(e.get("state") in states for e in self.data.get("events", []))

    @property
    def state(self):
        return self.data["state"] if self.data else None

    def lock(self):
        if fcntl is None:
            raise RetargetError("exclusive journal locking requires a POSIX platform (fcntl)")
        lock_path = str(self.path) + ".lock"
        fd = os.open(lock_path, os.O_RDWR | os.O_CREAT, 0o600)
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            os.close(fd)
            raise RetargetError(f"journal {self.path} is locked by another process") from exc
        self._lock_fd = fd

    def unlock(self):
        if self._lock_fd is not None:
            fcntl.flock(self._lock_fd, fcntl.LOCK_UN)
            os.close(self._lock_fd)
            self._lock_fd = None

    def create(self, header):
        self.data = dict(header, schema_version=JOURNAL_SCHEMA_VERSION, state=None, events=[])
        self.record("planned")

    def record(self, state, **details):
        if self._lock_fd is None:
            raise RetargetError("internal error: journal write without lock")
        self.data["state"] = state
        self.data.update(details)
        self.data["events"].append({"at": now(), "state": state, **details})
        self._write()
        print(f"journal: {state}")

    def _write(self):
        tmp = self.path.with_name(f".{self.path.name}.tmp.{os.getpid()}")
        payload = (json.dumps(self.data, indent=2, sort_keys=True) + "\n").encode()
        fd = os.open(str(tmp), os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        with os.fdopen(fd, "wb") as f:
            f.write(payload)
            f.flush()
            os.fsync(f.fileno())
        os.replace(str(tmp), str(self.path))
        dfd = os.open(str(self.path.parent), os.O_RDONLY)
        try:
            os.fsync(dfd)
        finally:
            os.close(dfd)


# ---------------------------------------------------------------------------
# Live state helpers


class Ctx:
    def __init__(self, manifest, cand, journal, git_repo, args):
        self.repo = manifest["repository"]
        self.source = manifest["source"]
        self.target = manifest["target"]
        self.cand = cand
        self.number = cand["number"]
        self.head = cand["head_sha"]
        self.rebased = cand.get("rebased_sha")
        self.journal = journal
        self.git_repo = git_repo
        self.args = args

    @property
    def sibling_branch(self):
        return f"retarget/legacy-{self.number}-{self.head[:12]}"


def ls_remote(ctx, repo, branch):
    ref = f"refs/heads/{branch}"
    out = git(ctx.git_repo, ["ls-remote", "--refs", remote_url(repo), ref], timeout=GH_TIMEOUT).stdout
    shas = [line.split("\t")[0] for line in out.splitlines() if line.split("\t")[-1] == ref]
    if len(shas) > 1:
        raise RetargetError(f"ambiguous ls-remote output for {repo} {ref}")
    return shas[0] if shas else None


def rev_parse(ctx, ref):
    res = git(ctx.git_repo, ["rev-parse", "--verify", "--quiet", "--end-of-options", f"{ref}^{{commit}}"],
              check=False)
    return res.stdout.strip() if res.returncode == 0 else None


def verify_local(ctx):
    for sha in (ctx.source["sha"], ctx.target["sha"], ctx.head, ctx.rebased):
        if rev_parse(ctx, sha) != sha:
            raise RetargetError(f"commit {sha} is not present in local repository {ctx.git_repo}")
    for key, expected in (("backup_ref", ctx.head), ("prepared_ref", ctx.rebased)):
        got = rev_parse(ctx, ctx.cand[key])
        if got != expected:
            raise RetargetError(f"{key} {ctx.cand[key]} resolves to {got}, expected {expected}")
    mb = git(ctx.git_repo, ["merge-base", ctx.head, ctx.source["sha"]]).stdout.strip()
    if mb != ctx.cand["merge_base"]:
        raise RetargetError(f"merge-base(head, source) is {mb}, manifest records {ctx.cand['merge_base']}")
    anc = git(ctx.git_repo, ["merge-base", "--is-ancestor", ctx.target["sha"], ctx.rebased], check=False)
    if anc.returncode != 0 or ctx.rebased == ctx.target["sha"]:
        raise RetargetError("rebased_sha is not a non-empty descendant of target.sha")


def verify_pins(ctx):
    for name, ref in (("source", ctx.source), ("target", ctx.target)):
        live = ls_remote(ctx, ctx.repo, ref["branch"])
        if live != ref["sha"]:
            raise RetargetError(f"{name} branch {ref['branch']} drifted: live {live}, pinned {ref['sha']}; "
                                "regenerate and re-approve the manifest")


def read_pr(ctx, number):
    pr = gh_json(["api", f"repos/{ctx.repo}/pulls/{number}"])
    if not isinstance(pr, dict):
        raise RetargetError(f"unexpected API response for PR #{number}")
    return pr


def _pr_head_repo(pr):
    repo = (pr.get("head") or {}).get("repo")
    return repo.get("full_name") if isinstance(repo, dict) else None


def check_original(ctx, pr, heads, bases, check_updated):
    c = ctx.cand
    problems = []
    if pr.get("merged") or pr.get("state") != "open":
        problems.append(f"state={pr.get('state')} merged={pr.get('merged')}")
    if ((pr.get("base") or {}).get("repo") or {}).get("full_name") != ctx.repo:
        problems.append("base repository mismatch")
    if _pr_head_repo(pr) != c["head_repository"]:
        problems.append(f"head repository {_pr_head_repo(pr)} != {c['head_repository']}")
    if (pr.get("head") or {}).get("ref") != c["head_ref"]:
        problems.append(f"head ref {(pr.get('head') or {}).get('ref')} != {c['head_ref']}")
    if (pr.get("user") or {}).get("login") != c["author"]:
        problems.append("author mismatch")
    if (pr.get("head") or {}).get("sha") not in heads:
        problems.append(f"head {(pr.get('head') or {}).get('sha')} not in expected {sorted(heads)}")
    if (pr.get("base") or {}).get("ref") not in bases:
        problems.append(f"base {(pr.get('base') or {}).get('ref')} not in expected {sorted(bases)}")
    if check_updated and pr.get("draft") is not c.get("draft"):
        problems.append(f"draft {pr.get('draft')} != manifest {c.get('draft')} (stale metadata)")
    if check_updated and pr.get("updated_at") != c["updated_at"]:
        problems.append(f"updated_at {pr.get('updated_at')} != manifest {c['updated_at']} (stale metadata)")
    if problems:
        raise RetargetError(f"PR #{ctx.number} live state does not match: " + "; ".join(problems))


def poll_pr(ctx, number, predicate, what):
    pr = None
    for attempt in range(POLL_ATTEMPTS):
        pr = read_pr(ctx, number)
        if predicate(pr):
            return pr
        if attempt + 1 < POLL_ATTEMPTS:
            time.sleep(POLL_DELAY)
    raise RetargetError(f"PR #{number} did not reach expected state ({what}); re-run with --resume")


def push(ctx, repo, branch, expected, src):
    lease = f"--force-with-lease=refs/heads/{branch}:{expected or ''}"
    git(ctx.git_repo, ["push", "--no-verify", "--porcelain", lease, remote_url(repo),
                       f"{src}:refs/heads/{branch}"], timeout=PUSH_TIMEOUT)
    live = ls_remote(ctx, repo, branch)
    if live != src:
        raise RetargetError(f"after push {repo}:{branch} is {live}, expected {src}")


def check_consent(ctx, pr):
    c = ctx.cand
    if pr.get("maintainer_can_modify") is not True and _pr_head_repo(pr) != ctx.repo:
        raise RetargetError("author strategy requires maintainerCanModify=true on the PR; use sibling instead")
    comment = gh_json(["api", f"repos/{ctx.repo}/issues/comments/{c['consent_comment_id']}"])
    expected = f"/retarget {ctx.target['branch']} {ctx.head}"
    first = (comment.get("body") or "").replace("\r\n", "\n").split("\n", 1)[0].strip()
    if not str(comment.get("issue_url", "")).endswith(f"/repos/{ctx.repo}/issues/{ctx.number}"):
        raise RetargetError("consent comment does not belong to this PR")
    if (comment.get("user") or {}).get("login") != c["author"]:
        raise RetargetError("consent comment was not written by the PR author")
    if first != expected:
        raise RetargetError(f"consent comment first line must be exactly `{expected}` (stale or unknown "
                            "consent; strategy is not downgraded automatically)")


# ---------------------------------------------------------------------------
# Sibling PR body


def _strip_non_prose(text):
    text = re.sub(r"<!--.*?-->", "", text, flags=re.S)
    text = re.sub(r"^```.*?^```", "", text, flags=re.S | re.M)
    return text


def template_headings(text):
    return [line.strip() for line in _strip_non_prose(text).splitlines() if re.match(r"^#{1,6}\s+\S", line)]


def load_templates(ctx):
    sha = ctx.source["sha"]
    templates = []
    for p in TEMPLATE_PATHS:
        res = git(ctx.git_repo, ["cat-file", "-p", f"{sha}:{p}"], check=False)
        if res.returncode == 0:
            templates.append(res.stdout)
    res = git(ctx.git_repo, ["ls-tree", "--name-only", sha, "--", TEMPLATE_DIR + "/"], check=False)
    for p in res.stdout.splitlines():
        if p.lower().endswith(".md"):
            templates.append(git(ctx.git_repo, ["cat-file", "-p", f"{sha}:{p}"]).stdout)
    return templates


def validate_body(ctx, body_file):
    if not body_file:
        raise RetargetError("sibling strategy requires --body-file with the reviewed, filled-in PR template")
    try:
        body = Path(body_file).read_text(encoding="utf-8")
    except OSError as exc:
        raise RetargetError(f"cannot read --body-file: {exc}") from exc
    link = re.compile(rf"https://github\.com/{re.escape(ctx.repo)}/pull/{ctx.number}(?!\d)")
    if not link.search(body):
        raise RetargetError(f"--body-file must link the original PR https://github.com/{ctx.repo}/pull/{ctx.number}")
    templates = load_templates(ctx)
    if templates:
        lines = {line.strip() for line in body.splitlines()}
        if not any(all(h in lines for h in template_headings(t)) for t in templates):
            raise RetargetError("--body-file does not contain every heading of the repository PR template")
        if any(body.strip() == t.strip() for t in templates):
            raise RetargetError("--body-file is the unfilled PR template")
    return body, hashlib.sha256(body.encode()).hexdigest()


def find_siblings(ctx):
    rows = gh_json(["pr", "list", "--repo", ctx.repo, "--head", ctx.sibling_branch, "--base",
                    ctx.target["branch"], "--state", "all", "--json",
                    "number,url,state,headRefName,headRefOid,baseRefName,isCrossRepository"])
    return [r for r in rows if r.get("headRefName") == ctx.sibling_branch and not r.get("isCrossRepository")]


# ---------------------------------------------------------------------------
# Forward execution


def author_forward(ctx):
    j, src, tgt = ctx.journal, ctx.source["branch"], ctx.target["branch"]
    c = ctx.cand
    verify_local(ctx)
    verify_pins(ctx)
    pr = read_pr(ctx, ctx.number)
    initial = j.state in (None, "planned")
    check_original(ctx, pr, {ctx.head, ctx.rebased}, {src, tgt}, check_updated=initial)
    check_consent(ctx, pr)
    live_head, live_base = pr["head"]["sha"], pr["base"]["ref"]
    if live_head == ctx.rebased and j.state not in ("push_pending", "pushed", "base_pending", "complete"):
        raise RetargetError("PR head already equals rebased_sha but the journal never recorded our push")
    if live_base == tgt and j.state not in ("base_pending", "complete"):
        raise RetargetError(f"PR base already {tgt} but the journal never recorded our base edit")
    remote = ls_remote(ctx, c["head_repository"], c["head_ref"])
    if j.state == "complete":
        if remote != ctx.rebased or live_head != ctx.rebased or live_base != tgt:
            raise RetargetError(f"journal is complete but live head={live_head} branch={remote} base={live_base} "
                                "differ from what was published; refusing to re-apply")
        print(f"PR #{ctx.number} already complete; nothing to do")
        return
    if remote == ctx.rebased and not j.intended("push_pending"):
        raise RetargetError("head branch already equals rebased_sha but the journal never recorded push intent")
    if j.state is None:
        j.create(journal_header(ctx))

    if remote == ctx.head:
        if j.state in ("pushed", "base_pending", "complete"):
            raise RetargetError("head branch was reset to the original after our push; refusing")
        j.record("push_pending", push={"repo": c["head_repository"], "ref": c["head_ref"],
                                       "lease": ctx.head, "new": ctx.rebased})
        push(ctx, c["head_repository"], c["head_ref"], ctx.head, ctx.rebased)
        j.record("pushed")
    elif remote == ctx.rebased:
        if j.state == "push_pending":
            j.record("pushed", reconciled=True)
    else:
        raise RetargetError(f"head branch is at {remote}, neither original nor ours; refusing to overwrite")

    pr = poll_pr(ctx, ctx.number, lambda p: (p.get("head") or {}).get("sha") == ctx.rebased, "head == rebased")
    check_original(ctx, pr, {ctx.rebased}, {src, tgt}, check_updated=False)
    if pr["base"]["ref"] == src:
        if j.state != "base_pending":
            j.record("base_pending", base={"from": src, "to": tgt})
        gh(["pr", "edit", str(ctx.number), "--repo", ctx.repo, "--base", tgt])
    elif not j.intended("base_pending"):
        raise RetargetError(f"PR base is already {tgt} but the journal never recorded base intent")
    pr = poll_pr(ctx, ctx.number, lambda p: (p.get("base") or {}).get("ref") == tgt, f"base == {tgt}")
    check_original(ctx, pr, {ctx.rebased}, {tgt}, check_updated=False)
    if j.state != "complete":
        j.record("complete")
    print(f"PR #{ctx.number} now targets {tgt} at {ctx.rebased}")


def sibling_forward(ctx):
    j, src, tgt, branch = ctx.journal, ctx.source["branch"], ctx.target["branch"], ctx.sibling_branch
    body_hash = None
    if j.state != "complete":
        _, body_hash = validate_body(ctx, ctx.args.body_file)
    verify_local(ctx)
    verify_pins(ctx)
    pr = read_pr(ctx, ctx.number)
    check_original(ctx, pr, {ctx.head}, {src}, check_updated=j.state in (None, "planned"))
    remote = ls_remote(ctx, ctx.repo, branch)
    if remote is not None and j.state not in ("branch_pending", "branch_pushed", "pr_pending", "complete"):
        raise RetargetError(f"upstream branch {branch} already exists but the journal never recorded creating it")
    if j.state is None:
        j.create(journal_header(ctx))

    if remote is None:
        if j.state in ("branch_pushed", "pr_pending", "complete"):
            raise RetargetError(f"upstream branch {branch} disappeared after creation; refusing")
        j.record("branch_pending", sibling_branch=branch)
        push(ctx, ctx.repo, branch, None, ctx.rebased)
        j.record("branch_pushed")
    elif remote != ctx.rebased:
        raise RetargetError(f"upstream branch {branch} is at {remote}, not prepared {ctx.rebased}; refusing")
    elif j.state == "branch_pending":
        j.record("branch_pushed", reconciled=True)

    title = ctx.args.title or f"[{tgt}] {ctx.cand['title']}"
    if j.state == "pr_pending" and (j.data.get("sibling_title") != title or j.data.get("body_sha256") != body_hash):
        raise RetargetError("sibling title/--body-file differ from the recorded pr_pending intent; refusing")
    found = find_siblings(ctx)
    if found and not j.intended("pr_pending"):
        raise RetargetError(f"sibling PR(s) {[r.get('number') for r in found]} exist for {branch} "
                            "but the journal never recorded creating one; refusing to adopt")
    if j.state == "complete" and [r.get("number") for r in found] != [j.data.get("sibling_pr")]:
        raise RetargetError(f"journal records sibling PR #{j.data.get('sibling_pr')} but found "
                            f"{[r.get('number') for r in found]}")
    if not found:
        if j.state == "complete":
            raise RetargetError("journal says complete but no sibling PR found")
        j.record("pr_pending", sibling_title=title, body_sha256=body_hash)
        gh(["pr", "create", "--repo", ctx.repo, "--base", tgt, "--head", branch, "--title", title,
            "--body-file", str(ctx.args.body_file)])
        found = find_siblings(ctx)
    if len(found) != 1:
        raise RetargetError(f"expected exactly one sibling PR for {branch}, found {len(found)}")
    sib = found[0]
    if sib.get("headRefOid") != ctx.rebased or sib.get("baseRefName") != tgt or sib.get("state") == "CLOSED":
        raise RetargetError(f"sibling PR #{sib.get('number')} does not match (head/base/state); refusing")
    if j.state != "complete":
        j.record("complete", sibling_pr=sib["number"], sibling_url=sib.get("url"))
    print(f"sibling PR #{sib['number']} ({sib.get('url')}) targets {tgt}; original #{ctx.number} untouched")


def journal_header(ctx):
    return {"repository": ctx.repo, "pr": ctx.number, "strategy": strategy_of(ctx.cand),
            "identity_hash": ctx.identity, "manifest": str(ctx.args.manifest),
            "head_sha": ctx.head, "rebased_sha": ctx.rebased, "target": ctx.target, "source": ctx.source}


# ---------------------------------------------------------------------------
# Rollback


def author_rollback(ctx):
    j, src, tgt, c = ctx.journal, ctx.source["branch"], ctx.target["branch"], ctx.cand
    pr = read_pr(ctx, ctx.number)
    check_original(ctx, pr, {ctx.head, ctx.rebased}, {src, tgt}, check_updated=False)
    if pr["base"]["ref"] == tgt and not j.intended("base_pending"):
        raise RetargetError(f"PR base is {tgt} but this tool never recorded changing it; refusing rollback")
    remote = ls_remote(ctx, c["head_repository"], c["head_ref"])
    if remote == ctx.rebased and not j.intended("push_pending"):
        raise RetargetError("head branch is at rebased_sha but this tool never recorded pushing it; refusing")
    if remote == ctx.rebased:
        if j.state != "rollback_push_pending":
            j.record("rollback_push_pending", rollback_push={"lease": ctx.rebased, "new": ctx.head})
        push(ctx, c["head_repository"], c["head_ref"], ctx.rebased, ctx.head)
        j.record("rollback_pushed")
    elif remote == ctx.head:
        if j.state == "rollback_push_pending":
            j.record("rollback_pushed", reconciled=True)
    else:
        raise RetargetError(f"head branch is at {remote}, neither ours nor original; refusing rollback")
    pr = poll_pr(ctx, ctx.number, lambda p: (p.get("head") or {}).get("sha") == ctx.head, "head == original")
    check_original(ctx, pr, {ctx.head}, {src, tgt}, check_updated=False)
    if pr["base"]["ref"] == tgt:
        if j.state != "rollback_base_pending":
            j.record("rollback_base_pending")
        gh(["pr", "edit", str(ctx.number), "--repo", ctx.repo, "--base", src])
        pr = poll_pr(ctx, ctx.number, lambda p: (p.get("base") or {}).get("ref") == src, f"base == {src}")
        check_original(ctx, pr, {ctx.head}, {src}, check_updated=False)
    j.record("rolled_back")


def sibling_rollback(ctx):
    j, tgt, branch = ctx.journal, ctx.target["branch"], ctx.sibling_branch
    pr = read_pr(ctx, ctx.number)
    check_original(ctx, pr, {ctx.head}, {ctx.source["branch"]}, check_updated=False)
    if not j.intended("pr_pending"):
        j.record("rolled_back", note=f"no sibling PR was requested; branch {branch} left for manual cleanup")
        return
    number = j.data.get("sibling_pr")
    if number is None:
        found = find_siblings(ctx)
        if len(found) > 1:
            raise RetargetError(f"multiple sibling PRs for {branch}; resolve manually")
        number = found[0]["number"] if found else None
    if number is not None:
        sib = read_pr(ctx, number)
        if sib.get("merged"):
            raise RetargetError(f"sibling PR #{number} is merged; refusing rollback")
        if (_pr_head_repo(sib) != ctx.repo or sib["head"].get("ref") != branch
                or sib["head"].get("sha") != ctx.rebased or sib["base"].get("ref") != tgt):
            raise RetargetError(f"sibling PR #{number} head/base changed; refusing to close")
        if sib.get("state") == "open":
            live = ls_remote(ctx, ctx.repo, branch)
            if live != ctx.rebased:
                raise RetargetError(f"sibling branch {branch} is at {live}, not {ctx.rebased}; refusing to close")
            j.record("rollback_close_pending", sibling_pr=number)
            gh(["pr", "close", str(number), "--repo", ctx.repo])
            poll_pr(ctx, number, lambda p: p.get("state") == "closed" and not p.get("merged"), "closed")
    j.record("rolled_back", note=f"branch {branch} left in place for operator cleanup")


# ---------------------------------------------------------------------------
# Preview


def planned_commands(ctx, rollback):
    c, src, tgt = ctx.cand, ctx.source["branch"], ctx.target["branch"]
    rb = ctx.rebased or "<rebased_sha>"
    if strategy_of(c) == "author":
        url, ref = remote_url(c["head_repository"]), c["head_ref"]
        if rollback:
            return [["git", "push", "--no-verify", "--porcelain", f"--force-with-lease=refs/heads/{ref}:{rb}",
                     url, f"{ctx.head}:refs/heads/{ref}"],
                    ["gh", "pr", "edit", str(ctx.number), "--repo", ctx.repo, "--base", src]]
        return [["git", "push", "--no-verify", "--porcelain", f"--force-with-lease=refs/heads/{ref}:{ctx.head}",
                 url, f"{rb}:refs/heads/{ref}"],
                ["gh", "pr", "edit", str(ctx.number), "--repo", ctx.repo, "--base", tgt]]
    branch = ctx.sibling_branch
    if rollback:
        return [["gh", "pr", "close", "<sibling-pr>", "--repo", ctx.repo]]
    return [["git", "push", "--no-verify", "--porcelain", f"--force-with-lease=refs/heads/{branch}:",
             remote_url(ctx.repo), f"{rb}:refs/heads/{branch}"],
            ["gh", "pr", "create", "--repo", ctx.repo, "--base", tgt, "--head", branch, "--title",
             ctx.args.title or f"[{tgt}] {c['title']}", "--body-file", str(ctx.args.body_file or "<body-file>")]]


def preview(ctx, blockers, journal_data):
    mode = "rollback" if ctx.args.rollback else ("resume" if ctx.args.resume else "publish")
    print(f"DRY RUN ({mode}) PR #{ctx.number} strategy={strategy_of(ctx.cand)} "
          f"{ctx.source['branch']}@{ctx.source['sha'][:12]} -> {ctx.target['branch']}@{ctx.target['sha'][:12]}")
    print(f"journal: {journal_data['state'] if journal_data else 'absent'} ({ctx.journal.path})")
    if blockers and not ctx.args.rollback:
        print("STATUS: BLOCKED")
        for b in blockers:
            print(f"  - {b}")
    else:
        print("STATUS: READY (live checks run only with --execute --updater-paused)")
    print("intended commands:")
    for cmd in planned_commands(ctx, ctx.args.rollback):
        print("  " + shlex.join(cmd))


# ---------------------------------------------------------------------------


def parse_args(argv):
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("--manifest", required=True, type=Path)
    p.add_argument("--pr", required=True, type=int)
    p.add_argument("--journal", required=True, type=Path)
    p.add_argument("--execute", action="store_true", help="perform GitHub writes (default: preview only)")
    p.add_argument("--resume", action="store_true", help="continue an existing journal")
    p.add_argument("--rollback", action="store_true", help="undo a journaled publish")
    p.add_argument("--updater-paused", action="store_true",
                   help="operator attests branch auto-update automation is paused for this PR")
    p.add_argument("--body-file", type=Path, help="reviewed sibling PR body following the repo PR template")
    p.add_argument("--title", help="sibling PR title (default: '[<target>] <original title>')")
    p.add_argument("--git-repo", type=Path, default=Path("."), help="local clone holding prepared refs")
    a = p.parse_args(argv)
    if a.resume and a.rollback:
        p.error("--resume and --rollback are mutually exclusive")
    return a


def main(argv=None):
    args = parse_args(argv)
    try:
        manifest, cand = load_manifest(args.manifest, args.pr)
        journal = Journal(args.journal)
        ctx = Ctx(manifest, cand, journal, args.git_repo, args)
        ctx.identity = identity_hash(manifest, cand)
        blockers = approval_blockers(manifest, cand, args.manifest.resolve().parent)
        existing = journal.read()
        if existing and (existing.get("identity_hash") != ctx.identity or existing.get("pr") != ctx.number
                         or existing.get("repository") != ctx.repo):
            raise RetargetError("journal does not match this manifest candidate (identity hash changed); refusing")
        if not args.execute:
            preview(ctx, blockers, existing)
            return 0
        if not args.updater_paused:
            raise RetargetError("--execute requires --updater-paused (attest PR auto-update automation is paused)")
        if blockers and not args.rollback:
            print("BLOCKED:\n" + "\n".join(f"  - {b}" for b in blockers), file=sys.stderr)
            return 2
        journal.lock()
        try:
            existing = journal.read()
            if existing and existing.get("identity_hash") != ctx.identity:
                raise RetargetError("journal identity changed while waiting for lock")
            strategy = strategy_of(cand)
            if args.rollback:
                if not existing:
                    raise RetargetError("nothing to roll back: journal does not exist")
                if existing["state"] == "rolled_back":
                    print("already rolled back")
                elif existing["state"] == "planned":
                    journal.record("rolled_back", note="no remote writes were recorded")
                else:
                    (author_rollback if strategy == "author" else sibling_rollback)(ctx)
                return 0
            if existing and not args.resume:
                raise RetargetError(f"journal exists (state={existing['state']}); use --resume or --rollback")
            if args.resume and not existing:
                raise RetargetError("--resume requires an existing journal")
            if existing and existing["state"] in ROLLBACK_STATES:
                raise RetargetError(f"journal is in rollback state {existing['state']}; refusing forward resume")
            (author_forward if strategy == "author" else sibling_forward)(ctx)
            return 0
        finally:
            journal.unlock()
    except RetargetError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
