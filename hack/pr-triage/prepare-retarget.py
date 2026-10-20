#!/usr/bin/env python3
"""Prepare review evidence for legacy retargeting. Never writes to GitHub."""

import argparse
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
import importlib.util
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import uuid


SPEC = importlib.util.spec_from_file_location(
    "triage", Path(__file__).with_name("pr-branch-triage.py")
)
TRIAGE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(TRIAGE)
FEATURE = re.compile(r"(?i)^\s*feat(?:ure)?\b")
SHA = re.compile(r"^[0-9a-f]{40}$")


def run(*args, cwd=None, check=True, timeout=180, env=None):
    return subprocess.run(
        args, cwd=cwd, check=check, timeout=timeout, capture_output=True, text=True, env=env
    )


def git(*args, **kwargs):
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
    return run(
        "git", "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false",
        *args, env=env, **kwargs
    )


def gh(*args):
    env = {k: v for k, v in os.environ.items() if not k.startswith("GH_") or k == "GH_TOKEN"}
    env.update(GH_HOST="github.com", GH_PROMPT_DISABLED="1", GH_NO_UPDATE_NOTIFIER="1")
    return json.loads(run("gh", *args, env=env).stdout)


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def same_identity(row, pr):
    return (
        pr["state"] == "open"
        and pr["base"]["ref"] == row["base_ref"]
        and pr["head"]["sha"] == row["head_sha"]
        and pr["updated_at"] == row["updated_at"]
    )


def overlap(files, release_prs):
    paths = set(files)
    return [
        {
            "number": pr["number"],
            "title": pr["title"],
            "files": sorted(paths.intersection(pr["files"])),
        }
        for pr in release_prs
        if paths.intersection(pr["files"])
    ]


def verify_routing(snapshot, pr, files, diff):
    category, _ = TRIAGE.classify_pr(files)
    signal, _, _ = TRIAGE.classify_content(diff)
    migration = TRIAGE.is_migration_pr(pr.get("title", ""), pr.get("body", ""), files)
    if category == "LEGACY_ONLY" and signal in ("CEL_CONTENT", "MIXED_CONTENT"):
        category = "MIXED"
    if migration:
        category = "REVIEW-MIGRATION"
    if (
        category != "LEGACY_ONLY" or snapshot.get("category") != category
        or snapshot.get("content_signal") != signal
        or snapshot.get("migration_flag") is not migration
    ):
        raise ValueError("Pinned routing differs from current classification; refresh report")


def recommend(row):
    if row.get("override") == "KEEP_MAIN":
        return "KEEP_MAIN", ["Explicit maintainer override: keep on main."]
    if row.get("override") == "SPLIT":
        return "SPLIT", ["Explicit maintainer override: split before retargeting."]
    reasons = []
    if row.get("migration_flag"):
        reasons.append("Migration/removal work must remain on main.")
    if row["probe"] == "EMPTY":
        return "KEEP_MAIN", ["No remaining commits after rebase; inspect already-landed evidence."]
    if row["draft"]:
        reasons.append("Draft: do not rebase while the author is iterating.")
    if FEATURE.search(row["title"]):
        reasons.append("Feature: explicit release-maintainer approval is required.")
    if row["shared_files"]:
        reasons.append("Shared-file hunks require an explicit legacy-only scope decision.")
    if row["overlap_prs"]:
        reasons.append("Overlaps open release PRs; compare fixes before moving.")
    if row.get("overlap_candidates"):
        reasons.append("Overlaps other legacy candidates; deduplicate or sequence before moving.")
    if any(label.startswith("type_") and label != "type_legacy" for label in row.get("labels", [])):
        reasons.append("Existing maintainer type label differs; resolve the override explicitly.")
    if row["category"] != "LEGACY_ONLY":
        reasons.append("Live classification is no longer legacy-only.")
    if row["probe"] != "OK":
        reasons.append(f"Probe {row['probe']}: author or maintainer intervention required.")
    if reasons:
        return "NEEDS_AUTHOR", reasons
    return "RETARGET", ["Clean legacy-only probe; evidence review, tests and approval still required."]


def summarize(manifest, output):
    rows = manifest["candidates"]
    for row in rows:
        row["overlap_candidates"] = overlap(
            row["files"], [r for r in rows if r["number"] != row["number"]]
        )
        row["recommendation"], row["reasons"] = recommend(row)
        write_json(output / str(row["number"]) / "candidate.json", row)
    write_json(output / "manifest.json", manifest)
    omitted = {"body", "legacy_files", "cel_files", "counts", "action"}
    review = {
        **manifest,
        "release_prs": [
            {key: value for key, value in pr.items() if key != "files"}
            for pr in manifest["release_prs"]
        ],
        "candidates": [
            {key: value for key, value in row.items() if key not in omitted}
            for row in rows
        ],
    }
    write_json(output / "review.json", review)
    (output / "manifest.md").write_text(render(manifest))


def prepare_one(snapshot, repo, source_ref, target_ref, namespace, output, release_prs):
    number = snapshot["number"]
    prefix = f"{namespace}/{number}"
    backup_ref = f"{prefix}/original"
    prepared_ref = f"{prefix}/rebased"
    evidence_dir = output / str(number)
    evidence_dir.mkdir()
    row = {
        **snapshot,
        "probe": "SKIPPED",
        "merge_base": None,
        "rebased_sha": None,
        "backup_ref": backup_ref,
        "prepared_ref": None,
        "conflict_files": [],
        "shared_files": [
            p for p in snapshot["files"] if TRIAGE.classify_file(p) == "SHARED"
        ],
        "overlap_prs": overlap(snapshot["files"], release_prs),
        "approval": None,
        "strategy": "sibling",
        "consent_comment_id": None,
        "evidence": {},
        "evidence_sha256": {},
    }
    worktree_added = False
    with tempfile.TemporaryDirectory(prefix=f"kyverno-retarget-{number}-") as temp:
        wt = Path(temp) / "worktree"
        isolated = Path(temp) / "probe.git"
        try:
            pr = gh("api", f"repos/{repo}/pulls/{number}")
            if not same_identity(snapshot, pr):
                raise ValueError("PR state/head/base/metadata changed since classification; refresh report")
            if not pr["head"]["repo"]:
                raise ValueError("PR head repository has been deleted")
            row["head_repository"] = pr["head"]["repo"]["full_name"]
            row["head_ref"] = pr["head"]["ref"]
            row["draft"] = pr["draft"]
            row["author"] = pr["user"]["login"]
            row["maintainerCanModify"] = pr["maintainer_can_modify"]
            if len(snapshot["files"]) != pr["changed_files"]:
                raise ValueError("File inventory is incomplete; refresh classification")
            git(
                "fetch", "--no-write-fetch-head", "-q", f"https://github.com/{repo}.git",
                f"pull/{number}/head:{backup_ref}",
            )
            if git("rev-parse", backup_ref).stdout.strip() != snapshot["head_sha"]:
                raise ValueError("PR head changed while fetching")
            merge_base = git("merge-base", source_ref, backup_ref).stdout.strip()
            row["merge_base"] = merge_base
            original_files = git(
                "diff", "--no-ext-diff", "--no-textconv", "--name-only", merge_base, backup_ref
            ).stdout.splitlines()
            if set(original_files) != set(snapshot["files"]):
                raise ValueError("Pinned source diff differs from classified file inventory")
            original_diff = git("diff", "--no-ext-diff", "--no-textconv", merge_base, backup_ref).stdout
            verify_routing(snapshot, pr, original_files, original_diff)
            (evidence_dir / "original.diff").write_text(original_diff)
            row["evidence"]["original_diff"] = f"{number}/original.diff"
            shared_diff = git(
                "diff", "--no-ext-diff", "--no-textconv", merge_base, backup_ref,
                "--", *row["shared_files"],
            ).stdout if row["shared_files"] else ""
            (evidence_dir / "shared.diff").write_text(shared_diff)
            row["evidence"]["shared_diff"] = f"{number}/shared.diff"
            row["shared_content_signal"] = TRIAGE.classify_content(shared_diff)[0]
            row["patch_equivalence"] = git(
                "cherry", target_ref, backup_ref, merge_base
            ).stdout.splitlines()
            (evidence_dir / "commits.txt").write_text(
                git("log", "--format=%H %s", f"{merge_base}..{backup_ref}").stdout
            )
            row["evidence"]["commits"] = f"{number}/commits.txt"
            # A new repository prevents PR attributes from selecting operator-configured drivers.
            git("init", "--bare", "--template=", "-q", str(isolated))
            common = git("rev-parse", "--path-format=absolute", "--git-common-dir").stdout.strip()
            target_sha = git("rev-parse", target_ref).stdout.strip()
            git(
                "-C", str(isolated), "fetch", "--no-write-fetch-head", "-q",
                common, snapshot["head_sha"], target_sha,
            )
            git("-C", str(isolated), "config", "user.name", "Kyverno retarget probe")
            git("-C", str(isolated), "config", "user.email", "retarget@users.noreply.github.com")
            git("-C", str(isolated), "worktree", "add", "-q", "-d", str(wt), snapshot["head_sha"])
            worktree_added = True
            result = git(
                "-c", "rebase.updateRefs=false", "-c", "rerere.enabled=false",
                "-c", "commit.gpgSign=false", "-C", str(wt), "rebase",
                "--no-autostash", "--onto", target_sha, merge_base,
                check=False, timeout=300,
            )
            (evidence_dir / "rebase.log").write_text(result.stdout + result.stderr)
            row["evidence"]["rebase_log"] = f"{number}/rebase.log"
            if result.returncode:
                row["conflict_files"] = git(
                    "-C", str(wt), "diff", "--no-ext-diff", "--no-textconv",
                    "--name-only", "--diff-filter=U"
                ).stdout.splitlines()
                row["probe"] = "CONFLICT" if row["conflict_files"] else "ERROR:rebase"
            else:
                rebased = git("-C", str(wt), "rev-parse", "HEAD").stdout.strip()
                git("fetch", "--no-write-fetch-head", "-q", str(isolated), rebased)
                git("update-ref", prepared_ref, rebased, "0" * 40)
                row["prepared_ref"] = prepared_ref
                row["rebased_sha"] = rebased
                count = int(git("rev-list", "--count", f"{target_ref}..{rebased}").stdout)
                row["probe"] = "OK" if count else "EMPTY"
                (evidence_dir / "final.diff").write_text(
                    git("diff", "--no-ext-diff", "--no-textconv", target_ref, rebased).stdout
                )
                row["evidence"]["final_diff"] = f"{number}/final.diff"
                (evidence_dir / "range-diff.txt").write_text(
                    git(
                        "range-diff", "--no-color", "--no-ext-diff", "--no-textconv",
                        f"{merge_base}..{backup_ref}", f"{target_ref}..{rebased}",
                    ).stdout if count else "EMPTY: all commits dropped; inspect patch equivalence.\n"
                )
                row["evidence"]["range_diff"] = f"{number}/range-diff.txt"
            if not same_identity(snapshot, gh("api", f"repos/{repo}/pulls/{number}")):
                row["probe"] = "ERROR:PR changed during preparation"
        except (subprocess.SubprocessError, OSError, ValueError, KeyError) as error:
            row["probe"] = f"ERROR:{error}"
            (evidence_dir / "error.txt").write_text(str(error) + "\n")
        finally:
            if worktree_added:
                # Removing this owned worktree also discards any in-progress rebase.
                git("-C", str(isolated), "worktree", "remove", "--force", str(wt))
    row["evidence_sha256"] = {
        key: hashlib.sha256((output / path).read_bytes()).hexdigest()
        for key, path in row["evidence"].items()
    }
    row["recommendation"], row["reasons"] = recommend(row)
    write_json(evidence_dir / "candidate.json", row)
    print(f"#{number}: {row['probe']} -> {row['recommendation']}", flush=True)
    return row


def render(manifest):
    rows = manifest["candidates"]
    counts = dict(Counter(r["probe"] for r in rows))
    lines = [
        "# Legacy PR migration manifest (DRY RUN)",
        "",
        f"Generated: {manifest['generated_at']}",
        f"Repository: {manifest['repository']}",
        f"Source: `{manifest['source']['branch']}@{manifest['source']['sha']}`",
        f"Target: `{manifest['target']['branch']}@{manifest['target']['sha']}`",
        "",
        f"Candidates: {len(rows)}. Probe results: `{json.dumps(counts, sort_keys=True)}`.",
        "**Recommendations are not approvals. Every approval is initially null.**",
        "No PRs, labels or remote branches were changed. Local backup/prepared refs are retained.",
        "Overlap means shared changed paths, not a proven duplicate fix.",
        "",
        "| PR | Recommendation | Probe | Shared files | Overlapping release PRs | Reasons |",
        "|---|---|---|---:|---|---|",
    ]
    for row in rows:
        links = ", ".join(f"#{p['number']}" for p in row["overlap_prs"])
        reasons = " ".join(row["reasons"]).replace("|", "\\|").replace("\n", " ")
        lines.append(
            f"| [#{row['number']}](https://github.com/{manifest['repository']}/pull/{row['number']}) "
            f"| {row['recommendation']} | {row['probe'].replace('|', '/')} "
            f"| {len(row['shared_files'])} | {links} | {reasons} |"
        )
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True, help="new artifact directory")
    parser.add_argument("--repo", default="kyverno/kyverno", choices=["kyverno/kyverno"])
    parser.add_argument("--workers", type=int, default=3)
    args = parser.parse_args()
    if not 1 <= args.workers <= 8:
        parser.error("--workers must be between 1 and 8")
    report_text = args.report.read_text()
    snapshots = json.loads(report_text)
    if not isinstance(snapshots, list) or not all(isinstance(r, dict) for r in snapshots):
        parser.error("report must be a JSON array of classified PR objects")
    inventory = dict(Counter(r["category"] for r in snapshots))
    snapshots = [r for r in snapshots if r["category"] == "LEGACY_ONLY"]
    if not snapshots:
        parser.error("report must contain legacy candidates")
    for row in snapshots:
        if type(row.get("number")) is not int or row["number"] <= 0:
            parser.error("every candidate must have a positive integer PR number")
        if row.get("base_ref") != "main" or not SHA.fullmatch(row.get("head_sha", "")):
            parser.error("report lacks pinned main PR identities; regenerate it")
        if row.get("content_signal") not in ("NEUTRAL", "LEGACY_CONTENT"):
            parser.error("every legacy candidate must have a successful --diff-scan")
    if len({r["number"] for r in snapshots}) != len(snapshots):
        parser.error("report must contain unique legacy candidates")
    args.output.mkdir(parents=True, exist_ok=False)
    namespace = f"refs/retarget/prepared/{uuid.uuid4().hex}"
    branches = {}
    for key, branch in (("source", "main"), ("target", "release-1.19")):
        data = gh("api", f"repos/{args.repo}/branches/{branch}")
        branches[key] = {"branch": branch, "sha": data["commit"]["sha"]}
    git(
        "fetch", "--no-write-fetch-head", "-q", f"https://github.com/{args.repo}.git",
        f"main:{namespace}/source", f"release-1.19:{namespace}/target",
    )
    for key in ("source", "target"):
        if git("rev-parse", f"{namespace}/{key}").stdout.strip() != branches[key]["sha"]:
            raise ValueError(f"{key} branch moved during capture; regenerate")
    release_prs = gh(
        "pr", "list", "-R", args.repo, "--state", "open", "--base", "release-1.19",
        "--limit", "1000", "--json", "number,title,headRefOid,files",
    )
    if len(release_prs) >= 1000:
        raise ValueError("Release PR inventory reached its limit; paginate before proceeding")
    for pr in release_prs:
        # The GraphQL files connection can truncate large PRs; REST is paginated.
        pages = gh("api", f"repos/{args.repo}/pulls/{pr['number']}/files", "--paginate", "--slurp")
        pr["files"] = [f["filename"] for page in pages for f in page]
    manifest = {
        "schema_version": 1,
        "repository": args.repo,
        **branches,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "classification_summary": inventory,
        "classification_report_sha256": hashlib.sha256(report_text.encode()).hexdigest(),
        "namespace": namespace,
        "release_prs": release_prs,
        "candidates": [],
    }
    write_json(args.output / "manifest.json", manifest)
    with ThreadPoolExecutor(max_workers=args.workers) as pool:
        rows = pool.map(
            lambda row: prepare_one(
                row, args.repo, f"{namespace}/source", f"{namespace}/target",
                namespace, args.output, release_prs,
            ),
            snapshots,
        )
        for row in rows:
            manifest["candidates"].append(row)
            write_json(args.output / "manifest.json", manifest)
    summarize(manifest, args.output)
    print(f"Saved {len(snapshots)} candidates to {args.output / 'manifest.json'}")


if __name__ == "__main__":
    main()
