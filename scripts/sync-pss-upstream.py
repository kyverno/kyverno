#!/usr/bin/env python3
"""Detects upstream Pod Security Admission (PSA) changes since the pinned commit.

Compares the commit pinned in scripts/pss-upstream/upstream.yaml with the head
of the pinned branch. Only non-test files in policy/ and fixtures in
test/testdata/ count as relevant. A relevant change is classified as:

- fixtures-only: only test/testdata/ changed, for example a new v1.<N> directory.
- manual: a policy/*.go file changed, so a person must review the Go change and
  update the CEL policies. Files that belong to no single check, such as
  policy/helpers.go, are grouped as "shared".

The script writes the candidate pin (upstream.yaml), a Markdown report
(report.md), and a machine-readable result (result.json) to --out. With --preview, it also runs
scripts/verify-policies-pss-fixtures.py against the candidate commit and adds
the result to the report. This is a dry run: it never pushes or opens a PR.

Usage: scripts/sync-pss-upstream.py --out DIR [--pin FILE] [--repository URL]
                                    [--branch NAME] [--preview --cli PATH]
                                    [--helm PATH] [--jobs N]

Exit codes:
  0  no relevant change since the pin
  2  relevant change; the candidate passes the fixture check, or --preview
     wasn't set
  3  relevant change; the candidate fails the fixture check
  1  error, for example a failed fetch, an invalid pin, or a verifier that
     didn't complete
"""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import sys
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[1]
DATA_DIR = ROOT / "scripts/pss-upstream"
VERIFIER = ROOT / "scripts/verify-policies-pss-fixtures.py"

# Reuse the verifier's pin validation and command runner.
sys.dont_write_bytecode = True
_spec = importlib.util.spec_from_file_location("verify_pss_fixtures", VERIFIER)
verifier = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(verifier)
VerifyError = verifier.VerifyError

RELEVANT_PATHS = ["policy", "test/testdata", ":!policy/*_test.go"]
SUMMARY_RE = re.compile(
    r"^checked (\d+) fixture results across (\d+) renders: (\d+) unexpected, (\d+) known, (\d+) stale$",
    re.MULTILINE,
)
EXIT_NO_CHANGE, EXIT_ERROR, EXIT_CHANGE, EXIT_MISMATCH = 0, 1, 2, 3


def log(message):
    print(f"[sync-pss-upstream] {message}", file=sys.stderr)


def git(git_bin, repo, *args):
    """Runs git in repo and returns stdout, or raises VerifyError."""
    result = verifier.run_command([git_bin, "-C", str(repo), *args])
    if result.returncode != 0:
        raise VerifyError(f"git {' '.join(args[:3])} failed: {result.stderr.strip()[-500:]}")
    return result.stdout


def fetch(git_bin, repository, branch, pin_commit, dest):
    """Fetches the branch, its tags, and the pinned commit into a blobless clone.

    Returns the full SHA of the branch head.
    """
    dest.mkdir(parents=True)
    git(git_bin, dest, "init", "-q")
    git(git_bin, dest, "remote", "add", "origin", repository)
    git(git_bin, dest, "fetch", "-q", "--filter=blob:none", "--tags", "origin",
        f"+refs/heads/{branch}:refs/remotes/origin/{branch}")
    head = git(git_bin, dest, "rev-parse", f"refs/remotes/origin/{branch}^{{commit}}").strip()
    # The pin is normally reachable from the branch. Fetch it directly if not.
    if verifier.run_command([git_bin, "-C", str(dest), "cat-file", "-e", f"{pin_commit}^{{commit}}"]).returncode != 0:
        result = verifier.run_command([git_bin, "-C", str(dest), "fetch", "-q", "--filter=blob:none",
                                       "origin", pin_commit])
        if result.returncode != 0:
            raise VerifyError(f"pinned commit {pin_commit} isn't in {repository}")
    return head


def check_id_of(path, content):
    """Returns the check ID for a policy/ file, or "shared".

    A check file sets its ID either inline (ID: "sysctls") or through a
    CheckID constant. Other files, such as helpers.go, are shared.
    """
    name = path.rsplit("/", 1)[-1]
    if not name.startswith("check_"):
        return "shared"
    match = re.search(r'\bID:\s*"([^"]+)"', content) or re.search(r'CheckID\s*=\s*"([^"]+)"', content)
    return match.group(1) if match else name[len("check_"):-len(".go")]


def fixture_versions(git_bin, repo, commit):
    """Returns the set of (level, "v1.<N>") fixture directories at a commit."""
    versions = set()
    for level in ("baseline", "restricted"):
        out = verifier.run_command([git_bin, "-C", str(repo), "ls-tree", "-d", "--name-only",
                                    f"{commit}:test/testdata/{level}"])
        if out.returncode == 0:
            versions.update((level, v) for v in out.stdout.split() if re.fullmatch(r"v1\.\d+", v))
    return versions


def first_release(git_bin, repo, commit):
    """Returns the earliest kubernetes-* tag that contains commit, by date."""
    tags = git(git_bin, repo, "tag", "--contains", commit, "--sort=creatordate", "--list", "kubernetes-*").split()
    return tags[0] if tags else "unreleased"


def relevant_commits(git_bin, repo, pin, head):
    """Returns [(sha, subject, kubernetes_commit or None)] for relevant non-merge commits."""
    out = git(git_bin, repo, "log", "--no-merges", "--format=%H%x1f%s%x1f%b%x1e", f"{pin}..{head}",
              "--", *RELEVANT_PATHS)
    commits = []
    for record in out.split("\x1e"):
        if not record.strip():
            continue
        sha, subject, body = (record.strip("\n").split("\x1f") + ["", ""])[:3]
        trailer = re.search(r"^Kubernetes-commit:\s*([0-9a-f]{40})\s*$", body, re.MULTILINE)
        commits.append((sha, subject, trailer.group(1) if trailer else None))
    return commits


def detect(git_bin, repository, branch, pin_commit, work):
    """Compares the pin with the branch head and returns a result dict."""
    repo = work / "upstream"
    head = fetch(git_bin, repository, branch, pin_commit, repo)
    result = {"repository": repository, "branch": branch, "pin": pin_commit, "head": head,
              "classification": "none", "files": [], "groups": {}, "new_versions": [],
              "commits": [], "go_diff": ""}
    if head == pin_commit:
        return result
    if verifier.run_command([git_bin, "-C", str(repo), "merge-base", "--is-ancestor",
                             pin_commit, head]).returncode != 0:
        raise VerifyError(f"pinned commit {pin_commit} isn't an ancestor of {branch} ({head}); "
                          "the branch history was rewritten, so update the pin by hand")
    files = git(git_bin, repo, "diff", "--name-only", pin_commit, head, "--", *RELEVANT_PATHS).split()
    result["files"] = files
    if not files:
        return result

    groups = {}
    for path in files:
        if path.startswith("policy/"):
            show = verifier.run_command([git_bin, "-C", str(repo), "show", f"{head}:{path}"])
            if show.returncode != 0:
                show = verifier.run_command([git_bin, "-C", str(repo), "show", f"{pin_commit}:{path}"])
            group = check_id_of(path, show.stdout if show.returncode == 0 else "")
        else:
            group = "fixtures"
        groups.setdefault(group, []).append(path)
    result["groups"] = groups
    result["classification"] = "manual" if any(p.startswith("policy/") for p in files) else "fixtures-only"
    new = fixture_versions(git_bin, repo, head) - fixture_versions(git_bin, repo, pin_commit)
    result["new_versions"] = sorted(f"{level}/{version}" for level, version in new)
    result["commits"] = [(sha, subject, k8s, first_release(git_bin, repo, sha))
                         for sha, subject, k8s in relevant_commits(git_bin, repo, pin_commit, head)]
    if result["classification"] == "manual":
        result["go_diff"] = git(git_bin, repo, "diff", pin_commit, head, "--", "policy", ":!policy/*_test.go")
    return result


def run_preview(args, candidate_pin):
    """Runs the fixture check against the candidate pin.

    Returns (status, summary, output). status is "pass" or "fail" when the
    check completed, and raises VerifyError when it didn't: the verifier exits
    with 1 for both a mismatch and an error, so only its summary line tells
    them apart.
    """
    command = [sys.executable, str(VERIFIER), "--cli", args.cli, "--helm", args.helm,
               "--pin", str(candidate_pin), "--jobs", str(args.jobs)]
    if args.mapping:
        command += ["--mapping", args.mapping]
    proc = verifier.run_command(command)
    output = (proc.stdout + proc.stderr).strip()
    match = SUMMARY_RE.search(proc.stdout)
    if not match:
        raise VerifyError(f"the fixture check didn't complete (exit code {proc.returncode}):\n{output[-1500:]}")
    unexpected, stale = int(match.group(3)), int(match.group(5))
    status = "pass" if proc.returncode == 0 and unexpected == 0 and stale == 0 else "fail"
    return status, match.group(0), output


def short(sha):
    return sha[:12]


def commit_link(repository, sha):
    base = repository[:-4] if repository.endswith(".git") else repository
    return f"[`{short(sha)}`]({base}/commit/{sha})"


def render_report(result, preview):
    """Returns the Markdown report for a detection result."""
    lines = ["# Upstream PSA sync report (dry run)", ""]
    lines.append(f"- Repository: {result['repository']} (`{result['branch']}`)")
    lines.append(f"- Pinned commit: {commit_link(result['repository'], result['pin'])}")
    lines.append(f"- Branch head: {commit_link(result['repository'], result['head'])}")
    lines.append(f"- Classification: **{result['classification']}**")
    lines.append("")
    if result["classification"] == "none":
        if result["head"] == result["pin"]:
            lines.append("The branch head is the pinned commit. Nothing to do.")
        else:
            lines.append("The branch moved, but no non-test file in `policy/` and no fixture in `test/testdata/` "
                         "changed. Nothing to do.")
        return "\n".join(lines) + "\n"

    if preview is not None:
        status, summary, output = preview
        verdict = "passes" if status == "pass" else "**fails**"
        lines += ["## Fixture check on the candidate commit", "",
                  f"The candidate {verdict} the fixture check: `{summary}`", ""]
        if status == "fail":
            failures = [l for l in output.splitlines() if l.startswith(("FAIL:", "ERROR:", "STALE:"))]
            lines += ["```text", *failures, "```", ""]
    else:
        lines += ["The fixture check didn't run (no `--preview`).", ""]

    lines += ["## Changed files", ""]
    for group in sorted(result["groups"], key=lambda g: (g in ("shared", "fixtures"), g)):
        paths = result["groups"][group]
        label = {"shared": "shared (affects several checks)", "fixtures": "fixtures"}.get(group, f"check `{group}`")
        if group == "fixtures" and len(paths) > 20:
            lines.append(f"- {label}: {len(paths)} files")
        else:
            lines.append(f"- {label}: " + ", ".join(f"`{p}`" for p in paths))
    if result["new_versions"]:
        lines.append("- New fixture versions: " + ", ".join(f"`{v}`" for v in result["new_versions"]))
    lines.append("")

    lines += ["## Upstream commits", ""]
    for sha, subject, k8s, release in result["commits"]:
        source = f", [kubernetes/kubernetes@{short(k8s)}](https://github.com/kubernetes/kubernetes/commit/{k8s})" if k8s else ""
        lines.append(f"- {commit_link(result['repository'], sha)} {subject} (first release: {release}{source})")
    lines.append("")

    if result["classification"] == "manual":
        lines += ["## Manual work", "",
                  "A Go check changed, so the CEL policies may need changes:", "",
                  "- [ ] Read the Go diff below and decide what the chart needs.",
                  "- [ ] Update the CEL in `charts/kyverno-policies/templates/`.",
                  "- [ ] Add a CLI test under `test/cli/test-validating-policy/pss-*`.",
                  "- [ ] Add a `semverCompare` gate if the change starts at a new version.",
                  "- [ ] Update `scripts/pss-upstream/mapping.yaml` for a new check.", "",
                  "<details><summary>Go diff of non-test <code>policy/</code> files</summary>", "",
                  "```diff", result["go_diff"].rstrip("\n"), "```", "", "</details>", ""]
    return "\n".join(lines) + "\n"


def write_result(result, preview, path):
    """Writes the machine-readable result that scripts/publish-pss-upstream.py reads."""
    data = {key: result[key] for key in ("repository", "branch", "pin", "head", "classification")}
    data["groups"] = sorted(result["groups"])
    data["preview"] = preview[0] if preview else "not-run"
    data["preview_summary"] = preview[1] if preview else ""
    path.write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")


def write_candidate_pin(pin, result, path):
    candidate = dict(pin, repository=result["repository"], branch=result["branch"], commit=result["head"])
    header = ("# Candidate pin written by scripts/sync-pss-upstream.py. Copy it to\n"
              "# scripts/pss-upstream/upstream.yaml to move the pin.\n")
    path.write_text(header + yaml.safe_dump(candidate, sort_keys=False), encoding="utf-8")


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--out", required=True, help="directory for upstream.yaml and report.md")
    parser.add_argument("--pin", default=str(DATA_DIR / "upstream.yaml"), help="pin file")
    parser.add_argument("--mapping", help="mapping file for the fixture check (default: the verifier's)")
    parser.add_argument("--repository", help="override the pinned repository (https:// only)")
    parser.add_argument("--branch", help="override the pinned branch")
    parser.add_argument("--git", default="git", help="path to git")
    parser.add_argument("--preview", action="store_true", help="run the fixture check on the candidate commit")
    parser.add_argument("--cli", help="path to kubectl-kyverno (required with --preview)")
    parser.add_argument("--helm", default=os.environ.get("HELM", "helm"), help="path to helm")
    parser.add_argument("--jobs", type=int, default=min(4, os.cpu_count() or 1), help="fixture check jobs")
    args = parser.parse_args(argv)
    if args.preview and not args.cli:
        parser.error("--preview needs --cli")
    try:
        pin = verifier.load_pin(yaml.safe_load(Path(args.pin).read_text(encoding="utf-8")) or {})
        repository = args.repository or pin["repository"]
        branch = args.branch or pin.get("branch") or "master"
        # Apply the verifier's https:// rule to an override too.
        verifier.load_pin({"repository": repository, "commit": pin["commit"]})
        if not re.fullmatch(r"[A-Za-z0-9._/-]+", branch) or ".." in branch:
            raise VerifyError(f"invalid branch name: {branch}")
        out = Path(args.out)
        out.mkdir(parents=True, exist_ok=True)
        with tempfile.TemporaryDirectory(prefix="pss-sync-") as tmp:
            log(f"comparing {repository} {branch} with pinned commit {pin['commit']}")
            result = detect(args.git, repository, branch, pin["commit"], Path(tmp))
        preview = None
        if result["classification"] != "none":
            write_candidate_pin(pin, result, out / "upstream.yaml")
            if args.preview:
                for name in ("cli", "helm"):
                    value = getattr(args, name)
                    if "/" in value or os.sep in value:
                        setattr(args, name, str(Path(value).resolve()))
                log(f"running the fixture check on {result['head']}")
                preview = run_preview(args, out / "upstream.yaml")
        report = render_report(result, preview)
        (out / "report.md").write_text(report, encoding="utf-8")
        write_result(result, preview, out / "result.json")
        print(report)
    except (VerifyError, OSError, yaml.YAMLError) as err:
        print(f"ERROR: {err}")
        return EXIT_ERROR
    if result["classification"] == "none":
        return EXIT_NO_CHANGE
    return EXIT_MISMATCH if preview is not None and preview[0] == "fail" else EXIT_CHANGE


if __name__ == "__main__":
    sys.exit(main())
