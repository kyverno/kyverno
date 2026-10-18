#!/usr/bin/env python3
"""Verifies the PSS ValidatingPolicies in the kyverno-policies chart against
upstream Pod Security Admission (PSA) fixtures.

Upstream generates pass and fail pod fixtures for every level and policy
version (test/testdata/<level>/v1.<N>/{pass,fail}/<checkid><n>.yaml). This
script renders the chart for each level, Kubernetes version, and
podSecurityUserNamespaces value in scripts/pss-upstream/mapping.yaml, applies
the rendered policies to the fixtures at the commit pinned in
scripts/pss-upstream/upstream.yaml with kubectl-kyverno, and compares the
results:

- A pass fixture must not fail on any policy.
- A fail fixture must fail the policy mapped to its check ID. When that policy
  isn't rendered, the fixture must fail at least one policy.
- A policy that errors on any fixture is reported, and an error never counts
  as a failure.
- A fail fixture with an unmapped check ID is an error, which is how a new
  upstream check shows up.
- Differences listed in knownDifferences are reported, not failed. An entry
  that matches nothing is stale and fails the check.

The API server defaults a volume with no source to emptyDir before admission,
but the CLI sees fixtures as written, so the script applies that default first.

This needs no cluster. It needs Python with PyYAML, helm, kubectl-kyverno, and
git (unless --fixtures points at a local test/testdata directory).

Usage: scripts/verify-policies-pss-fixtures.py --cli PATH [--helm PATH]
                                               [--fixtures DIR] [--jobs N]

Exit codes: 0 when everything matches, 1 on a mismatch or an error.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

import yaml


ROOT = Path(__file__).resolve().parents[1]
DATA_DIR = ROOT / "scripts/pss-upstream"
LEVELS = ("baseline", "restricted")
KINDS = ("pass", "fail")
USERNS_VALUES = (False, True)


class VerifyError(Exception):
    """An error that stops the check, as opposed to a mismatch."""


@dataclass(frozen=True)
class Run:
    """One chart render: a level, a Kubernetes minor version, and a flag value."""

    level: str
    minor: int
    userns: bool

    def __str__(self):
        return f"{self.level} v1.{self.minor} userns={str(self.userns).lower()}"


@dataclass(frozen=True)
class Finding:
    """A fixture whose result differs from upstream.

    error is True when no known difference can excuse it, for example an
    unmapped check ID or a fixture with no result.
    """

    run: Run
    kind: str
    fixture: str
    message: str
    error: bool = False

    def __str__(self):
        return f"{self.run}: {self.kind}/{self.fixture} {self.message}"


def log(message):
    print(f"[verify-policies-pss-fixtures] {message}", file=sys.stderr)


def parse_minor(version):
    """Returns the minor number of a "1.<minor>" version string."""
    match = re.fullmatch(r"1\.(\d+)", str(version))
    if not match:
        raise VerifyError(f'invalid version "{version}", want "1.<minor>"')
    return int(match.group(1))


def load_config(mapping):
    """Validates the mapping file and returns it with versions normalized."""
    checks = mapping.get("checks")
    if not isinstance(checks, dict) or not checks:
        raise VerifyError("mapping: checks must be a non-empty map")
    for check_id, policy in checks.items():
        if not isinstance(check_id, str) or not isinstance(policy, str) or not policy:
            raise VerifyError(f"mapping: checks.{check_id} must map to a policy name")
    lowered = [check_id.lower() for check_id in checks]
    if len(set(lowered)) != len(lowered):
        raise VerifyError("mapping: check IDs must be unique ignoring case")

    versions = mapping.get("versions")
    if not isinstance(versions, list) or not versions:
        raise VerifyError("mapping: versions must be a non-empty list")
    for version in versions:
        if version != "latest":
            parse_minor(version)

    known = mapping.get("knownDifferences") or []
    if not isinstance(known, list):
        raise VerifyError("mapping: knownDifferences must be a list")
    for i, entry in enumerate(known):
        where = f"mapping: knownDifferences[{i}]"
        if not isinstance(entry, dict):
            raise VerifyError(f"{where} must be a map")
        if entry.get("level") not in LEVELS:
            raise VerifyError(f"{where}.level must be one of {', '.join(LEVELS)}")
        if entry.get("kind") not in KINDS:
            raise VerifyError(f"{where}.kind must be one of {', '.join(KINDS)}")
        fixtures = entry.get("fixtures")
        if not isinstance(fixtures, list) or not fixtures or not all(isinstance(f, str) for f in fixtures):
            raise VerifyError(f"{where}.fixtures must be a non-empty list of fixture names")
        if not str(entry.get("reason") or "").strip():
            raise VerifyError(f"{where}.reason is required")
        for bound in ("minVersion", "maxVersion"):
            if bound in entry:
                parse_minor(entry[bound])
        values = entry.get("values") or {}
        if not isinstance(values, dict) or set(values) - {"podSecurityUserNamespaces"}:
            raise VerifyError(f"{where}.values may only set podSecurityUserNamespaces")
    return {"checks": checks, "versions": versions, "knownDifferences": known}


def load_pin(pin):
    for key in ("repository", "commit"):
        if not isinstance(pin.get(key), str) or not pin[key]:
            raise VerifyError(f"pin: {key} is required")
    # The URL goes to git remote add, so allow only HTTPS, not ext:: or file://.
    if not pin["repository"].startswith("https://"):
        raise VerifyError("pin: repository must be an https:// URL")
    if not re.fullmatch(r"[0-9a-f]{40}", pin["commit"]):
        raise VerifyError("pin: commit must be a full 40-character SHA")
    return pin


def resolve_versions(versions, available):
    """Turns the configured versions into minor numbers that have fixtures.

    Returns the minors, deduplicated in order, and a list of error messages for
    versions without a fixture directory.
    """
    minors, errors = [], []
    for version in versions:
        if version == "latest":
            if not available:
                errors.append("no fixture versions found")
                continue
            minor = max(available)
        else:
            minor = parse_minor(version)
        if minor not in available:
            errors.append(f"no fixture directory for v1.{minor}")
        elif minor not in minors:
            minors.append(minor)
    return minors, errors


def available_minors(fixtures_dir):
    """Returns the minor versions that have fixtures for every level."""
    per_level = []
    for level in LEVELS:
        level_dir = fixtures_dir / level
        found = set()
        if level_dir.is_dir():
            for child in level_dir.iterdir():
                match = re.fullmatch(r"v1\.(\d+)", child.name)
                if match and child.is_dir():
                    found.add(int(match.group(1)))
        per_level.append(found)
    return set.intersection(*per_level)


def check_id(fixture):
    """Returns the lowercased check ID of a fixture name, for example
    "procmount_restricted" for "procmount_restricted1"."""
    return re.sub(r"\d+$", "", fixture).lower()


def normalize_pod(pod):
    """Applies the API server default for volumes with no source.

    SetDefaults_Volume in kubernetes/kubernetes sets emptyDir when every
    volume source is nil. A source set to null is nil too. Returns the number
    of volumes changed.
    """
    changed = 0
    for volume in ((pod or {}).get("spec") or {}).get("volumes") or []:
        if not isinstance(volume, dict) or "name" not in volume:
            continue
        sources = [key for key in volume if key != "name"]
        if all(volume[key] is None for key in sources):
            for key in sources:
                del volume[key]
            volume["emptyDir"] = {}
            changed += 1
    return changed


def prepare_fixtures(src, dst):
    """Copies normalized fixtures from src to dst.

    Returns the fixture names (metadata.name), which the policy report uses.
    """
    dst.mkdir(parents=True)
    names = []
    for path in sorted(src.glob("*.yaml")):
        pod = yaml.safe_load(path.read_text(encoding="utf-8"))
        name = ((pod or {}).get("metadata") or {}).get("name")
        if not name:
            raise VerifyError(f"{path}: fixture has no metadata.name")
        normalize_pod(pod)
        (dst / path.name).write_text(yaml.safe_dump(pod, sort_keys=False), encoding="utf-8")
        names.append(name)
    if not names:
        raise VerifyError(f"{src}: no fixtures")
    return names


def run_command(args):
    try:
        return subprocess.run(args, capture_output=True, text=True)
    except FileNotFoundError as err:
        raise VerifyError(f"command not found: {args[0]}") from err
    except OSError as err:
        # For example, a binary built for another OS or a file without
        # execute permission.
        raise VerifyError(f"can't run {args[0]}: {err}") from err


def render(helm, chart, run, path):
    """Renders the chart for one run and returns the ValidatingPolicy names."""
    result = run_command([
        helm, "template", "pss", str(chart),
        "--kube-version", f"v1.{run.minor}.0",
        "--set", "policyType=ValidatingPolicy",
        "--set", f"podSecurityStandard={run.level}",
        "--set", f"podSecurityUserNamespaces={str(run.userns).lower()}",
    ])
    if result.returncode != 0:
        raise VerifyError(f"{run}: helm template failed: {result.stderr.strip()[-500:]}")
    path.write_text(result.stdout, encoding="utf-8")
    names = {doc["metadata"]["name"] for doc in yaml.safe_load_all(result.stdout)
             if isinstance(doc, dict) and doc.get("kind") == "ValidatingPolicy"}
    if not names:
        raise VerifyError(f"{run}: the chart rendered no ValidatingPolicy")
    return names


def parse_report(text):
    """Parses a kubectl-kyverno JSON policy report.

    Returns {fixture name: {policy name: result}}.
    """
    try:
        report = json.loads(text)
    except ValueError as err:
        raise VerifyError(f"unparsable policy report: {err}") from err
    if not isinstance(report, dict) or not isinstance(report.get("results", []), list):
        raise VerifyError("unexpected policy report shape")
    results = {}
    for entry in report.get("results") or []:
        for resource in entry.get("resources") or []:
            results.setdefault(resource.get("name"), {})[entry.get("policy")] = entry.get("result")
    return results


def apply_policies(cli, policies, resources):
    """Runs kubectl-kyverno apply and returns the parsed report.

    The CLI exits with 1 whenever a policy fails, so the exit code is ignored
    and the report decides.
    """
    result = run_command([cli, "apply", str(policies), "--resource", str(resources),
                          "--policy-report", "--output-format", "json"])
    if not result.stdout.strip():
        raise VerifyError(f"kubectl-kyverno apply on {resources} printed no report "
                          f"(exit code {result.returncode}): {result.stderr.strip()[-500:]}")
    return parse_report(result.stdout)


def compare(run, kind, fixtures, results, rendered, checks):
    """Compares the results for one run and kind with upstream expectations."""
    policy_for = {check.lower(): policy for check, policy in checks.items()}
    findings = []
    for fixture in fixtures:
        if fixture not in results:
            findings.append(Finding(run, kind, fixture, "has no result in the policy report", error=True))
            continue
        outcome = results[fixture]
        # Only "fail" is a rejection. A policy that errors didn't evaluate the
        # fixture, so it's reported on its own and never counts as a rejection.
        failed = sorted(p for p, r in outcome.items() if r == "fail")
        errored = sorted(p for p, r in outcome.items() if r == "error")
        if errored:
            findings.append(Finding(run, kind, fixture, f"errored on {', '.join(errored)}"))
        if kind == "pass":
            if failed:
                findings.append(Finding(run, kind, fixture, f"rejected by {', '.join(failed)}"))
            continue
        cid = check_id(fixture)
        want = policy_for.get(cid)
        if want is None:
            findings.append(Finding(run, kind, fixture, f'has unmapped check ID "{cid}"', error=True))
        elif want in rendered:
            if outcome.get(want) != "fail":
                got = ", ".join(failed) or "none"
                findings.append(Finding(run, kind, fixture, f"expected {want} to fail, got {got}"))
        elif not failed:
            findings.append(Finding(run, kind, fixture,
                                    f"expected a failure ({want} isn't rendered), got none"))
    return findings


def entry_matches(entry, finding):
    run = finding.run
    if finding.error or entry["level"] != run.level or entry["kind"] != finding.kind:
        return False
    if finding.fixture not in entry["fixtures"]:
        return False
    if "minVersion" in entry and run.minor < parse_minor(entry["minVersion"]):
        return False
    if "maxVersion" in entry and run.minor > parse_minor(entry["maxVersion"]):
        return False
    values = entry.get("values") or {}
    if "podSecurityUserNamespaces" in values and values["podSecurityUserNamespaces"] != run.userns:
        return False
    return True


def apply_known_differences(findings, known):
    """Splits findings into unexpected and known ones, and finds stale entries.

    Returns (unexpected, known findings as (finding, entry) pairs, stale
    messages). An entry is stale for each listed fixture it never matched.
    """
    unexpected, matched = [], []
    used = set()
    for finding in findings:
        entry_index = next((i for i, entry in enumerate(known) if entry_matches(entry, finding)), None)
        if entry_index is None:
            unexpected.append(finding)
        else:
            matched.append((finding, known[entry_index]))
            used.add((entry_index, finding.fixture))
    stale = [f"knownDifferences[{i}]: {entry['level']}/{entry['kind']}/{fixture} matched nothing"
             for i, entry in enumerate(known) for fixture in entry["fixtures"]
             if (i, fixture) not in used]
    return unexpected, matched, stale


def fetch_fixtures(git, pin, dest):
    """Fetches test/testdata at the pinned commit with a sparse, shallow clone."""
    steps = [
        [git, "init", "-q", str(dest)],
        [git, "-C", str(dest), "remote", "add", "origin", pin["repository"]],
        [git, "-C", str(dest), "sparse-checkout", "set", "test/testdata"],
        [git, "-C", str(dest), "fetch", "-q", "--depth", "1", "--filter=blob:none", "origin", pin["commit"]],
        [git, "-C", str(dest), "checkout", "-q", "FETCH_HEAD"],
    ]
    for args in steps:
        result = run_command(args)
        if result.returncode != 0:
            raise VerifyError(f"{' '.join(args[1:4])} failed: {result.stderr.strip()[-500:]}")
    return dest / "test/testdata"


def verify_run(args, run, work, prepared):
    """Renders the chart for one run and checks both fixture kinds."""
    policies = work / f"policies-{run.level}-{run.minor}-{str(run.userns).lower()}.yaml"
    rendered = render(args.helm, args.chart, run, policies)
    findings = []
    for kind in KINDS:
        resources, fixtures = prepared[(run.level, run.minor, kind)]
        results = apply_policies(args.cli, policies, resources)
        findings.extend(compare(run, kind, fixtures, results, rendered, args.config["checks"]))
    return findings


def verify(args):
    with tempfile.TemporaryDirectory(prefix="pss-fixtures-") as tmp:
        work = Path(tmp)
        if args.fixtures:
            fixtures_dir = Path(args.fixtures)
        else:
            log(f"fetching fixtures at {args.pin['commit']}")
            fixtures_dir = fetch_fixtures(args.git, args.pin, work / "upstream")

        minors, errors = resolve_versions(args.config["versions"], available_minors(fixtures_dir))
        if errors:
            for error in errors:
                print(f"ERROR: {error}")
            return 1

        prepared = {}
        for level in LEVELS:
            for minor in minors:
                for kind in KINDS:
                    dst = work / "fixtures" / level / f"v1.{minor}" / kind
                    src = fixtures_dir / level / f"v1.{minor}" / kind
                    prepared[(level, minor, kind)] = (dst, prepare_fixtures(src, dst))

        runs = [Run(level, minor, userns) for level in LEVELS for minor in minors for userns in USERNS_VALUES]
        log(f"checking {len(runs)} renders ({', '.join(f'v1.{m}' for m in minors)}) with {args.jobs} jobs")
        with ThreadPoolExecutor(max_workers=args.jobs) as pool:
            findings = [f for result in pool.map(lambda r: verify_run(args, r, work, prepared), runs)
                        for f in result]

    unexpected, known, stale = apply_known_differences(findings, args.config["knownDifferences"])
    for finding, entry in known:
        print(f"KNOWN: {finding} ({' '.join(entry['reason'].split())})")
    for finding in unexpected:
        print(f"{'ERROR' if finding.error else 'FAIL'}: {finding}")
    for message in stale:
        print(f"STALE: {message}")
    checked = sum(len(prepared[(r.level, r.minor, k)][1]) for r in runs for k in KINDS)
    print(f"checked {checked} fixture results across {len(runs)} renders: "
          f"{len(unexpected)} unexpected, {len(known)} known, {len(stale)} stale")
    return 1 if unexpected or stale else 0


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    parser.add_argument("--cli", required=True, help="path to kubectl-kyverno")
    parser.add_argument("--helm", default=os.environ.get("HELM", "helm"), help="path to helm")
    parser.add_argument("--git", default="git", help="path to git")
    parser.add_argument("--chart", default=str(ROOT / "charts/kyverno-policies"), help="chart directory")
    parser.add_argument("--pin", default=str(DATA_DIR / "upstream.yaml"), help="pin file")
    parser.add_argument("--mapping", default=str(DATA_DIR / "mapping.yaml"), help="mapping file")
    parser.add_argument("--fixtures", help="local upstream test/testdata directory; skips the fetch")
    parser.add_argument("--jobs", type=int, default=min(4, os.cpu_count() or 1),
                        help="renders to check in parallel (default: CPUs, at most 4)")
    args = parser.parse_args(argv)
    if args.jobs < 1:
        parser.error("--jobs must be at least 1")
    # Windows doesn't run a relative path such as $(CLI_BIN), so make paths
    # absolute. Bare names like "helm" are left to the PATH lookup.
    for name in ("cli", "helm", "git"):
        value = getattr(args, name)
        if "/" in value or os.sep in value:
            setattr(args, name, str(Path(value).resolve()))
    try:
        args.config = load_config(yaml.safe_load(Path(args.mapping).read_text(encoding="utf-8")) or {})
        args.pin = load_pin(yaml.safe_load(Path(args.pin).read_text(encoding="utf-8")) or {})
        return verify(args)
    except (VerifyError, OSError, yaml.YAMLError) as err:
        print(f"ERROR: {err}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
