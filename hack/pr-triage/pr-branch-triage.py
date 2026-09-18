#!/usr/bin/env python3
"""
Dry-run classifier for open kyverno/kyverno PRs targeting `main`.

Classifies each PR by the files it touches into:
  LEGACY_ONLY     -> retarget to release-1.19      -> proposed label: type_legacy
  CEL_ONLY        -> keep on main                   -> proposed label: type_cel
  MIXED           -> manual: split or keep on main  -> proposed label: type_mixed
  SHARED_ONLY     -> keep on main (shared infra)     -> proposed label: type_shared
  REVIEW-MIGRATION-> keep on main, human must confirm -> no label applied automatically

Two tiers of classification:
  Tier 1 (always run): path-glob classification of changed files (RULES below).
  Tier 2 (--diff-scan, only refines Tier-1 SHARED_ONLY rows): scans the added/
    removed diff lines for legacy vs CEL identifiers (see classify_content()).
    A shared file with CEL-only content is promoted to CEL_ONLY (safe — no
    retargeting risk). A shared file with legacy-only or mixed content is
    downgraded to MIXED for mandatory human review — it is NEVER auto-promoted
    to LEGACY_ONLY from content alone, so it can never reach the automated
    retarget gate without a human setting override=RETARGET.

A migration-grace heuristic (§1.5 of PR_REBASE_PLAN.md) flags PRs whose
title/body reference legacy/migration/deprecation/1.20 AND that touch
pkg/deprecations/**, charts/kyverno/**, or a legacypolicies-fix CLI path.
These are reported as REVIEW-MIGRATION and are never auto-labeled or
auto-retargeted, even if Tier 1 would otherwise call them LEGACY_ONLY.

Every row also carries a `probe` field ("SKIPPED" by default). Pass
--probe-rebase to actually attempt `git rebase --onto <target-base>` for each
LEGACY_ONLY PR in a throwaway git worktree (nothing is pushed; the rebase is
always aborted and the worktree removed). This must be run from inside a
clone of kyverno/kyverno with a remote that has both `main` and the target
base (default `release-1.19`) fetchable. The approval gate in the plan
(category=LEGACY_ONLY && override!=KEEP_MAIN && probe=OK) is only
machine-checkable once this field is populated.

Read-only: only uses `gh api`/`gh pr list`/`gh pr diff` GET calls, plus local,
non-mutating `git fetch`/`git worktree`/`git rebase --abort` when
--probe-rebase is used. No PR is ever modified by this script.

Usage:
  pr-branch-triage.py [--repo kyverno/kyverno] [--base main] [--limit N]
                      [--out report.md] [--json report.json]
                      [--skip-bots] [--diff-scan] [--probe-rebase]
                      [--probe-target-base release-1.19]
  pr-branch-triage.py --reclassify report.json [--diff-scan] [--probe-rebase]
"""
import argparse
import fnmatch
import json
import re
import subprocess
import sys
import threading
from collections import Counter
from concurrent.futures import ThreadPoolExecutor, as_completed

# ---------------------------------------------------------------------------
# Path rules. Order matters: first match wins. Evaluated per changed file.
# More specific globs must come before broader ones that would otherwise
# shadow them (e.g. a single-file LEGACY override inside an otherwise-SHARED
# directory must be listed in OVERRIDE, checked before SHARED).
# ---------------------------------------------------------------------------
RULES = [
    # ---- Explicit overrides: specific legacy files inside otherwise-shared
    # directories. Must be checked before the broader SHARED entry below, or
    # the SHARED glob shadows them and they are never reached.
    ("LEGACY", [
        "pkg/admissionpolicy/kyvernopolicy_checker*",
    ]),
    # ---- SHARED overrides: legacy-looking paths that CEL/reports depend on --
    ("SHARED", [
        "pkg/engine/api/**",
        "pkg/engine/jmespath/**",
        "pkg/engine/mutate/patch/**",
        "pkg/engine/context/loaders/**",
        "pkg/engine/factories/**",
        "pkg/engine/adapters/**",
        "pkg/admissionpolicy/**",
        "**/AGENTS.md", "**/CLAUDE.md", "**/*.md",
    ]),
    # ---- CEL (policies.kyverno.io) -----------------------------------------
    ("CEL", [
        "pkg/cel/**",
        "pkg/background/gpol/**",
        "pkg/background/mpol/**",
        "pkg/policy/gpol.go", "pkg/policy/gpol_test.go",
        "pkg/policy/mpol.go", "pkg/policy/mpol_test.go",
        "pkg/webhooks/resource/gpol/**",
        "pkg/webhooks/resource/ivpol/**",
        "pkg/webhooks/resource/mpol/**",
        "pkg/webhooks/resource/vpol/**",
        "pkg/webhooks/celexception/**",
        "pkg/controllers/deleting/**",
        "pkg/controllers/policystatus/gpol.go", "pkg/controllers/policystatus/ivpol.go",
        "pkg/controllers/policystatus/mpol.go", "pkg/controllers/policystatus/ngpol.go",
        "pkg/controllers/policystatus/nivpol.go", "pkg/controllers/policystatus/vpol.go",
        "pkg/controllers/admissionpolicygenerator/vpol.go",
        "pkg/controllers/admissionpolicygenerator/mpol*.go",
        "pkg/controllers/webhook/mpol*.go",
        "pkg/image/verifiers/ivpol/**",
        "pkg/image/verification/**",
        "config/crds/policies.kyverno.io/**",
        "charts/kyverno/charts/crds/templates/policies.kyverno.io/**",
        "test/conformance/chainsaw/cel/**",
        "test/conformance/chainsaw/validating-policies/**",
        "test/conformance/chainsaw/mutating-policies/**",
        "test/conformance/chainsaw/generating-policies/**",
        "test/conformance/chainsaw/deleting-policies/**",
        "test/conformance/chainsaw/image-validating-policies/**",
        "test/conformance/chainsaw/namespaced-*/**",
        # Explicit CEL CLI test suites (kept as an explicit list, NOT a
        # "test/cli/test-*-policy/**" wildcard: that wildcard also matched
        # the legacy test/cli/test-cleanup-policy/** suite and misclassified
        # it as CEL).
        "test/cli/test-deleting-policy/**",
        "test/cli/test-generating-policy/**",
        "test/cli/test-image-validating-policy/**",
        "test/cli/test-mutating-policy/**",
        "test/cli/test-validating-policy/**",
        "test/cli/test-context-*-vpol/**", "test/cli/test-context-*-mpol/**",
        "test/cli/test-context-*-gpol/**", "test/cli/test-context-*-dpol/**",
        "test/cli/test-context-*-ivpol/**", "test/cli/test-gpol-custom-crd/**",
        "cmd/cli/kubectl-kyverno/processor/*vpol*", "cmd/cli/kubectl-kyverno/processor/*mpol*",
        "cmd/cli/kubectl-kyverno/processor/*gpol*", "cmd/cli/kubectl-kyverno/processor/*dpol*",
        "cmd/cli/kubectl-kyverno/processor/*ivpol*",
    ]),
    # ---- LEGACY (kyverno.io ClusterPolicy / Policy / CleanupPolicy) --------
    ("LEGACY", [
        "api/kyverno/v1/**",
        "api/kyverno/v1beta1/**",
        "api/kyverno/v2beta1/**",
        "api/kyverno/v2/cleanup_policy*",
        "api/kyverno/v2/updaterequest_types.go",
        "pkg/engine/**",
        "pkg/autogen/**",
        "pkg/background/**",
        "pkg/policy/**",
        "pkg/policycache/**",
        "pkg/controllers/policycache/**",
        "pkg/controllers/cleanup/**",
        "pkg/controllers/admissionpolicygenerator/cpol.go",
        "pkg/controllers/admissionpolicygenerator/generate-vap.go",
        "pkg/controllers/admissionpolicygenerator/vap.go",
        "pkg/validation/policy/**",
        "pkg/validation/cleanuppolicy/**",
        "pkg/webhooks/resource/generation/**",
        "pkg/webhooks/resource/imageverification/**",
        "pkg/webhooks/resource/mutation/**",
        "pkg/webhooks/resource/validation/**",
        "pkg/webhooks/resource/updaterequest*.go",
        "pkg/webhooks/resource/validation*.go",
        "pkg/webhooks/updaterequest/**",
        "pkg/webhooks/policy/**",
        "pkg/image/verifiers/cpol/**",
        "pkg/cosign/**", "pkg/notary/**",
        "pkg/pss/**",
        "cmd/cleanup-controller/**",
        "cmd/background-controller/**",
        "cmd/cli/kubectl-kyverno/commands/migrate/**",
        "cmd/cli/kubectl-kyverno/commands/fix/**",
        "cmd/cli/kubectl-kyverno/fix/**",
        "config/crds/kyverno/kyverno.io_clusterpolicies.yaml",
        "config/crds/kyverno/kyverno.io_policies.yaml",
        "config/crds/kyverno/kyverno.io_cleanuppolicies.yaml",
        "config/crds/kyverno/kyverno.io_clustercleanuppolicies.yaml",
        "config/crds/kyverno/kyverno.io_updaterequests.yaml",
        "charts/kyverno-policies/**",
        "test/conformance/chainsaw/validate/**",
        "test/conformance/chainsaw/mutate/**",
        "test/conformance/chainsaw/generate/**",
        "test/conformance/chainsaw/cleanup/**",
        "test/conformance/chainsaw/autogen/**",
        "test/conformance/chainsaw/verify-images/**",
        "test/conformance/chainsaw/verify-manifests/**",
        "test/conformance/chainsaw/background-only/**",
        "test/conformance/chainsaw/rangeoperators/**",
        "test/conformance/chainsaw/generate-validating-admission-policy/**",
        "test/conformance/chainsaw/generate-mutating-admission-policy*/**",
        "test/conformance/chainsaw/deferred/**",
        "test/conformance/chainsaw/force-failure-policy-ignore/**",
        "test/conformance/chainsaw/policy-validation/**",
        "test/cli/test/**", "test/cli/apply/**", "test/cli/test-generate/**",
        "test/cli/test-mutate/**", "test/cli/test-fail/**", "test/cli/test-cleanup-policy/**",
        "test/cli/test-context-apicall/**", "test/cli/test-context-configmap/**",
        "test/cli/test-exceptions/**", "test/cli/scenarios_to_cli/**",
        "test/cli/test-ruleless-policy/**", "test/cli/sample-policy-exclusion/**",
        "test/policy/**",
        "test/fuzz/**",
    ]),
]

# Everything else is SHARED (infra, docs, deps, CI, common webhooks, reports, ...)

# ---------------------------------------------------------------------------
# Tier-2 diff-content regexes (used only to refine Tier-1 SHARED_ONLY rows).
# ---------------------------------------------------------------------------
LEGACY_CONTENT_RE = re.compile(
    r"\b(kyvernov1|kyvernov2beta1|kyvernov1beta1)\.|\bClusterPolicy\b|\bCleanupPolicy\b|"
    r"\bClusterCleanupPolicy\b|\bUpdateRequest\b|engineapi\.|enginecontext\.|\bjmespath\b|"
    r"\bautogen\.|policycache\.|\bkyverno\.io/v1\b|kyverno\.io/v2beta1|\bpkg/engine\b|\bcpol\b|"
    r"\bpolicy\.Interface\b|kyvernov1\.Rule|PolicyInterface"
)
CEL_CONTENT_RE = re.compile(
    r"policiesv1(alpha1|beta1)\.|\b(Namespaced)?(Validating|Mutating|Generating|Deleting|ImageValidating)Policy\b|"
    r"policies\.kyverno\.io|\b(vpol|mpol|gpol|dpol|ivpol|nvpol|nmpol|ngpol|nivpol)\b|celengine\.|pkg/cel\b"
)

# ---------------------------------------------------------------------------
# Migration-grace heuristic (§1.5): PRs that touch legacy paths in order to
# gate/warn/block/migrate them belong on `main`, not release-1.19.
# ---------------------------------------------------------------------------
MIGRATION_TITLE_RE = re.compile(
    r"(?i)\b(migrat|deprecat|1\.20|legacy[ -_]*(policy|policies|gate|optout|warn|signal|block|cr|crd|manifest))\b"
)
MIGRATION_PATH_GLOBS = [
    "pkg/deprecations/**",
    "cmd/cli/kubectl-kyverno/**/legacypolicies/**",
    "cmd/cli/kubectl-kyverno/**/legacypolicies*",
    "charts/kyverno/**/legacy-policy*",
    "charts/kyverno/templates/hooks/post-upgrade-migrate*",
]

# ---------------------------------------------------------------------------
# Bot-author detection for --skip-bots. GitHub represents Dependabot/Renovate
# PR authors in more than one login form depending on how they were created
# (classic "dependabot[bot]" vs. GitHub-App-flavoured "app/dependabot"), so
# match on both.
# ---------------------------------------------------------------------------
BOT_LOGIN_RE = re.compile(r"(?i)^(app/)?(dependabot|renovate)(\[bot\])?$")


def is_bot_login(login):
    return bool(BOT_LOGIN_RE.match(login))


def _glob_match(path, g):
    return fnmatch.fnmatch(path, g) or fnmatch.fnmatch(path, g.replace("/**", ""))


def classify_file(path):
    for cat, globs in RULES:
        for g in globs:
            if _glob_match(path, g):
                return cat
            # fnmatch "*" matches "/" so "dir/**" behaves as a recursive prefix
    return "SHARED"


def classify_pr(files):
    cats = Counter(classify_file(f) for f in files)
    has_legacy, has_cel = cats["LEGACY"] > 0, cats["CEL"] > 0
    if has_legacy and has_cel:
        return "MIXED", cats
    if has_legacy:
        return "LEGACY_ONLY", cats
    if has_cel:
        return "CEL_ONLY", cats
    return "SHARED_ONLY", cats


def classify_content(diff_text):
    """Tier-2: scan added/removed diff lines for legacy vs CEL identifiers."""
    hunks = [
        l[1:] for l in diff_text.splitlines()
        if (l.startswith("+") or l.startswith("-")) and not l.startswith(("+++", "---"))
    ]
    lg = sum(1 for l in hunks if LEGACY_CONTENT_RE.search(l))
    ce = sum(1 for l in hunks if CEL_CONTENT_RE.search(l))
    if lg and ce:
        signal = "MIXED_CONTENT"
    elif lg:
        signal = "LEGACY_CONTENT"
    elif ce:
        signal = "CEL_CONTENT"
    else:
        signal = "NEUTRAL"
    return signal, lg, ce


def refine_with_content(row, repo):
    """Apply Tier-2 content refinement.

    Policy (keeps §2.2/§5 consistent — see PR_REBASE_PLAN.md):
      - SHARED_ONLY + CEL-only content    -> promote to CEL_ONLY (safe: keeps on main)
      - SHARED_ONLY + legacy/mixed content -> demote to MIXED (manual review)
      - LEGACY_ONLY + CEL/mixed content   -> demote to MIXED (manual review, prevents
        unsafe retargeting of mixed PRs where CEL code is in shared files)
      - Diff fetch error                  -> demote to MIXED (fail closed for safety)
      - Content signals alone NEVER auto-promote to LEGACY_ONLY.
    """
    cat = row["category"]
    if cat not in ("SHARED_ONLY", "LEGACY_ONLY"):
        row.setdefault("content_signal", None)
        return
    try:
        res = subprocess.run(
            ["gh", "pr", "diff", str(row["number"]), "-R", repo],
            capture_output=True, text=True, timeout=60,
            check=False,
        )
        if res.returncode != 0:
            row["content_signal"] = f"ERROR:diff_exit_{res.returncode}"
            row.pop("content_legacy_lines", None)
            row.pop("content_cel_lines", None)
            row["category"] = "MIXED"
            return
        diff = res.stdout
    except Exception as e:  # noqa: BLE001 - best-effort, record and move on
        row["content_signal"] = f"ERROR:{e}"
        row["category"] = "MIXED"
        return

    signal, lg, ce = classify_content(diff)
    row["content_signal"] = signal
    row["content_legacy_lines"] = lg
    row["content_cel_lines"] = ce

    if cat == "SHARED_ONLY":
        if signal == "CEL_CONTENT":
            row["category"] = "CEL_ONLY"
        elif signal in ("LEGACY_CONTENT", "MIXED_CONTENT"):
            row["category"] = "MIXED"
    elif cat == "LEGACY_ONLY":
        if signal in ("CEL_CONTENT", "MIXED_CONTENT"):
            row["category"] = "MIXED"


def is_migration_pr(title, body, files):
    text = f"{title or ''}\n{body or ''}"
    if not re.search(r"(?i)\b(migrat|deprecat|1\.20|legacy)\b", text):
        return False
    if not any(_glob_match(f, g) for f in files for g in MIGRATION_PATH_GLOBS):
        return False
    return True


def probe_rebase(repo, number, base, target_base):
    """Best-effort local rebase probe. Never pushes anything; always aborts
    the rebase and removes the worktree/ref it creates. Returns (status, conflict_files).

    Uses explicit local refs (refs/triage/pr-<N>, refs/triage/base-<base>-<N>,
    refs/triage/base-<target>-<N>) rather than FETCH_HEAD, and uses
    --no-write-fetch-head to prevent .git/FETCH_HEAD lock contention during
    concurrent probes.
    """
    pr_ref = f"refs/triage/pr-{number}"
    base_ref = f"refs/triage/base-{base}-{number}"
    target_ref = f"refs/triage/base-{target_base}-{number}"
    wt = f".wt-{number}"
    try:
        subprocess.run(
            ["git", "fetch", "--no-write-fetch-head", "-q", f"https://github.com/{repo}.git",
             f"pull/{number}/head:{pr_ref}", f"{base}:{base_ref}", f"{target_base}:{target_ref}"],
            check=True, capture_output=True, text=True, timeout=120,
        )
        subprocess.run(["git", "worktree", "add", "-q", "-d", wt, pr_ref],
                        check=True, capture_output=True, text=True, timeout=60)
        merge_base = subprocess.run(
            ["git", "-C", wt, "merge-base", "HEAD", base_ref],
            capture_output=True, text=True, timeout=30,
        ).stdout.strip()
        if not merge_base:
            return "ERROR:no-merge-base", []
        r = subprocess.run(
            ["git", "-C", wt, "rebase", "--onto", target_ref, merge_base],
            capture_output=True, text=True, timeout=180,
        )
        if r.returncode == 0:
            subprocess.run(["git", "-C", wt, "rebase", "--abort"], capture_output=True, text=True, timeout=60)
            return "OK", []
        conflicts = subprocess.run(
            ["git", "-C", wt, "diff", "--name-only", "--diff-filter=U"],
            capture_output=True, text=True, timeout=30,
        ).stdout.strip().splitlines()
        subprocess.run(["git", "-C", wt, "rebase", "--abort"], capture_output=True, text=True, timeout=60)
        if conflicts:
            return "CONFLICT", conflicts
        return f"ERROR:rebase_exit_{r.returncode}", []
    except Exception as e:  # noqa: BLE001
        return f"ERROR:{e}", []
    finally:
        subprocess.run(["git", "worktree", "remove", "-f", wt], capture_output=True, text=True, timeout=30)
        for ref in (pr_ref, base_ref, target_ref):
            subprocess.run(["git", "update-ref", "-d", ref], capture_output=True, text=True, timeout=30)


ACTION = {
    "LEGACY_ONLY": "Retarget -> release-1.19 (rebase onto release-1.19)",
    "CEL_ONLY": "Keep on main",
    "MIXED": "Manual review: split into 2 PRs or keep on main",
    "SHARED_ONLY": "Keep on main (verify after legacy removal lands)",
    "REVIEW-MIGRATION": "Keep on main — requires human confirmation (migration-grace, see §1.5)",
}

# Proposed label per category. These are NOT applied by this script — see
# apply-labels.sh, which defaults to a dry run (prints `gh pr edit` commands)
# and only mutates PRs when invoked with --execute. REVIEW-MIGRATION PRs get
# no automatic label: a maintainer must classify them by hand.
LABEL = {
    "LEGACY_ONLY": "type_legacy",
    "CEL_ONLY": "type_cel",
    "MIXED": "type_mixed",
    "SHARED_ONLY": "type_shared",
    "REVIEW-MIGRATION": None,
}


def gh(args):
    out = subprocess.run(["gh"] + args, check=True, capture_output=True, text=True).stdout
    return json.loads(out) if out.strip() else []


def finalize_row(row, a):
    """Apply migration-flag override, then compute action/label/probe fields.
    Shared by fresh-fetch, single-PR, and --reclassify code paths."""
    cat, counts = classify_pr(row["files"])
    row["category"] = cat
    row["counts"] = dict(counts)
    row["legacy_files"] = [f for f in row["files"] if classify_file(f) == "LEGACY"]
    row["cel_files"] = [f for f in row["files"] if classify_file(f) == "CEL"]

    if a.diff_scan:
        refine_with_content(row, a.repo)
    else:
        row["content_signal"] = None
        row.pop("content_legacy_lines", None)
        row.pop("content_cel_lines", None)

    row["migration_flag"] = is_migration_pr(row.get("title", ""), row.get("body", ""), row["files"])
    if row["migration_flag"]:
        row["category"] = "REVIEW-MIGRATION"
        row["override"] = "KEEP_MAIN"
    else:
        row.setdefault("override", None)

    cat = row["category"]
    row["action"] = ACTION[cat]
    row["proposed_label"] = LABEL[cat]

    # Reset probe status so reclassification without probe doesn't keep stale status
    row["probe"] = "SKIPPED"
    row["conflict_files"] = []
    if a.probe_rebase and (cat == "LEGACY_ONLY" or row.get("override") == "RETARGET"):
        status, conflicts = probe_rebase(a.repo, row["number"], a.base, a.probe_target_base)
        row["probe"] = status
        row["conflict_files"] = conflicts
    return row


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default="kyverno/kyverno")
    ap.add_argument("--base", default="main")
    ap.add_argument("--pr", type=int, help="triage a single PR number (used by pr-branch-guard CI)")
    ap.add_argument("--limit", type=int, default=500)
    ap.add_argument("--out", default="pr-triage-report.md")
    ap.add_argument("--json", dest="json_out", default="pr-triage-report.json")
    ap.add_argument("--skip-bots", action="store_true", help="skip dependabot/renovate PR authors")
    ap.add_argument("--diff-scan", action="store_true",
                     help="Tier-2: fetch `gh pr diff` and refine by diff content")
    ap.add_argument("--probe-rebase", action="store_true",
                     help="attempt a local, non-pushing `git rebase --onto` for each LEGACY_ONLY PR")
    ap.add_argument("--probe-target-base", default="release-1.19")
    ap.add_argument("--reclassify", metavar="JSON", help="re-run classification offline from a previous JSON report")
    ap.add_argument("--workers", type=int, default=8,
                     help="parallel `gh` subprocess workers for file/diff/body fetches (I/O bound, not CPU bound)")
    a = ap.parse_args()

    if a.pr:
        pr = gh(["pr", "view", str(a.pr), "-R", a.repo,
                 "--json", "number,title,body,author,isDraft,createdAt,updatedAt,headRefName,headRepositoryOwner,"
                           "isCrossRepository,maintainerCanModify,mergeable,labels"])
        raw = subprocess.run(["gh", "api", f"repos/{a.repo}/pulls/{a.pr}/files", "--paginate",
                              "--jq", ".[].filename"], check=True, capture_output=True, text=True).stdout
        files = [l for l in raw.splitlines() if l.strip()]
        row = {
            "number": pr["number"], "title": pr["title"], "body": pr.get("body") or "",
            "author": pr["author"]["login"], "draft": pr["isDraft"], "fork": pr["isCrossRepository"],
            "maintainerCanModify": pr["maintainerCanModify"], "mergeable": pr["mergeable"],
            "head": f"{pr['headRepositoryOwner']['login']}:{pr['headRefName']}",
            "labels": [l["name"] for l in pr["labels"]],
            "files": files,
        }
        finalize_row(row, a)
        write_reports(a, [row])
        return

    if a.reclassify:
        rows = json.load(open(a.reclassify))
        lock = threading.Lock()
        done = [0]

        def process(r):
            if "body" not in r or r.get("body") is None:
                # Older report snapshots didn't capture the PR body (needed
                # for the migration-grace heuristic) — backfill it lazily.
                res = subprocess.run(
                    ["gh", "pr", "view", str(r["number"]), "-R", a.repo, "--json", "body", "-q", ".body"],
                    capture_output=True, text=True, timeout=30,
                )
                if res.returncode == 0:
                    r["body"] = res.stdout.strip()
                else:
                    r["body"] = ""
                    r["body_fetch_error"] = True
            finalize_row(r, a)
            with lock:
                done[0] += 1
                print(f"[{done[0]}/{len(rows)}] #{r['number']} ...", file=sys.stderr, end="\r")

        with ThreadPoolExecutor(max_workers=a.workers) as pool:
            futures = [pool.submit(process, r) for r in rows]
            for f in as_completed(futures):
                f.result()  # surface any exception
        print(file=sys.stderr)
        write_reports(a, rows)
        return

    prs = gh(["pr", "list", "-R", a.repo, "--state", "open", "--base", a.base, "--limit", str(a.limit),
              "--json", "number,title,body,author,isDraft,createdAt,updatedAt,headRefName,headRepositoryOwner,"
                        "isCrossRepository,maintainerCanModify,mergeable,labels,changedFiles"])
    prs = [pr for pr in prs if not (a.skip_bots and is_bot_login(pr["author"]["login"]))]
    rows = [None] * len(prs)
    lock = threading.Lock()
    done = [0]

    def fetch_and_finalize(i, pr):
        raw = subprocess.run(["gh", "api", f"repos/{a.repo}/pulls/{pr['number']}/files", "--paginate",
                              "--jq", ".[].filename"], check=True, capture_output=True, text=True).stdout
        files = [l for l in raw.splitlines() if l.strip()]
        row = {
            "number": pr["number"], "title": pr["title"], "body": pr.get("body") or "", "author": pr["author"]["login"],
            "draft": pr["isDraft"], "fork": pr["isCrossRepository"],
            "maintainerCanModify": pr["maintainerCanModify"], "mergeable": pr["mergeable"],
            "head": f"{pr['headRepositoryOwner']['login']}:{pr['headRefName']}",
            "labels": [l["name"] for l in pr["labels"]],
            "files": files,
        }
        finalize_row(row, a)
        rows[i] = row
        with lock:
            done[0] += 1
            print(f"[{done[0]}/{len(prs)}] #{pr['number']} ...", file=sys.stderr, end="\r")

    with ThreadPoolExecutor(max_workers=a.workers) as pool:
        futures = [pool.submit(fetch_and_finalize, i, pr) for i, pr in enumerate(prs)]
        for f in as_completed(futures):
            f.result()  # surface any exception
    print(file=sys.stderr)
    write_reports(a, rows)


def write_reports(a, rows):
    with open(a.json_out, "w") as fh:
        json.dump(rows, fh, indent=2)

    summary = Counter(r["category"] for r in rows)
    order = ["LEGACY_ONLY", "MIXED", "CEL_ONLY", "SHARED_ONLY", "REVIEW-MIGRATION"]
    with open(a.out, "w") as fh:
        fh.write(f"# PR branch triage (DRY RUN) — {a.repo} base={a.base}\n\n")
        fh.write("| Category | Count | Proposed label | Action |\n|---|---|---|---|\n")
        for c in order:
            fh.write(f"| {c} | {summary[c]} | `{LABEL[c]}` | {ACTION[c]} |\n" if LABEL[c]
                     else f"| {c} | {summary[c]} | _(none — human review)_ | {ACTION[c]} |\n")
        fh.write("\n**No PRs were modified. This report is read-only. Labels are proposed, not applied "
                 "(see `apply-labels.sh --execute`). `probe` is `SKIPPED` unless `--probe-rebase` was passed; "
                 "the automated-retarget gate requires `category=LEGACY_ONLY && override!=KEEP_MAIN && probe=OK`.**\n")
        for c in order:
            sub = [r for r in rows if r["category"] == c]
            if not sub:
                continue
            label_hdr = f"`{LABEL[c]}`" if LABEL[c] else "none (human review)"
            fh.write(f"\n## {c} ({len(sub)}) — label {label_hdr}\n\n")
            fh.write("| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | "
                     "Content signal | Probe | Proposed label | Proposed Action |\n")
            fh.write("|---|---|---|---|---|---|---|---|---|---|---|---|\n")
            for r in sub:
                k = r["counts"]
                lbl = f"`{r['proposed_label']}`" if r.get("proposed_label") else "—"
                fh.write(f"| [#{r['number']}](https://github.com/{a.repo}/pull/{r['number']}) | @{r['author']} | "
                         f"{r['title'].replace('|', '\\|')[:70]} | {'Y' if r['draft'] else ''} | "
                         f"{'Y' if r['fork'] else ''} | {'Y' if r['maintainerCanModify'] else 'N'} | {r['mergeable']} | "
                         f"{k.get('LEGACY',0)}/{k.get('CEL',0)}/{k.get('SHARED',0)} | "
                         f"{r.get('content_signal') or ''} | {r.get('probe','SKIPPED')} | {lbl} | {r['action']} |\n")
            if c == "MIXED":
                fh.write("\n<details><summary>Mixed PR file breakdown</summary>\n\n")
                for r in sub:
                    fh.write(f"### #{r['number']} {r['title']}\n")
                    if r.get("legacy_files"):
                        fh.write(f"- LEGACY files: {', '.join(r['legacy_files'][:15])}\n")
                    if r.get("cel_files"):
                        fh.write(f"- CEL files: {', '.join(r['cel_files'][:15])}\n")
                    if r.get("content_signal"):
                        fh.write(f"- Content signal: `{r['content_signal']}` (legacy diff lines: {r.get('content_legacy_lines', 0)}, cel diff lines: {r.get('content_cel_lines', 0)})\n")
                    fh.write("\n")
                fh.write("</details>\n")
    print(f"wrote {a.out} and {a.json_out}")
    for c in order:
        print(f"{c:16s} {summary[c]}")


if __name__ == "__main__":
    main()
