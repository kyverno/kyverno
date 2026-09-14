#!/usr/bin/env python3
"""
Dry-run classifier for open kyverno/kyverno PRs targeting `main`.

Classifies each PR by the files it touches into:
  LEGACY_ONLY  -> retarget to release-1.19   -> proposed label: type_legacy
  CEL_ONLY     -> keep on main                -> proposed label: type_cel
  MIXED        -> manual: split or keep on main -> proposed label: type_mixed
  SHARED_ONLY  -> keep on main (shared infra, docs, deps, CI, ...) -> proposed label: type_shared

Read-only: only uses `gh api` / `gh pr list` GET calls. No PR is modified.
Proposed labels are recorded in the JSON/Markdown reports only; applying them
to PRs is a separate, explicit step handled by apply-labels.sh (dry-run by
default, requires --execute to mutate anything).

Usage:
  pr-branch-triage.py [--repo kyverno/kyverno] [--base main] [--limit N]
                      [--out report.md] [--json report.json] [--rules rules.yaml]
"""
import argparse
import fnmatch
import json
import subprocess
import sys
from collections import Counter

# ---------------------------------------------------------------------------
# Path rules. Order matters: first match wins. Evaluated per changed file.
# Prefix "!" is not used; instead the more specific CEL globs come first so
# that e.g. pkg/background/gpol/** is CEL even though pkg/background/** is legacy.
# ---------------------------------------------------------------------------
RULES = [
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
        "test/cli/test-*-policy/**",
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
        "pkg/admissionpolicy/kyvernopolicy_checker*",
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


def classify_file(path):
    for cat, globs in RULES:
        for g in globs:
            if fnmatch.fnmatch(path, g) or fnmatch.fnmatch(path, g.replace("/**", "")):
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


ACTION = {
    "LEGACY_ONLY": "Retarget -> release-1.19 (rebase onto release-1.19)",
    "CEL_ONLY": "Keep on main",
    "MIXED": "Manual review: split into 2 PRs or keep on main",
    "SHARED_ONLY": "Keep on main (verify after legacy removal lands)",
}

# Proposed label per category. These are NOT applied by this script — see
# apply-labels.sh, which defaults to a dry run (prints `gh pr edit` commands)
# and only mutates PRs when invoked with --execute.
LABEL = {
    "LEGACY_ONLY": "type_legacy",
    "CEL_ONLY": "type_cel",
    "MIXED": "type_mixed",
    "SHARED_ONLY": "type_shared",
}


def gh(args):
    out = subprocess.run(["gh"] + args, check=True, capture_output=True, text=True).stdout
    return json.loads(out) if out.strip() else []


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--repo", default="kyverno/kyverno")
    ap.add_argument("--base", default="main")
    ap.add_argument("--limit", type=int, default=500)
    ap.add_argument("--out", default="pr-triage-report.md")
    ap.add_argument("--json", dest="json_out", default="pr-triage-report.json")
    ap.add_argument("--skip-bots", action="store_true", help="skip dependabot/renovate PRs")
    ap.add_argument("--reclassify", metavar="JSON", help="re-run classification offline from a previous JSON report")
    a = ap.parse_args()

    if a.reclassify:
        rows = json.load(open(a.reclassify))
        for r in rows:
            cat, counts = classify_pr(r["files"])
            r.update(category=cat, action=ACTION[cat], proposed_label=LABEL[cat], counts=dict(counts),
                     legacy_files=[f for f in r["files"] if classify_file(f) == "LEGACY"],
                     cel_files=[f for f in r["files"] if classify_file(f) == "CEL"])
        write_reports(a, rows); return
    prs = gh(["pr", "list", "-R", a.repo, "--state", "open", "--base", a.base, "--limit", str(a.limit),
              "--json", "number,title,author,isDraft,createdAt,updatedAt,headRefName,headRepositoryOwner,"
                        "isCrossRepository,maintainerCanModify,mergeable,labels,changedFiles"])
    rows = []
    for i, pr in enumerate(prs, 1):
        login = pr["author"]["login"]
        if a.skip_bots and (login.startswith("dependabot") or login.startswith("renovate")):
            continue
        print(f"[{i}/{len(prs)}] #{pr['number']} ...", file=sys.stderr, end="\r")
        raw = subprocess.run(["gh", "api", f"repos/{a.repo}/pulls/{pr['number']}/files", "--paginate",
                              "--jq", ".[].filename"], check=True, capture_output=True, text=True).stdout
        files = [l for l in raw.splitlines() if l.strip()]
        cat, counts = classify_pr(files)
        rows.append({
            "number": pr["number"], "title": pr["title"], "author": login,
            "draft": pr["isDraft"], "fork": pr["isCrossRepository"],
            "maintainerCanModify": pr["maintainerCanModify"], "mergeable": pr["mergeable"],
            "head": f"{pr['headRepositoryOwner']['login']}:{pr['headRefName']}",
            "labels": [l["name"] for l in pr["labels"]],
            "files": files, "counts": dict(counts), "category": cat, "action": ACTION[cat],
            "proposed_label": LABEL[cat],
            "legacy_files": [f for f in files if classify_file(f) == "LEGACY"],
            "cel_files": [f for f in files if classify_file(f) == "CEL"],
        })
    print(file=sys.stderr)
    write_reports(a, rows)


def write_reports(a, rows):
    with open(a.json_out, "w") as fh:
        json.dump(rows, fh, indent=2)

    summary = Counter(r["category"] for r in rows)
    order = ["LEGACY_ONLY", "MIXED", "CEL_ONLY", "SHARED_ONLY"]
    with open(a.out, "w") as fh:
        fh.write(f"# PR branch triage (DRY RUN) — {a.repo} base={a.base}\n\n")
        fh.write("| Category | Count | Proposed label | Action |\n|---|---|---|---|\n")
        for c in order:
            fh.write(f"| {c} | {summary[c]} | `{LABEL[c]}` | {ACTION[c]} |\n")
        fh.write("\n**No PRs were modified. This report is read-only. Labels are proposed, not applied "
                 "(see `apply-labels.sh --execute`).**\n")
        for c in order:
            sub = [r for r in rows if r["category"] == c]
            if not sub:
                continue
            fh.write(f"\n## {c} ({len(sub)}) — label `{LABEL[c]}`\n\n")
            fh.write("| PR | Author | Title | Draft | Fork | CanModify | Mergeable | Files (L/C/S) | Proposed label | Proposed Action |\n")
            fh.write("|---|---|---|---|---|---|---|---|---|---|\n")
            for r in sub:
                k = r["counts"]
                fh.write(f"| [#{r['number']}](https://github.com/{a.repo}/pull/{r['number']}) | @{r['author']} | "
                         f"{r['title'].replace('|', '\\|')[:70]} | {'Y' if r['draft'] else ''} | "
                         f"{'Y' if r['fork'] else ''} | {'Y' if r['maintainerCanModify'] else 'N'} | {r['mergeable']} | "
                         f"{k.get('LEGACY',0)}/{k.get('CEL',0)}/{k.get('SHARED',0)} | `{r.get('proposed_label', LABEL[c])}` | {r['action']} |\n")
            if c == "MIXED":
                fh.write("\n<details><summary>Mixed PR file breakdown</summary>\n\n")
                for r in sub:
                    fh.write(f"### #{r['number']} {r['title']}\n- LEGACY: {', '.join(r['legacy_files'][:15])}\n"
                             f"- CEL: {', '.join(r['cel_files'][:15])}\n\n")
                fh.write("</details>\n")
    print(f"wrote {a.out} and {a.json_out}")
    for c in order:
        print(f"{c:12s} {summary[c]}")


if __name__ == "__main__":
    main()
