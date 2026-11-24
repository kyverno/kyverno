#!/usr/bin/env python3
"""Finds the commit that broke an E2E test that fails on every Kubernetes version.

The Tests bisect workflow (.github/workflows/tests-bisect.yaml) uses it. Like
report.py, it uses the Python 3 standard library only.

  bisect.py select DIR [--max-tests N]
      Reads the e2e-reports-* artifacts of a failed Tests run in DIR and prints
      a JSON object that says whether to bisect and, if so, the target: the
      suite, variant, Kubernetes version, and the failing tests to run. It
      picks only tests that fail on every version they ran on.

  bisect.py run --base SHA --head SHA --suite SUITE [--variant VARIANT]
                --k8s VERSION --tests-json JSON --out FILE
                [--max-steps N] [--step-timeout SECONDS] [--deadline SECONDS]
      Run from the root of a full clone. Checks out the head first and runs
      only the target tests on it with make test-e2e-local. If they don't fail
      there, it stops: the failure doesn't reproduce. Otherwise it binary
      searches the first-parent commits between the base, the last green
      commit, and the head. Writes the outcome to FILE as JSON.

  bisect.py comment --result FILE --plan JSON [--prs FILE] [--run-url URL]
      Prints the Markdown comment for the tracking issue. --prs is a JSON file
      of the pull requests of the first bad commit, as the GitHub API returns
      them.

A commit is bad when one of the target tests fails, and good when they all
pass or don't exist yet at the commit (no chainsaw-test.yaml under the suite
has their name). It's skipped otherwise: when no chainsaw report was written,
for example because the build or the cluster setup failed, or when a target
test was skipped or is missing from the report.
"""

import argparse
import importlib.util
import json
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
_spec = importlib.util.spec_from_file_location("report", HERE / "report.py")
report = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(report)

GOOD, BAD, SKIP = "good", "bad", "skip"
MAX_TESTS = 5
# Characters with a meaning in a Go regular expression.
_REGEX_META = re.compile(r"([\\.+*?()|\[\]{}^$])")


# --- select ------------------------------------------------------------------

def test_regex(names):
    """Returns a chainsaw test-name regex that matches exactly these tests."""
    return "^(" + "|".join(_REGEX_META.sub(r"\\\1", n) for n in sorted(names)) + ")$"


def select(results, max_tests=MAX_TESTS):
    """Chooses what to bisect from the collected results of a run.

    Returns a dict with "bisect" (bool) and "reason". When bisect is true, it
    also has the target "suite", "variant", "k8s", "tests", and "regex", and
    "others": the tests that fail on every version but belong to another
    suite, which aren't bisected.
    """
    by_suite = {}
    for result in results:
        by_suite.setdefault((result["suite"], result["variant"]), []).append(result)
    failing, partial = [], 0
    for (suite, variant), subset in sorted(by_suite.items()):
        for row in report.group(subset):
            if row["scope"] == "all versions":
                failing.append({**row, "suite_name": suite, "variant": variant})
            else:
                partial += 1
    if not failing:
        reason = ("no test fails on every Kubernetes version it ran on"
                  if partial else "no chainsaw test failed")
        return {"bisect": False, "reason": reason}
    if len(failing) > max_tests:
        return {"bisect": False,
                "reason": f"{len(failing)} tests fail on every version, more than the limit of {max_tests}"}
    if any("/" in row["test"] for row in failing):
        return {"bisect": False, "reason": "a failing test name contains a slash"}

    # Bisect one suite: the one with the most failing tests, then the one that
    # failed on the most versions. The others are listed, since a single
    # change often breaks several suites.
    groups = {}
    for row in failing:
        groups.setdefault((row["suite_name"], row["variant"]), []).append(row)
    (suite, variant), rows = min(groups.items(), key=lambda item: (
        -len(item[1]), -max(len(row["failed"]) for row in item[1]), item[0]))
    # The latest version that every test of the suite failed on.
    common = set.intersection(*(set(row["failed"]) for row in rows))
    if not common:
        common = set(rows[0]["failed"])
        rows = [row for row in rows if set(row["failed"]) & common]
    k8s = max(common, key=report.version_key)
    rows = [row for row in rows if k8s in row["failed"]]
    tests = sorted(row["test"] for row in rows)
    others = sorted(f"{row['test']} ({row['suite']})" for row in failing if row not in rows)
    return {
        "bisect": True,
        "reason": f"{len(tests)} tests of {report.suite_label(suite, variant)} fail on every version",
        "suite": suite, "variant": variant, "k8s": k8s,
        "tests": tests, "regex": test_regex(tests), "others": others,
    }


# --- bisection ---------------------------------------------------------------

def next_probe(count, verdicts):
    """Returns the index of the next commit to test, or None when done.

    The commits are indexed oldest first; the base is the good commit before
    index 0 and the head is index count - 1. verdicts maps tested indexes to
    good, bad, or skip.
    """
    low, high = bounds(count, verdicts)
    if high is None:
        return None
    untested = [i for i in range(low + 1, high) if i not in verdicts]
    if not untested:
        return None
    # The untested commit closest to the middle of the open range.
    middle = (low + high) / 2
    return min(untested, key=lambda i: (abs(i - middle), i))


def bounds(count, verdicts):
    """Returns (last good index, first bad index after it), or high None."""
    bad = sorted(i for i, v in verdicts.items() if v == BAD)
    if not bad:
        return -1, None
    high = bad[0]
    low = max([i for i, v in verdicts.items() if v == GOOD and i < high], default=-1)
    return low, high


def outcome(commits, verdicts):
    """Returns the outcome dict once next_probe returns None or time runs out."""
    head = len(commits) - 1
    if verdicts.get(head) == SKIP:
        return {"outcome": "inconclusive", "suspects": []}
    if head in verdicts and verdicts[head] != BAD:
        return {"outcome": "not-reproducible", "suspects": []}
    low, high = bounds(len(commits), verdicts)
    if high is None:
        return {"outcome": "inconclusive", "suspects": []}
    suspects = commits[low + 1:high + 1]
    if len(suspects) == 1:
        return {"outcome": "found", "suspects": suspects}
    return {"outcome": "range", "suspects": suspects}


def bisect(commits, probe, max_steps, deadline=None, clock=time.monotonic):
    """Runs the bisection. probe(sha) returns good, bad, or skip.

    Tests the head first and stops when it isn't bad. Returns the outcome
    dict with the "steps" taken.
    """
    verdicts, steps = {}, []
    index = len(commits) - 1
    while index is not None:
        if len(steps) >= max_steps or (deadline is not None and clock() >= deadline):
            break
        started = clock()
        verdict = probe(commits[index])
        verdicts[index] = verdict
        steps.append({"sha": commits[index], "verdict": verdict, "seconds": round(clock() - started)})
        if index == len(commits) - 1 and verdict != BAD:
            break
        index = next_probe(len(commits), verdicts)
    result = outcome(commits, verdicts)
    if index is not None and result["outcome"] == "range":
        result["outcome"] = "budget"
    result["steps"] = steps
    return result


# The metadata.name of a chainsaw-test.yaml file. test_bisect.py checks that it
# finds the name of every test in test/conformance/chainsaw.
_TEST_NAME = re.compile(r"^metadata:[ \t]*\n(?:[ \t]+.*\n)*?[ \t]+name:[ \t]*[\"']?([^\"'\s#]+)", re.M)


def test_names(suite_dir):
    """Returns the names of the chainsaw tests under suite_dir."""
    names = set()
    for path in Path(suite_dir).rglob("chainsaw-test.y*ml"):
        match = _TEST_NAME.search(path.read_text(errors="replace"))
        if match:
            names.add(match.group(1))
    return names


def verdict_of(report_dir, tests, existing=frozenset()):
    """Classifies a commit from the chainsaw JSON report in report_dir.

    bad: a target test failed. skip: there's no report, a target test was
    skipped, or a target test exists at the commit (it's in existing) but
    isn't in the report. good: every target test passed or doesn't exist yet,
    so the commit can't have broken it.
    """
    path = Path(report_dir) / report.REPORT
    if not path.is_file():
        return SKIP
    try:
        data = json.loads(path.read_text())
    except ValueError:
        return SKIP
    statuses = {t.get("name"): report.status(t) for t in data.get("tests") or [] if t.get("name") in tests}
    if "failed" in statuses.values():
        return BAD
    if "skipped" in statuses.values():
        return SKIP
    if any(name not in statuses and name in existing for name in tests):
        return SKIP
    return GOOD


def make_probe(args, log=sys.stderr, runner=subprocess.run, root="."):
    """Returns a probe that runs the target tests at a commit with make.

    runner stands in for subprocess.run, and root for the clone, in the tests.
    """
    tests = json.loads(args.tests_json)
    suite_dir = Path(root) / "test" / "conformance" / "chainsaw" / args.suite

    def probe(sha):
        print(f"::group::Testing {sha}", file=log, flush=True)
        work = Path(tempfile.mkdtemp(prefix="bisect-"))
        try:
            if runner(["git", "checkout", "--force", "--quiet", sha]).returncode != 0:
                print(f"Can't check out {sha}, so it's skipped.", file=log, flush=True)
                return SKIP
            # make expands $ in command-line variables, so it's doubled.
            command = ["timeout", "--kill-after=60", str(args.step_timeout), "make", "test-e2e-local",
                       f"SUITE={args.suite}", f"TEST={test_regex(tests).replace('$', '$$')}", f"K8S={args.k8s}",
                       f"VARIANT={args.variant}", f"E2E_REPORT_DIR={work}"]
            code = runner(command).returncode
            verdict = verdict_of(work, tests, test_names(suite_dir))
            print(f"make exited with {code}; {sha} is {verdict}", file=log, flush=True)
            return verdict
        finally:
            runner(["make", "kind-delete-cluster"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            shutil.rmtree(work, ignore_errors=True)
            print("::endgroup::", file=log, flush=True)

    return probe


def run(args):
    commits = subprocess.run(
        ["git", "rev-list", "--first-parent", "--reverse", f"{args.base}..{args.head}"],
        check=True, capture_output=True, text=True).stdout.split()
    if not commits or commits[-1] != args.head:
        result = {"outcome": "inconclusive", "suspects": [], "steps": [],
                  "error": "the head isn't a first-parent descendant of the base"}
    else:
        deadline = time.monotonic() + args.deadline if args.deadline else None
        result = bisect(commits, make_probe(args), args.max_steps, deadline)
    result["commits"] = len(commits)
    Path(args.out).write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result))


# --- comment -----------------------------------------------------------------

def text(value):
    """Escapes untrusted text for Markdown: no HTML, tables, or mentions."""
    value = str(value).replace("\\", "\\\\").replace("&", "&amp;")
    value = value.replace("<", "&lt;").replace(">", "&gt;").replace("|", "\\|")
    value = value.replace("`", "'").replace("@", "@&#8203;").replace("\n", " ")
    return value


def pr_links(prs):
    merged = [p for p in prs or [] if p.get("merged_at")]
    return ", ".join(f"{p['html_url']} ({text(p.get('title', ''))})" for p in merged)


def comment(result, plan, prs=None, run_url="", repo_url=""):
    """Returns the Markdown comment for the tracking issue."""
    suite = report.suite_label(plan["suite"], plan.get("variant", ""))
    tests = ", ".join(f"`{text(t)}`" for t in plan["tests"])
    def commit(sha):
        return f"[`{sha[:9]}`]({repo_url}/commit/{sha})" if repo_url else f"`{sha[:9]}`"
    lines = ["### Automatic bisection", ""]
    lines.append(f"Ran {tests} of `{text(suite)}` on Kubernetes {text(plan['k8s'])} "
                 f"between the last green commit {commit(plan['base'])} and {commit(plan['head'])}.")
    lines.append("")
    kind = result.get("outcome")
    suspects = result.get("suspects") or []
    if kind == "found":
        links = pr_links(prs)
        lines.append(f"First bad commit: {commit(suspects[0])}" + (f", from {links}." if links else "."))
    elif kind in ("range", "budget"):
        reason = ("some commits couldn't be tested" if kind == "range"
                  else "the bisection ran out of steps or time")
        lines.append(f"The first bad commit is one of these {len(suspects)} commits; {reason}:")
        lines.append("")
        lines += [f"- {commit(s)}" for s in suspects]
    elif kind == "not-reproducible":
        lines.append("The tests passed when run again on the head commit, so the failure isn't "
                     "reproducible this way. It may be flaky or depend on the other tests of the suite.")
    else:
        detail = result.get("error") or "the head commit couldn't be built or tested"
        lines.append(f"The bisection is inconclusive: {text(detail)}.")
    if plan.get("others"):
        lines += ["", "Also failing on every version, but not bisected: "
                  + ", ".join(text(o) for o in plan["others"]) + "."]
    steps = result.get("steps") or []
    if steps:
        lines += ["", "<details><summary>Steps</summary>", "", "| Commit | Result | Minutes |",
                  "| --- | --- | --- |"]
        lines += [f"| {commit(s['sha'])} | {s['verdict']} | {round(s['seconds'] / 60)} |" for s in steps]
        lines += ["", "</details>"]
    if run_url:
        lines += ["", f"Bisection run: {run_url}"]
    return "\n".join(lines) + "\n"


# --- main --------------------------------------------------------------------

def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    commands = parser.add_subparsers(dest="command", required=True)

    command = commands.add_parser("select")
    command.add_argument("dir")
    command.add_argument("--max-tests", type=int, default=MAX_TESTS)
    command.set_defaults(run=lambda a: print(json.dumps(select(report.collect(a.dir)[0], a.max_tests))))

    command = commands.add_parser("run")
    command.add_argument("--base", required=True)
    command.add_argument("--head", required=True)
    command.add_argument("--suite", required=True)
    command.add_argument("--variant", default="")
    command.add_argument("--k8s", required=True)
    command.add_argument("--tests-json", required=True)
    command.add_argument("--out", required=True)
    command.add_argument("--max-steps", type=int, default=6)
    command.add_argument("--step-timeout", type=int, default=2400)
    command.add_argument("--deadline", type=int, default=0, help="seconds before no new step starts")
    command.set_defaults(run=run)

    command = commands.add_parser("comment")
    command.add_argument("--result", required=True)
    command.add_argument("--plan", required=True)
    command.add_argument("--prs", default="")
    command.add_argument("--run-url", default="")
    command.add_argument("--repo-url", default="")
    command.set_defaults(run=lambda a: sys.stdout.write(comment(
        json.loads(Path(a.result).read_text()), json.loads(a.plan),
        json.loads(Path(a.prs).read_text()) if a.prs else [], a.run_url, a.repo_url)))

    args = parser.parse_args(argv)
    args.run(args)


if __name__ == "__main__":
    main()
