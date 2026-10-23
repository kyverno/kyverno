#!/usr/bin/env python3
"""Opens one Kyverno issue for each Pod Security Admission (PSA) minor release that needs CEL policy work.

The script reads the kubernetes-1.<minor>.0 tags of the read-only PSA mirror,
compares each minor release after the baseline in scripts/psa-release-issues.yaml
with the previous minor release, maps the changes to the PSS CEL templates, and
opens an issue unless one exists. Patches and pre-releases are skipped. It stores
no state. See docs/dev/psa-release-issues/README.md for the rules.

Exit codes: 0 when every release was handled, 1 on an error.
"""

import argparse
import hashlib
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[1]
CONFIG_NAME = "scripts/psa-release-issues.yaml"
CONFIG_FILE = ROOT / CONFIG_NAME
TEMPLATES_DIR = "charts/kyverno-policies/templates"
HELPERS_FILE = f"{TEMPLATES_DIR}/_helpers.tpl"
CHART_FILE = "charts/kyverno-policies/Chart.yaml"
CLI_TESTS_DIR = "test/cli/test-validating-policy"
DEFAULT_REPOSITORY = "https://github.com/kubernetes/pod-security-admission"
STAGING_PATH = "staging/src/k8s.io/pod-security-admission"
LABEL = "psa-cel-update"
MANUAL_LABEL = "needs-manual-review"
LABEL_INFO = {
    LABEL: ("0E8A16", "A Pod Security Admission release that may need PSS CEL policy updates"),
    MANUAL_LABEL: ("D93F0B", "Changes the PSA release check couldn't map with confidence"),
    "area/pss": ("607D8B", "Pod Security Standards"),
}
# Groups that belong to no single check.
UNMAPPED_GROUPS = ("shared", "api", "test-framework")
MAX_ISSUES_PER_RUN = 5
MAX_BODY_CHARS = 65000
MAX_LISTED_COMMITS = 50

# Only minor releases. Patches and pre-releases are skipped.
TAG_RE = re.compile(r"^kubernetes-1\.(\d+)\.0$")
FIXTURE_RE = re.compile(r"^test/testdata/(baseline|restricted)/v1\.(\d+)/(pass|fail)/([^/]+)\.yaml$")
RELEVANT_PATHS = ["policy", "api", "test"]
# Changes that make a release relevant. Test code is listed only as context.
RELEVANT_KINDS = {"check", "shared", "api", "fixture"}


class ReleaseError(Exception):
    """An error that stops the check of one release, or of the whole run."""


def log(message):
    print(f"[psa-release-issues] {message}", file=sys.stderr)


def run_command(args):
    # Git must never wait for a password: a missing or private repository has
    # to fail at once, not hang.
    env = dict(os.environ, GIT_TERMINAL_PROMPT="0")
    try:
        # gh and git print UTF-8 whatever the locale, and issue text can hold any character.
        return subprocess.run(args, capture_output=True, text=True, encoding="utf-8", errors="replace", env=env)
    except OSError as err:
        raise ReleaseError(f"can't run {args[0]}: {err}") from err


def git(git_bin, repo, *args):
    """Runs git in repo and returns stdout, or raises ReleaseError."""
    result = run_command([git_bin, "-C", str(repo), *args])
    if result.returncode != 0:
        raise ReleaseError(f"git {' '.join(args[:3])} failed: {result.stderr.strip()[-500:]}")
    return result.stdout


def check_id_of(path, content):
    """Returns the check ID that a policy/check_*.go file declares.

    A check sets its ID inline (ID: "sysctls") or through a CheckID constant.
    Without either, the ID comes from the file name.
    """
    match = re.search(r'\bID:\s*"([^"]+)"', content) or re.search(r'CheckID\s*=\s*"([^"]+)"', content)
    return match.group(1) if match else path.rsplit("/", 1)[-1][len("check_"):-len(".go")]


def fixture_check_id(fixture):
    """Returns the lowercased check ID of a fixture name, such as "sysctls" for "sysctls1"."""
    return re.sub(r"\d+$", "", fixture).lower()


def load_config(path):
    """Reads and validates the baseline and the check-to-policy map."""
    data = yaml.safe_load(Path(path).read_text(encoding="utf-8")) or {}
    if not isinstance(data, dict):
        raise ReleaseError(f"{path}: must be a map with since and checks")
    since = data.get("since")
    if not isinstance(since, str) or not TAG_RE.match(since):
        raise ReleaseError(f"{path}: since must be a kubernetes-1.<minor>.0 tag")
    checks = data.get("checks")
    if not isinstance(checks, dict) or not checks:
        raise ReleaseError(f"{path}: checks must be a non-empty map")
    for check_id, policy in checks.items():
        if not isinstance(check_id, str) or not isinstance(policy, str) or not policy:
            raise ReleaseError(f"{path}: checks.{check_id} must map to a policy name")
    lowered = [check_id.lower() for check_id in checks]
    if len(set(lowered)) != len(lowered):
        raise ReleaseError(f"{path}: check IDs must be unique ignoring case")
    return {"since": since, "checks": checks}


class Release:
    """A Kubernetes minor release, named by its kubernetes-1.<minor>.0 tag in the PSA mirror."""

    def __init__(self, tag, date=None):
        match = TAG_RE.match(tag)
        if not match:
            raise ValueError(f"not a minor release tag: {tag}")
        self.tag = tag
        self.date = date
        self.minor = int(match.group(1))

    @property
    def version(self):
        """The Kubernetes version, for example v1.38.0."""
        return "v" + self.tag[len("kubernetes-"):]

    @property
    def psa_tag(self):
        """The matching v0.* tag in the PSA mirror."""
        return self.version.replace("v1.", "v0.", 1)

    def key(self):
        return self.minor

    def __repr__(self):
        return self.tag


def parse_releases(text):
    """Parses `git for-each-ref` lines of "<tag>\\x1f<ISO date>" into minor releases."""
    releases = []
    for line in text.splitlines():
        tag, _, date = line.partition("\x1f")
        if TAG_RE.match(tag):
            releases.append(Release(tag, datetime.fromisoformat(date) if date else None))
    return sorted(releases, key=Release.key)


def previous_release(release, releases):
    """Returns v1.<minor - 1>.0 to compare with, or None."""
    return next((r for r in releases if r.minor == release.minor - 1), None)


def fetch_releases(git_bin, repository, dest):
    """Fetches the mirror's kubernetes-1.* tags into a blobless clone and returns the minor releases."""
    dest.mkdir(parents=True)
    git(git_bin, dest, "init", "-q")
    git(git_bin, dest, "remote", "add", "origin", repository)
    result = run_command([git_bin, "-C", str(dest), "fetch", "-q", "--filter=blob:none", "--no-tags",
                          "origin", "+refs/tags/kubernetes-1.*:refs/tags/kubernetes-1.*"])
    if result.returncode != 0:
        lines = result.stderr.strip().splitlines()
        raise ReleaseError(f"can't fetch the kubernetes-1.* tags from {repository}: "
                           f"{lines[-1] if lines else 'no output'}")
    refs = git(git_bin, dest, "for-each-ref", "--format=%(refname:short)%1f%(creatordate:iso-strict)",
               "refs/tags/kubernetes-1.*")
    return parse_releases(refs)


class Analysis:
    """The relevant upstream changes between two releases."""

    def __init__(self, release, previous):
        self.release = release
        self.previous = previous
        # group -> list of {"path", "status", "kind", "level", "version", "fixture"}
        self.groups = {}
        self.new_versions = []
        self.copied = 0
        self.signals = {}
        self.commits = []
        # Upstream check IDs missing from the config, and config IDs missing upstream.
        self.unmapped_checks = []
        self.stale_checks = []

    def add(self, group, change):
        self.groups.setdefault(group, []).append(change)

    @property
    def relevant(self):
        """True for any change other than fixtures copied unchanged to a new version.

        Test-only changes count too: they get an issue that asks a person to
        confirm them, never "no action needed".
        """
        return bool(self.groups or self.unmapped_checks or self.stale_checks)


def tree_blobs(git_bin, repo, ref, path):
    """Returns {relative path: blob SHA} for the files under path at ref."""
    result = run_command([git_bin, "-C", str(repo), "ls-tree", "-r", ref, "--", path])
    blobs = {}
    for line in result.stdout.splitlines() if result.returncode == 0 else []:
        meta, _, name = line.partition("\t")
        blobs[name[len(path) + 1:]] = meta.split()[2]
    return blobs


def show(git_bin, repo, ref, path):
    result = run_command([git_bin, "-C", str(repo), "show", f"{ref}:{path}"])
    return result.stdout if result.returncode == 0 else ""


def check_group(git_bin, repo, release, previous, path, lowered):
    """Returns (group, kind) for a changed path."""
    name = path.rsplit("/", 1)[-1]
    if path.startswith("policy/"):
        is_test = name.endswith("_test.go")
        source = path[:-len("_test.go")] + ".go" if is_test else path
        if not name.startswith("check_"):
            return "shared", "shared-test" if is_test else "shared"
        content = show(git_bin, repo, release.tag, source) or show(git_bin, repo, previous.tag, source)
        return check_id_of(source, content), "check-test" if is_test else "check"
    if path.startswith("api/"):
        return "api", "api-test" if name.endswith("_test.go") else "api"
    match = re.fullmatch(r"test/fixtures_(\w+)\.go", path)
    # fixtures_test.go tests the generator itself, not one check.
    if match and not name.endswith("_test.go"):
        return lowered.get(match.group(1).lower(), match.group(1)), "fixture-generator"
    return "test-framework", "test-framework"


def upstream_check_ids(git_bin, repo, ref):
    """Returns {check ID: path} for every non-test policy/check_*.go file at ref."""
    names = git(git_bin, repo, "ls-tree", "--name-only", ref, "policy/").split()
    paths = [n for n in names if re.fullmatch(r"policy/check_\w+\.go", n) and not n.endswith("_test.go")]
    return {check_id_of(p, show(git_bin, repo, ref, p)): p for p in paths}


def analyze(git_bin, repo, release, previous, checks):
    """Compares two releases and returns an Analysis."""
    analysis = Analysis(release, previous)
    lowered = {check_id.lower(): check_id for check_id in checks}
    out = git(git_bin, repo, "diff", "--name-status", "--no-renames", previous.tag, release.tag,
                   "--", *RELEVANT_PATHS)
    changes = [line.split("\t", 1) for line in out.splitlines() if "\t" in line]

    # A new fixture directory is mostly a copy of the previous version. Keep
    # only the files that differ from it.
    old_dirs = {(m.group(1), m.group(2)) for m in
                (FIXTURE_RE.match(f"test/testdata/{p}") for p in tree_blobs(git_bin, repo, previous.tag, "test/testdata"))
                if m}
    # (level, minor) -> (blobs of v1.<minor - 1>, blobs of v1.<minor>), both at the release.
    new_dirs = {}
    for status, path in changes:
        match = FIXTURE_RE.match(path)
        if match and (match.group(1), match.group(2)) not in old_dirs and (match.group(1), match.group(2)) not in new_dirs:
            level, minor = match.group(1), int(match.group(2))
            new_dirs[(level, match.group(2))] = (
                tree_blobs(git_bin, repo, release.tag, f"test/testdata/{level}/v1.{minor - 1}"),
                tree_blobs(git_bin, repo, release.tag, f"test/testdata/{level}/v1.{minor}"),
            )
    analysis.new_versions = sorted((f"{level}/v1.{minor}" for level, minor in new_dirs),
                                   key=lambda v: (v.split("/")[0], int(v.split(".")[-1])))

    # A fixture that the new version drops isn't in the diff, so list it as deleted.
    for (level, minor), (before, after) in new_dirs.items():
        changes += [["D", f"test/testdata/{level}/v1.{minor}/{r}"] for r in sorted(set(before) - set(after))]

    for status, path in changes:
        match = FIXTURE_RE.match(path)
        if match:
            level, minor, kind, fixture = match.groups()
            if (level, minor) in new_dirs:
                before, after = new_dirs[(level, minor)]
                relative = path.split(f"/v1.{minor}/", 1)[1]
                if before.get(relative) == after.get(relative):
                    analysis.copied += 1
                    continue
                # Compared with the previous version, not with the previous release.
                status = "D" if relative not in after else "M" if relative in before else "A"
            group = "shared" if fixture.startswith("base") else lowered.get(fixture_check_id(fixture),
                                                                             fixture_check_id(fixture))
            analysis.add(group, {"path": path, "status": status, "kind": "fixture", "level": level,
                                 "version": f"1.{minor}", "fixture": fixture, "result": kind})
            continue
        # Any other file, such as test/testdata/README.md, counts as test code, so it's never dropped silently.
        group, kind = check_group(git_bin, repo, release, previous, path, lowered)
        analysis.add(group, {"path": path, "status": status, "kind": kind})

    # The set of upstream checks can only change when a check file changes.
    if any(c["kind"] == "check" for items in analysis.groups.values() for c in items):
        upstream = upstream_check_ids(git_bin, repo, release.tag)
        analysis.unmapped_checks = sorted(set(upstream) - set(checks))
        analysis.stale_checks = sorted(set(checks) - set(upstream))

    for group, items in analysis.groups.items():
        code = [c["path"] for c in items if c["kind"] in ("check", "shared", "api")]
        if code:
            diff = git(git_bin, repo, "diff", previous.tag, release.tag, "--", *code)
            analysis.signals[group] = diff_signals(diff, items, set(checks) | {group})
    analysis.commits = upstream_commits(git_bin, repo, previous, release)
    return analysis


def strip_comments(code, in_comment):
    """Removes Go comments from a line. Returns (code, in_comment) for the next line."""
    kept = ""
    while code:
        if in_comment:
            end = code.find("*/")
            if end < 0:
                return kept, True
            code, in_comment = code[end + 2:], False
        else:
            start = code.find("/*")
            if start < 0:
                kept, code = kept + code, ""
            else:
                kept, code, in_comment = kept + code[:start], code[start + 2:], True
    return kept.split("//", 1)[0], in_comment


def diff_signals(diff, items, check_ids=()):
    """Returns behavior hints from a Go diff: version gates and changed string values.

    Comments, import paths, and check IDs aren't values, so they're left out.
    """
    signals = {"new": [], "removed": [], "gates": [], "removed_gates": [], "added_values": [], "removed_values": []}
    for change in items:
        if change["kind"] == "check" and change["status"] == "A":
            signals["new"].append(change["path"])
        if change["kind"] == "check" and change["status"] == "D":
            signals["removed"].append(change["path"])
    in_comment = False
    for line in diff.splitlines():
        if line.startswith("diff --git"):
            in_comment = False
        if line.startswith(("+++", "---")) or line[:1] not in ("+", "-", " "):
            continue
        # Context lines only track whether a block comment is open.
        code, in_comment = strip_comments(line[1:], in_comment)
        if line[0] == " ":
            continue
        # Import paths, such as "slices" or corev1 "k8s.io/api/core/v1", aren't values.
        if re.fullmatch(r'\s*(import\s+)?(\w+\s+)?"[^"]+"\s*', code):
            continue
        gates = [f"1.{m}" for m in re.findall(r"MajorMinorVersion\(\s*1\s*,\s*(\d+)\s*\)", code)]
        signals["gates" if line[0] == "+" else "removed_gates"] += gates
        values = [v for v in re.findall(r'"([^"\s]+)"', code) if v not in check_ids]
        signals["added_values" if line[0] == "+" else "removed_values"] += values
    added, removed = set(signals["added_values"]), set(signals["removed_values"])
    # A value on both sides moved rather than changed.
    signals["added_values"] = sorted(added - removed)
    signals["removed_values"] = sorted(removed - added)
    # Likewise for a version gate on both sides.
    added, removed = set(signals["gates"]), set(signals["removed_gates"])
    signals["gates"] = sorted(added - removed, key=lambda v: int(v.split(".")[1]))
    signals["removed_gates"] = sorted(removed - added, key=lambda v: int(v.split(".")[1]))
    return signals


def upstream_commits(git_bin, repo, previous, release):
    """Returns [(sha, subject, kubernetes commit or None)] for commits that touch relevant paths."""
    out = git(git_bin, repo, "log", "--no-merges", f"--max-count={MAX_LISTED_COMMITS + 1}",
                   "--format=%H%x1f%s%x1f%b%x1e", f"{previous.tag}..{release.tag}", "--", *RELEVANT_PATHS)
    commits = []
    for record in out.split("\x1e"):
        if not record.strip():
            continue
        sha, subject, body = (record.strip("\n").split("\x1f") + ["", ""])[:3]
        trailer = re.search(r"^Kubernetes-commit:\s*([0-9a-f]{40})\s*$", body, re.MULTILINE)
        commits.append((sha, subject, trailer.group(1) if trailer else None))
    return commits


class Chart:
    """Looks up the Kyverno files that implement a PSS policy."""

    def __init__(self, root):
        self.root = Path(root)
        self.templates = {p.name[:-len(".cel.yaml")]: p for p in
                          sorted((self.root / TEMPLATES_DIR).glob("*/*.cel.yaml"))}
        # A helper's body runs to the next define. Its own {{ end }} can't mark
        # the end, because nested if blocks also end with {{ end }}.
        parts = re.split(r'\{\{-?\s*define\s+"kyverno-policies\.(\w+)"\s*-?\}\}', self.read(HELPERS_FILE))
        self.helpers = dict(zip(parts[1::2], parts[2::2]))
        self.tests = sorted(p for p in (self.root / CLI_TESTS_DIR).glob("pss-*") if p.is_dir())
        usage = {name: sum(1 for t in self.templates.values() if self.includes(t, name)) for name in self.helpers}
        # A helper used by most templates, or one that selects a profile, is
        # plumbing, not check logic.
        self.check_helpers = {name for name, count in usage.items()
                              if 0 < count < len(self.templates) / 2 and not name.startswith("podSecurity")}
        # The oldest Kubernetes version the chart supports, such as 1.25, or None.
        match = re.search(r'^kubeVersion:\s*"?>=\s*(\d+\.\d+)', self.read(CHART_FILE), re.MULTILINE)
        self.min_kube = match.group(1) if match else None

    def read(self, relative):
        path = self.root / relative
        return path.read_text(encoding="utf-8") if path.is_file() else ""

    @staticmethod
    def includes(template, helper):
        return f'"kyverno-policies.{helper}"' in template.read_text(encoding="utf-8")

    def relative(self, path):
        return path.relative_to(self.root).as_posix()

    def lookup(self, policy):
        """Returns the template, check helpers, version gates, and tests for a policy."""
        template = self.templates.get(policy)
        if not template:
            return None
        text = template.read_text(encoding="utf-8")
        helpers = sorted(h for h in self.check_helpers if self.includes(template, h))
        sources = [text] + [self.helpers[h] for h in helpers]
        gates = sorted({m for s in sources for m in re.findall(r'semverCompare\s+"\s*>=\s*1\.(\d+)', s)}, key=int)
        tests = [self.relative(t) for t in self.tests
                 # disallow-proc-mount must not match disallow-proc-mount-strict.
                 if any(re.search(rf"(?<![\w-]){re.escape(policy)}(?![\w-])", f.read_text(encoding="utf-8"))
                        for f in t.glob("*.yaml"))]
        return {"template": self.relative(template), "level": template.parent.name, "helpers": helpers,
                "gates": [f"1.{g}" for g in gates], "tests": tests, "sources": "\n".join(sources)}


def unsure(subject, problem, action, config=False):
    """Returns an uncertain change: what the automation couldn't determine, and what a person should do.

    config is True when resolving it means editing `checks` in the config.
    """
    return {"subject": subject, "problem": problem, "action": action, "config": config}


def map_changes(analysis, mapping, chart):
    """Maps each changed group to Kyverno files. Returns (mapped, uncertain).

    mapped is a list of dicts per group. uncertain lists, as unsure() dicts,
    the changes that can't be tied to the Kyverno policies with confidence; any
    of them marks the issue for manual review.
    """
    mapped, uncertain = [], []
    for check_id in analysis.unmapped_checks:
        uncertain.append(unsure(check_id, f"Upstream declares this check, but `checks` in `{CONFIG_NAME}` doesn't map "
                                "it, so no Kyverno policy is identified for it.",
                                f"Map {code(check_id)} to a policy in `checks`, or decide whether Kyverno needs a new "
                                "policy for it.", config=True))
    for check_id in analysis.stale_checks:
        uncertain.append(unsure(check_id, f"`checks` in `{CONFIG_NAME}` maps this check, but upstream no longer "
                                "declares it. It may be renamed or removed.",
                                f"Find what replaced {code(check_id)} upstream, then rename or remove its entry in "
                                "`checks`.", config=True))
    for group in sorted(analysis.groups):
        items = analysis.groups[group]
        signals = analysis.signals.get(group, {})
        entry = {"group": group, "policy": mapping["checks"].get(group), "items": items, "signals": signals,
                 "files": None}
        mapped.append(entry)
        if not any(c["kind"] in RELEVANT_KINDS for c in items):
            # Test code can encode a behavior change that the fixtures don't
            # show yet, so a person confirms it.
            entry["test_only"] = True
            continue
        if group in UNMAPPED_GROUPS:
            entry["policy"] = None
            where = "`api/`" if group == "api" else "shared `policy/` code or base fixtures"
            changes = "`api/` changes" if group == "api" else "shared `policy/` code and base fixture changes"
            uncertain.append(unsure(group, f"Changes in {where} can affect every check, so they map to no single "
                                    "policy.",
                                    f"Read the {changes} in the comparison, and decide which of the "
                                    f"{len(chart.templates)} PSS CEL templates they affect, if any."))
            continue
        policy = entry["policy"]
        if not policy:
            # The upstream check scan already reported a declared, unmapped check.
            if group not in analysis.unmapped_checks:
                uncertain.append(unsure(group, f"This check has no entry in `checks` in `{CONFIG_NAME}`. It may be a "
                                        "new upstream check or a renamed one.",
                                        f"Map {code(group)} to a policy in `checks`, or confirm that upstream removed "
                                        "it.", config=True))
            continue
        files = entry["files"] = chart.lookup(policy)
        if not files:
            uncertain.append(unsure(group, f"This check maps to policy {code(policy)}, but no `{policy}.cel.yaml` "
                                    f"template exists under `{TEMPLATES_DIR}`.",
                                    f"Fix the policy name for {code(group)} in `checks`, or restore the template.",
                                    config=True))
            continue
        added, gates = signals.get("added_values", []), signals.get("gates", [])
        # A name match only: under which version gate the template has a value isn't checked.
        entry["missing_values"] = [v for v in added if not has_value(files["sources"], v)]
        entry["present_values"] = [v for v in added if has_value(files["sources"], v)]
        entry["missing_gates"] = [g for g in gates if g not in files["gates"]]
        # A value that upstream removed but the template still has may need work, so it isn't handled.
        entry["kept_values"] = [v for v in signals.get("removed_values", []) if has_value(files["sources"], v)]
        removed_gates = signals.get("removed_gates", [])
        entry["handled"] = (bool(added or gates) and not entry["missing_values"] and not entry["missing_gates"]
                            and not entry["kept_values"] and not removed_gates)
        if signals.get("new") and signals.get("removed"):
            uncertain.append(unsure(group, "A check file for this check was renamed or split, so its check ID or level "
                                    "may have moved.",
                                    f"Confirm that {code(group)} still maps to {code(policy)}, and update `checks` if "
                                    "it moved.", config=True))
        if removed_gates:
            # A removed or moved gate changes behavior for some versions in a way the diff can't show.
            uncertain.append(unsure(group, "Upstream removed the version gate "
                                    f"{listed('>=' + g for g in removed_gates)}, so the check's behavior changes "
                                    "for some Kubernetes versions.",
                                    f"Compare the version gates in {code(files['template'])} with the upstream "
                                    f"{code(group)} check, and update its `semverCompare` gates to match."))
        code_changed = any(c["kind"] == "check" for c in items)
        if not entry["handled"] and code_changed and not (gates or removed_gates or added
                                                           or signals.get("removed_values")):
            # The heuristics found nothing, which doesn't mean nothing changed.
            entry["no_signals"] = True
            uncertain.append(unsure(group, "The check code changed, but the diff shows no version gate or value "
                                    "change, so the automation can't tell whether behavior changed.",
                                    f"Read the {code(group)} code changes in the comparison, and decide whether "
                                    f"{code(policy)} needs an update."))
    return mapped, uncertain


def has_value(sources, value):
    # A whole quoted literal, so net.ipv4.tcp doesn't match "net.ipv4.tcp_rmem".
    return re.search(f"[\"']{re.escape(value)}[\"']", sources) is not None


def code(value):
    # Upstream text can hold backticks; replacing them keeps it inside the code span.
    return f"`{str(value).replace('`', chr(39))}`"


def listed(values, empty="none"):
    return ", ".join(code(v) for v in values) or empty


def inert(text):
    """Puts upstream text in a code span, so @mentions and #refs don't ping or link anyone."""
    return f"`{text.replace('`', chr(39))}`" if text else ""


def utc(date):
    return date.astimezone(timezone.utc).strftime("%Y-%m-%d %H:%M UTC") if date else "unknown"


def plural(count, word):
    return f"{count} {word}" + ("" if count == 1 else "s")


def sysctls_versions(entry, minor):
    """Returns the versions, such as ["1.38"], that scripts/verify-policies-sysctls.sh must check for a change.

    The script checks a fixed list of names at each version it lists, so it
    covers a change only when it lists the version where upstream gates it.
    """
    return entry["signals"].get("gates") or [minor]


def needs_update(entry):
    """True for a mapped policy whose template lacks an upstream change, or whose upstream fixtures changed."""
    return bool(entry["files"]) and not (entry.get("test_only") or entry.get("handled") or entry.get("no_signals"))


def issue_kind(mapped, uncertain):
    """Returns "manual", "update", or "confirm" for an issue."""
    if uncertain:
        return "manual"
    if any(needs_update(e) for e in mapped):
        return "update"
    return "confirm"


KIND_NAMES = {"update": "Update required", "confirm": "Confirmation required", "manual": "Manual review required"}
CODE_KINDS = ("check", "shared", "api")
SYSCTLS_POLICY = "restrict-sysctls"
SYSCTLS_SCRIPT = "scripts/verify-policies-sysctls.sh"


def category(change):
    """Returns "code", "fixtures", or "tests" for a changed file."""
    if change["kind"] in CODE_KINDS:
        return "code"
    return "fixtures" if change["kind"] == "fixture" else "tests"


def code_hints(entry):
    """Returns what the Go diff shows for a group's code: file changes, version gates, and values."""
    signals, hints = entry["signals"], []
    if signals.get("new") and signals.get("removed"):
        hints.append("check file renamed or split")
    elif signals.get("new"):
        hints.append("new check file")
    elif signals.get("removed"):
        hints.append("check file removed")
    if signals.get("gates"):
        hints.append("version gate " + listed(">=" + g for g in signals["gates"]))
    if signals.get("removed_gates"):
        hints.append("removed version gate " + listed(">=" + g for g in signals["removed_gates"]))
    if signals.get("added_values"):
        hints.append("added " + listed(signals["added_values"][:10]))
    if signals.get("removed_values"):
        hints.append("removed " + listed(signals["removed_values"][:10]))
    return "; ".join(hints) or "no version gate or value change found"


def group_name(group):
    return {"shared": "shared `policy/` code", "api": "`api/`"}.get(group, code(group))


def render_issue(analysis, mapped, uncertain, repository, min_kube=None):
    """Returns (title, body) for a release with upstream changes.

    The body keeps apart what the upstream diff shows (What changed) and what
    the automation mapped, couldn't map, or only infers (Kyverno impact, where
    an inference is labeled as one). Actions and validation follow the issue
    kind, so a confirm issue never asks for policy work. min_kube is the
    oldest Kubernetes version the chart supports, such as "1.25".
    """
    rel, prev = analysis.release, analysis.previous
    base = repository[:-4] if repository.endswith(".git") else repository
    k8s = "https://github.com/kubernetes/kubernetes"
    compare = f"{base}/compare/{prev.tag}...{rel.tag}"
    minor = f"1.{rel.minor}"
    kind = issue_kind(mapped, uncertain)
    title = {
        "manual": f"Pod Security Admission {rel.version}: review the PSS CEL policies (manual review)",
        "update": f"Pod Security Admission {rel.version}: update the PSS CEL policies",
        "confirm": f"Pod Security Admission {rel.version}: confirm the PSS CEL policies need no change",
    }[kind]
    updates = [e for e in mapped if needs_update(e)]
    handled = [e for e in mapped if e.get("handled")]
    test_only = [e["group"] for e in mapped if e.get("test_only")]

    if kind == "update":
        action = (f"Update {listed(e['policy'] for e in updates)} to match upstream, then run the checks under "
                  "**Validation**.")
    elif kind == "confirm":
        reasons = []
        if handled:
            reasons.append("the templates already seem to handle the upstream changes to "
                           + listed(e["group"] for e in handled))
        if test_only:
            reasons.append("only test code changed for " + listed(test_only))
        reasons = reasons or ["the automation found no change that needs an update"]
        action = (f"Confirm that no CEL change is needed, because {' and '.join(reasons)}, and record the outcome in "
                  "this issue.")
    else:
        action = (f"Resolve the {plural(len(uncertain), 'item')} under **Not mapped**, then decide whether a policy "
                  "needs an update.")
    lines = [
        "## Summary", "",
        f"- **Release:** Kubernetes {code(rel.version)}, compared with {code(prev.version)}",
        f"- **Issue type:** {KIND_NAMES[kind]}",
        f"- **Recommended action:** {action}",
        "",
    ]
    if base != DEFAULT_REPOSITORY:
        lines += ["> [!NOTE]",
                  f"> This issue was generated from {code(base)}, not `kubernetes/pod-security-admission`. It may "
                  "describe a test release, and its `kubernetes/kubernetes` links may not exist.", ""]

    lines += [
        "## Release details", "",
        "| Field | Value |",
        "| --- | --- |",
        f"| Release | [{code(rel.version)}]({k8s}/releases/tag/{rel.version}) |",
        f"| Previous release | [{code(prev.version)}]({k8s}/releases/tag/{prev.version}) |",
        f"| Published | {utc(rel.date)} |",
        f"| PSA source | [{code(STAGING_PATH)}]({k8s}/tree/{rel.version}/{STAGING_PATH}) |",
        f"| PSA mirror tags | [{code(rel.tag)}]({base}/tree/{rel.tag}), [{code(rel.psa_tag)}]({base}/tree/{rel.psa_tag}) |",
        f"| Comparison | [{code(prev.tag + '...' + rel.tag)}]({compare}) |",
        "",
    ]

    # What changed: only what the diff shows.
    by_category = {"code": [], "fixtures": [], "tests": []}
    for entry in mapped:
        for name in by_category:
            if any(category(c) == name for c in entry["items"]):
                by_category[name].append(entry)
    lines += ["## What changed", ""]
    if by_category["code"]:
        lines.append("- **Code:**")
        lines += [f"  - {group_name(e['group'])}: {code_hints(e)}" for e in by_category["code"]]
    if by_category["fixtures"]:
        lines.append("- **Fixtures:**")
        for entry in by_category["fixtures"]:
            fixtures = [c for c in entry["items"] if c["kind"] == "fixture"]
            levels = sorted({c["level"] for c in fixtures})
            versions = sorted({c["version"] for c in fixtures}, key=lambda v: int(v.split(".")[1]))
            lines.append(f"  - {group_name(entry['group'])}: {plural(len(fixtures), 'fixture')} for {listed(versions)} "
                         f"({', '.join(levels)})")
    if by_category["tests"]:
        count = sum(1 for e in by_category["tests"] for c in e["items"] if category(c) == "tests")
        lines.append(f"- **Test code:** {plural(count, 'file')} for "
                     f"{listed(e['group'] for e in by_category['tests'])}")
    if analysis.new_versions:
        lines.append(f"- **Left out:** {plural(analysis.copied, 'fixture')} under the new "
                     f"{listed(analysis.new_versions)}, because they're unchanged copies of the previous version.")
    if by_category["code"]:
        lines += ["", "Version gates and values are string matches from the Go diff (`MajorMinorVersion(1, N)` calls "
                  "and string literals), not a semantic analysis."]
    lines.append("")

    # Kyverno impact: the mapping results, and the changes that couldn't be mapped.
    lines += ["## Kyverno impact", ""]
    with_files = [e for e in mapped if e["files"] and not e.get("test_only")]
    for entry in with_files:
        files = entry["files"]
        if entry.get("no_signals"):
            status = "impact unknown, see **Not mapped**"
        elif entry.get("handled"):
            status = ("may already handle the change. This is an inference: every name and version gate that upstream "
                      "added appears in the template or its helpers.")
        else:
            status = "needs an update"
        lines += [
            f"- {code(entry['policy'])} ({files['level']}), mapped from upstream check {code(entry['group'])}: "
            f"{status}",
            f"  - Template: {code(files['template'])}",
            f"  - Check helpers: {listed(files['helpers'])}",
            f"  - Version gates in the template: {listed('>=' + g for g in files['gates'])}",
            f"  - CLI tests: {listed(files['tests'], 'none found')}",
        ]
        missing = entry.get("missing_values", []) + [">=" + g for g in entry.get("missing_gates", [])]
        if entry.get("present_values"):
            lines.append(f"  - Names already in the template or its helpers: {listed(entry['present_values'])}. "
                         "Under which version gate isn't checked.")
        if missing:
            lines.append(f"  - Not in the template or its helpers: {listed(missing)}")
        if entry.get("kept_values"):
            lines.append(f"  - Removed upstream, but still in the template: {listed(entry['kept_values'])}")
    if uncertain:
        if with_files:
            lines.append("")
        lines += ["**Not mapped.** The automation couldn't determine the Kyverno impact of these changes:", ""]
        lines += [f"- {group_name(u['subject'])}: {u['problem']}" for u in uncertain]
    if test_only:
        if with_files or uncertain:
            lines.append("")
        them = "it" if len(test_only) == 1 else "them"
        lines.append(f"**Test code only:** {listed(test_only)}. The automation found no check code or fixture change "
                     f"for {them}, so it lists no CEL template for {them}. That doesn't rule out a behavior change "
                     "that the tests encode.")
    lines.append("")

    lines += ["## Required actions", ""]
    for entry in updates:
        files, signals = entry["files"], entry["signals"]
        lines.append(f"- [ ] Update {code(files['template'])} for the upstream {code(entry['group'])} change:")
        for value in entry["missing_values"]:
            lines.append(f"  - [ ] Handle {code(value)}, which upstream added and the template doesn't contain.")
        for gate in entry["missing_gates"]:
            lines.append(f"  - [ ] Gate the change with `semverCompare \">={gate}.0-0\"`, because upstream changes "
                         f"behavior from {gate}.")
        for value in entry["kept_values"]:
            lines.append(f"  - [ ] Decide whether {code(value)}, which upstream removed, must stay for older "
                         "Kubernetes versions.")
        if entry["policy"] == SYSCTLS_POLICY:
            # Without a line for the new version, make verify-policies-sysctls passes without checking the change.
            checks = " and ".join(f"`check v{v}.0`" for v in sysctls_versions(entry, minor))
            lines.append(f"  - [ ] Update the expected names in `{SYSCTLS_SCRIPT}`, with a {checks} line, so "
                         "`make verify-policies-sysctls` checks this change.")
        if not any(c["kind"] == "check" for c in entry["items"]):
            versions = sorted({c["version"] for c in entry["items"] if c["kind"] == "fixture"},
                              key=lambda v: int(v.split(".")[1]))
            lines.append(f"  - [ ] Compare {code(entry['policy'])} with the changed upstream fixtures for "
                         f"{listed(versions)}, and fix each result that differs.")
        if files["tests"]:
            lines.append(f"  - [ ] Add or update a CLI test under {listed(files['tests'])}.")
        else:
            lines.append(f"  - [ ] Add a CLI test under `{CLI_TESTS_DIR}/`. None was found for this policy.")
    for entry in handled:
        tests = f", and that {listed(entry['files']['tests'])} covers it" if entry["files"]["tests"] else ""
        lines.append(f"- [ ] Confirm that {code(entry['files']['template'])} already matches the upstream "
                     f"{code(entry['group'])} change{tests}.")
    if test_only:
        lines.append(f"- [ ] Read the upstream test changes for {listed(test_only)} in the comparison, and confirm "
                     "that they don't encode a behavior change.")
    lines += [f"- [ ] {u['action']}" for u in uncertain]
    if kind == "confirm":
        lines.append("- [ ] Comment with the outcome: no CEL change is needed, or a linked follow-up issue tracks "
                     "the update.")
    lines.append("")

    def fixture_check(entries, policy):
        # Only the sysctls allow-list has a command that compares the chart with upstream.
        how = ("`make verify-policies-sysctls` checks the sysctls allow-list; compare any other fixture by hand."
               if any(e["policy"] == SYSCTLS_POLICY for e in entries) else
               "No command runs them against the PSS policies, so compare them by hand.")
        # The versions of the changed fixtures, or the release for a code-only change.
        versions = sorted({c["version"] for e in entries for c in e["items"] if c["kind"] == "fixture"},
                          key=lambda v: int(v.split(".")[1])) or [minor]
        paths = " and ".join(f"`test/testdata/*/v{v}/`" for v in versions)
        return (f"- [ ] Each upstream `pass` and `fail` fixture in {paths} for "
                f"{listed(e['group'] for e in entries)} gets the same result from {policy}. {how}")

    def sysctls_check(entry):
        versions = " and ".join(f"`v{v}.0`" for v in sysctls_versions(entry, minor))
        return f"- [ ] `make verify-policies-sysctls` passes, and its output lists {versions}."

    lines += ["## Validation", ""]
    if updates:
        lines.append(fixture_check(updates, "the updated policy"))
        lines.append("- [ ] The PSS CLI tests pass with `make test-cli-local-vpols`.")
        gated = sorted({g for e in updates for g in e["signals"].get("gates", [])}, key=lambda v: int(v.split(".")[1]))
        if gated:
            lines.append(f"- [ ] Behavior that starts at Kubernetes {listed(gated)} is gated with `semverCompare`, "
                         "so older versions keep the old behavior.")
        lines += [sysctls_check(e) for e in updates if e["policy"] == SYSCTLS_POLICY]
        oldest = f"`--kube-version v{min_kube}.0`" if min_kube else "the oldest Kubernetes version the chart supports"
        lines.append(f"- [ ] `helm template charts/kyverno-policies` renders with {oldest} and with "
                     f"`--kube-version v{minor}.0`.")
    for entry in handled:
        lines.append(fixture_check([entry], code(entry["policy"])))
        if entry["policy"] == SYSCTLS_POLICY:
            lines.append(sysctls_check(entry))
    if uncertain:
        lines.append("- [ ] Each item under **Not mapped** is resolved: mapped in `checks`, ruled out, or tracked in a "
                     "follow-up issue.")
        if any(u["config"] for u in uncertain):
            lines.append("- [ ] After you change `checks`, `python3 scripts/test-psa-release-issues.py` passes.")
        if not updates:
            lines += ["", "If the review leads to a policy change, also check that the upstream fixtures get the same "
                      "result, that `make test-cli-local-vpols` passes, and that version-dependent behavior is gated "
                      "with `semverCompare`."]
    if kind == "confirm" and not handled:
        lines.append("None, unless the review finds a behavior change. Then update the policy, and validate it as an "
                     "update.")
    lines.append("")

    lines += ["## Upstream evidence", "", "### Commits", ""]
    if analysis.commits:
        for sha, subject, k8s_sha in analysis.commits[:MAX_LISTED_COMMITS]:
            # Commit links, not PR links: a PR link would add a cross-reference to the upstream PR.
            source = f" ([kubernetes/kubernetes@{k8s_sha[:12]}]({k8s}/commit/{k8s_sha}))" if k8s_sha else ""
            lines.append(f"- [{code(sha[:12])}]({base}/commit/{sha}) {inert(subject)}{source}")
        if len(analysis.commits) > MAX_LISTED_COMMITS:
            lines.append(f"- More commits: see the [comparison]({compare}).")
    else:
        lines.append("None found in `policy/`, `api/`, or `test/`.")

    # The issue carries no diff. Go code files link to their section of the comparison,
    # which GitHub anchors as "diff-" plus the SHA-256 of the path.
    def changed_file(change):
        if category(change) == "code":
            anchor = hashlib.sha256(change["path"].encode()).hexdigest()
            return f"[{code(change['path'])}]({compare}#diff-{anchor}) ({change['status']})"
        return f"{code(change['path'])} ({change['status']})"

    lines += ["", "### Changed files", "", f"See the [full comparison]({compare}).", ""]
    for name, label in (("code", "Code"), ("fixtures", "Fixtures"), ("tests", "Test code")):
        changes = [c for e in mapped for c in e["items"] if category(c) == name]
        if changes:
            lines.append(f"- **{label}:** " + ", ".join(changed_file(c) for c in changes[:30]) +
                         (f", and {len(changes) - 30} more" if len(changes) > 30 else ""))

    lines += ["", "---", "", "_Opened by `.github/workflows/psa-release-issues.yaml`._", "", marker(rel)]
    body = "\n".join(lines)
    if len(body) > MAX_BODY_CHARS:
        body = body[:MAX_BODY_CHARS - 200].rsplit("\n", 1)[0] + "\n\n_Truncated._\n\n" + marker(rel)
    return title, body


def marker(release):
    return f"<!-- psa-cel-update: {release.tag} -->"


def no_action_reason(analysis):
    """Explains a release without changes. Only call it when analysis.relevant is False."""
    if analysis.copied:
        return (f"only {analysis.copied} fixtures copied unchanged to " +
                ", ".join(code(v) for v in analysis.new_versions or ["a new version"]))
    return "no change in `policy/`, `api/`, or `test/`"


class GitHub:
    """A small GitHub REST client on top of `gh api`, limited to issues, issue comments, and labels.

    The only change it makes to an existing issue is closing a duplicate that
    this script opened.
    """

    def __init__(self, repo):
        if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
            raise ReleaseError(f"invalid repository: {repo}")
        self.repo = repo

    def check_allowed(self, method, path, body):
        prefix = f"repos/{re.escape(self.repo)}"
        allowed = (
            # Reading the repository itself, to check that Issues are on.
            method == "GET" and path == f"repos/{self.repo}"
        ) or (
            method in ("GET", "POST") and re.match(rf"^{prefix}/(issues|labels)(\?|/|$)", path)
            and not re.match(rf"^{prefix}/issues/\d+$", path)
        ) or (
            # Closing a duplicate is the only issue update.
            method == "PATCH" and re.fullmatch(rf"{prefix}/issues/\d+", path)
            and set(body or {}) <= {"state", "state_reason"} and (body or {}).get("state") == "closed"
        )
        if not allowed:
            raise ReleaseError(f"refusing {method} {path}")

    def request(self, method, path, body=None, paginate=False):
        self.check_allowed(method, path, body)
        command = ["gh", "api", "-X", method, path, "-H", "Accept: application/vnd.github+json"]
        if paginate:
            command += ["--paginate", "--jq", ".[] | @json"]
        body_file = None
        if body is not None:
            with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False, encoding="utf-8") as handle:
                json.dump(body, handle)
                body_file = handle.name
            command += ["--input", body_file]
        try:
            result = run_command(command)
        finally:
            if body_file:
                os.unlink(body_file)
        if result.returncode != 0:
            if method == "GET" and "(HTTP 404)" in result.stderr:
                return None
            raise ReleaseError(f"gh api {method} {path} failed: {(result.stderr or result.stdout).strip()[-500:]}")
        if paginate:
            return [json.loads(line) for line in result.stdout.splitlines() if line.strip()]
        return json.loads(result.stdout) if result.stdout.strip() else {}

    def issues_enabled(self):
        """Returns whether the repository has Issues turned on."""
        return bool((self.request("GET", f"repos/{self.repo}") or {}).get("has_issues"))

    def issues(self):
        """Returns [{"number", "body"}] for every open and closed issue with the label."""
        issues = self.request("GET", f"repos/{self.repo}/issues?labels={LABEL}&state=all&per_page=100", paginate=True)
        return [{"number": i["number"], "body": i.get("body") or ""} for i in issues if "pull_request" not in i]

    def ensure_labels(self, names):
        for name in names:
            if self.request("GET", f"repos/{self.repo}/labels/{name.replace('/', '%2F')}") is None:
                color, description = LABEL_INFO[name]
                self.request("POST", f"repos/{self.repo}/labels",
                             {"name": name, "color": color, "description": description})

    def create_issue(self, title, body, labels):
        return self.request("POST", f"repos/{self.repo}/issues", {"title": title, "body": body, "labels": list(labels)})

    def close_duplicate(self, number, original):
        self.request("POST", f"repos/{self.repo}/issues/{number}/comments",
                     {"body": f"Duplicate of #{original}, which another run of this workflow opened first."})
        self.request("PATCH", f"repos/{self.repo}/issues/{number}", {"state": "closed", "state_reason": "duplicate"})


def matching(issues, release):
    # Every body ends with its marker. Upstream text earlier in the body can't fake one.
    return sorted(i["number"] for i in issues if i["body"].rstrip().endswith(marker(release)))


def open_issue(github, release, title, body, labels):
    """Opens the issue unless one exists. Returns (number, url, note).

    The workflow runs one job at a time, but a second run elsewhere, such as a
    local run, can still race. So the script re-reads the issues right before
    it creates one, and after creating, keeps the oldest issue for the release
    and closes its own as a duplicate.
    """
    existing = matching(github.issues(), release)
    if existing:
        return existing[0], None, "opened by another run meanwhile"
    issue = github.create_issue(title, body, labels)
    number = issue.get("number")
    found = matching(github.issues(), release)
    first = min(found + [number])
    if first != number:
        github.close_duplicate(number, first)
        return first, None, f"closed the duplicate #{number} that a concurrent run caused"
    return number, issue.get("html_url"), None


def select_releases(releases, args):
    """Returns the releases to check: one tag, or every minor release after the baseline."""
    if args.tag:
        selected = [r for r in releases if r.tag == args.tag]
        if not selected:
            raise ReleaseError(f"tag {args.tag} isn't in {args.repository}")
        return selected
    baseline = next((r for r in releases if r.tag == args.since), None)
    if not baseline:
        raise ReleaseError(f"baseline tag {args.since} isn't in {args.repository}")
    return [r for r in releases if r.minor > baseline.minor]


def run(args, github, chart, config, git_bin="git"):
    """Checks every minor release after the baseline. Returns (summary lines, failures).

    No state is stored: each run recomputes the result for every release after
    the baseline from the mirror's tags, and the issue markers record which
    releases already have an issue. A failed or skipped run loses nothing.
    """
    # With Issues off, listing issues still works but creating one fails, so a
    # dry run would pass and the real run would fail. Check first, in both.
    if not github.issues_enabled():
        raise ReleaseError(f"Issues are turned off in {github.repo}. Turn them on under Settings > General > "
                           "Features, or run the workflow in another repository.")
    summary, failures = [], []
    with tempfile.TemporaryDirectory(prefix="psa-release-") as tmp:
        repo = Path(tmp) / "upstream"
        releases = fetch_releases(git_bin, args.repository, repo)
        selected = select_releases(releases, args)
        log(f"{len(releases)} releases, {len(selected)} to check")
        issues = github.issues()
        created, labeled = 0, set()
        for release in sorted(selected, key=Release.key):
            line = f"- {code(release.version)}"
            try:
                if matching(issues, release):
                    summary.append(f"{line}: issue already exists (#{matching(issues, release)[0]})")
                    continue
                previous = previous_release(release, releases)
                if not previous:
                    raise ReleaseError("no previous release to compare with; check this release by hand")
                line += f" (since {code(previous.version)})"
                analysis = analyze(git_bin, repo, release, previous, config["checks"])
                if not analysis.relevant:
                    summary.append(f"{line}: no action needed: {no_action_reason(analysis)}")
                    continue
                if created >= MAX_ISSUES_PER_RUN:
                    summary.append(f"{line}: waits for the next run (limit of {MAX_ISSUES_PER_RUN} issues per run)")
                    continue
                mapped, uncertain = map_changes(analysis, config, chart)
                title, body = render_issue(analysis, mapped, uncertain, args.repository, chart.min_kube)
                labels = [LABEL, "area/pss"] + ([MANUAL_LABEL] if uncertain else [])
                review = {"manual": " (manual review)", "confirm": " (confirm only)",
                          "update": ""}[issue_kind(mapped, uncertain)]
                out = Path(args.out) / f"{release.tag}.md"
                out.write_text(f"# {title}\n\nLabels: {', '.join(labels)}\n\n{body}\n", encoding="utf-8")
                if args.dry_run:
                    summary.append(f"{line}: would open an issue{review} (dry run, see {out.name})")
                else:
                    github.ensure_labels([name for name in labels if name not in labeled])
                    labeled.update(labels)
                    number, url, note = open_issue(github, release, title, body, labels)
                    link = f"[#{number}]({url})" if url else f"#{number}"
                    summary.append(f"{line}: {'opened' if not note else 'issue'} {link}{review}"
                                   + (f" ({note})" if note else ""))
                    issues.append({"number": number, "body": marker(release)})
                created += 1
            except (ReleaseError, OSError) as err:
                log(f"{release.tag}: {err}")
                summary.append(f"{line}: **error**: {err}")
                failures.append(release.tag)
    return summary, failures


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--repo", required=True, help="the repository to open issues in, as OWNER/NAME")
    parser.add_argument("--out", required=True, help="directory for summary.md and the issue bodies")
    parser.add_argument("--dry-run", action="store_true", help="write the issues to --out instead of opening them")
    parser.add_argument("--since", help="baseline tag: check the minor releases after it (default: since in --config)")
    parser.add_argument("--tag", help="check only this kubernetes-1.<minor>.0 tag")
    parser.add_argument("--repository", default=DEFAULT_REPOSITORY,
                        help="the PSA mirror to read, https://github.com/OWNER/REPO (a fork for testing)")
    parser.add_argument("--config", default=str(CONFIG_FILE), help="baseline and check-to-policy map")
    parser.add_argument("--git", default="git", help="path to git")
    args = parser.parse_args(argv)
    for name in ("tag", "since"):
        if getattr(args, name) and not TAG_RE.match(getattr(args, name)):
            parser.error(f"--{name} must look like kubernetes-1.<minor>.0")
    try:
        # The URL goes to git remote add, and the issue links assume GitHub, so
        # allow only a GitHub repository URL. A fork works for testing.
        if not re.fullmatch(r"https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repository):
            raise ReleaseError("--repository must look like https://github.com/OWNER/REPO")
        config = load_config(args.config)
        args.since = args.since or config["since"]
        out = Path(args.out)
        out.mkdir(parents=True, exist_ok=True)
        summary, failures = run(args, GitHub(args.repo), Chart(ROOT), config, args.git)
    except (ReleaseError, OSError, yaml.YAMLError) as err:
        print(f"ERROR: {err}")
        return 1
    mode = " (dry run)" if args.dry_run else ""
    scope = f"release {args.tag}" if args.tag else f"minor releases after {args.since}"
    report = "\n".join([f"# Pod Security Admission release check{mode}", "", f"Checked {scope}.", "",
                        *(summary or ["- No new releases."]), ""])
    (out / "summary.md").write_text(report, encoding="utf-8")
    print(report)
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
