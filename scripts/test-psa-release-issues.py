#!/usr/bin/env python3
"""Offline tests for scripts/psa-release-issues.py (requires PyYAML and git).

Each test builds a small local git repository shaped like the
kubernetes/pod-security-admission mirror, with dated kubernetes-1.* tags as
the mocked releases, and a small chart. GitHub is faked by replacing
GitHub.request or the gh process, so no network is used. WorkflowTest checks
the workflow file's permissions, pins, and defaults, but doesn't run it: only
a real run on GitHub Actions verifies the workflow itself.
"""

import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from contextlib import redirect_stderr, redirect_stdout
from unittest import mock

import yaml


ROOT = Path(__file__).resolve().parents[1]
# Keep the import from writing scripts/__pycache__.
sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location("psa_release_issues", ROOT / "scripts/psa-release-issues.py")
psa = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(psa)

LICENSE = '/*\nCopyright The Kubernetes Authors.\n\nLicensed under the Apache License (the "License");\n*/\n\n'
CHECK_SYSCTLS = LICENSE + (
    'package policy\n\n'
    'import (\n\t"strings"\n)\n\n'
    'func CheckSysctls() Check {\n\treturn Check{\n\t\tID: "sysctls",\n'
    '\t\tVersions: []VersionedCheck{\n'
    '\t\t\t{MinimumVersion: api.MajorMinorVersion(1, 0), CheckPod: sysctlsV1Dot0},\n'
    '@GATES@\t\t},\n\t}\n}\n\n'
    'var allowed = sets.NewString(\n\t"kernel.shm_rmid_forced",\n@VALUES@)\n'
)


def check_sysctls(gate=None, values=()):
    """Returns check_sysctls.go with an optional 1.<gate> version gate and extra allowed values."""
    gates = f"\t\t\t{{MinimumVersion: api.MajorMinorVersion(1, {gate}), CheckPod: sysctlsV1Dot{gate}}},\n"
    gates = gates if gate else ""
    # Go braces rule out str.format on the template.
    return CHECK_SYSCTLS.replace("@GATES@", gates).replace("@VALUES@", "".join(f'\t"{v}",\n' for v in values))


CHECK_CAPS = 'package policy\n\nfunc CheckCapabilitiesBaseline() Check { return Check{ID: "capabilities_baseline"} }\n'
FIXTURE = "apiVersion: v1\nkind: Pod\nmetadata:\n  name: {name}\nspec: {{}}\n"
K8S_SHA = "b" * 40
# The synthetic mirror's URL. It isn't the upstream mirror, so the issues say they may describe a test release.
TEST_MIRROR = "https://example.com/psa"
SECTIONS = ["Summary", "Release details", "What changed", "Kyverno impact", "Required actions", "Validation",
            "Upstream evidence"]

HELPERS = '''{{- define "kyverno-policies.labels" -}}
app: kyverno
{{- end -}}

{{- define "kyverno-policies.podSecurityBaseline" -}}
true
{{- end -}}

{{/* Safe sysctls. */}}
{{- define "kyverno-policies.allowedSysctls" -}}
{{- $s := list "kernel.shm_rmid_forced" -}}
{{- if semverCompare ">=1.32.0-0" .Capabilities.KubeVersion.Version -}}
{{- $s = concat $s (list "net.ipv4.tcp_rmem") -}}
{{- end -}}
{{- if semverCompare ">=1.37.0-0" .Capabilities.KubeVersion.Version -}}
{{- $s = concat $s (list "net.ipv4.tcp_notsent_lowat") -}}
{{- end -}}
{{- end -}}
'''
TEMPLATE = '''{{{{- $name := "{name}" }}}}
{{{{- if eq (include "kyverno-policies.podSecurityBaseline" .) "true" }}}}
metadata:
  labels: {{{{ include "kyverno-policies.labels" . }}}}
{extra}
{{{{- end }}}}
'''
CONFIG = {
    "checks": {"sysctls": "restrict-sysctls", "capabilities_baseline": "disallow-capabilities"},
}


class Upstream:
    """A local repository shaped like the PSA mirror, with dated, annotated release tags."""

    def __init__(self, path):
        self.path = path
        self.when = int((datetime.now(timezone.utc) - timedelta(days=60)).timestamp())
        path.mkdir(parents=True)
        self.run("init", "-q", "-b", "master")
        self.write({
            "policy/check_sysctls.go": check_sysctls(),
            "policy/check_capabilities_baseline.go": CHECK_CAPS,
            "policy/helpers.go": "package policy\n",
            "policy/check_sysctls_test.go": "package policy\n",
            "api/helpers.go": "package api\n",
            "test/run.go": "package test\n",
            "test/fixtures_sysctls.go": "package test\n",
            "test/testdata/baseline/v1.36/pass/sysctls0.yaml": FIXTURE.format(name="sysctls0"),
            "test/testdata/baseline/v1.36/fail/capabilities_baseline0.yaml": FIXTURE.format(name="caps0"),
            "go.mod": "module k8s.io/pod-security-admission\n",
        })
        self.commit("Initial")

    def env(self):
        stamp = f"{self.when} +0000"
        return dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@example.com", GIT_COMMITTER_NAME="t",
                    GIT_COMMITTER_EMAIL="t@example.com", GIT_AUTHOR_DATE=stamp, GIT_COMMITTER_DATE=stamp)

    def run(self, *args):
        result = subprocess.run(["git", "-C", str(self.path), *args], capture_output=True, text=True, env=self.env())
        if result.returncode != 0:
            raise AssertionError(f"git {args}: {result.stderr}")
        return result.stdout.strip()

    def write(self, files):
        for name, content in files.items():
            path = self.path / name
            path.parent.mkdir(parents=True, exist_ok=True)
            # Bytes keep "\n" on Windows; write_text has no newline argument before Python 3.10.
            path.write_bytes(content.encode("utf-8"))

    def commit(self, message, files=None, body=f"Kubernetes-commit: {K8S_SHA}"):
        if files:
            self.write(files)
        self.when += 60
        self.run("add", "-A")
        self.run("commit", "-q", "--allow-empty", "-m", f"{message}\n\n{body}")
        return self.run("rev-parse", "HEAD")

    def release(self, version, days_ago=1, branch_from=None, files=None, message="Change"):
        """Tags a release the way the publishing bot does: on a side commit that bumps go.mod.

        files are committed to master first. With branch_from, the release
        goes on a release branch cut from that tag, like a patch release.
        """
        self.when = int((datetime.now(timezone.utc) - timedelta(days=days_ago)).timestamp())
        if files:
            self.commit(message, files)
        head = self.run("rev-parse", "HEAD")
        self.run("checkout", "-q", "--detach", branch_from or head)
        sha = self.commit(f"Update dependencies to v0.{version} tag", {"go.mod": f"module psa // {version}\n"}, body="")
        self.run("tag", "-a", "-m", version, f"kubernetes-1.{version}", sha)
        # master continues from where it was, without the go.mod bump.
        self.run("checkout", "-q", "master")
        return sha


class Base(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self.tmp.name)
        self.upstream = Upstream(self.dir / "upstream")
        self.chart = self.make_chart(self.dir / "kyverno")

    def tearDown(self):
        self.tmp.cleanup()

    @staticmethod
    def make_chart(root):
        templates = root / psa.TEMPLATES_DIR
        (templates / "baseline").mkdir(parents=True)
        (templates / "_helpers.tpl").write_text(HELPERS, encoding="utf-8")
        (templates / "baseline/restrict-sysctls.cel.yaml").write_text(TEMPLATE.format(
            name="restrict-sysctls", extra='  expr: {{ include "kyverno-policies.allowedSysctls" . }}'), encoding="utf-8")
        for name in ("disallow-capabilities", "disallow-host-path", "disallow-proc-mount"):
            (templates / f"baseline/{name}.cel.yaml").write_text(TEMPLATE.format(name=name, extra=""), encoding="utf-8")
        tests = root / psa.CLI_TESTS_DIR
        for name in ("restrict-sysctls", "disallow-proc-mount-strict"):
            (tests / f"pss-{name}").mkdir(parents=True)
            (tests / f"pss-{name}/kyverno-test.yaml").write_text(f"policies: [{name}]\n", encoding="utf-8")
        return psa.Chart(root)

    def releases(self):
        work = self.dir / f"clone{len(list(self.dir.iterdir()))}"
        return work, psa.fetch_releases("git", str(self.upstream.path), work)

    def analyze(self, tag, previous, checks=None):
        repo, releases = self.releases()
        by_tag = {r.tag: r for r in releases}
        return psa.analyze("git", repo, by_tag[tag], by_tag[previous], checks or CONFIG["checks"])

    def issue(self, tag="kubernetes-1.38.0", previous="kubernetes-1.37.0", config=CONFIG, repository=TEST_MIRROR,
              min_kube=None):
        """Analyzes, maps, and renders one release. Returns all the results, and the body split into sections."""
        analysis = self.analyze(tag, previous, config["checks"])
        mapped, uncertain = psa.map_changes(analysis, config, self.chart)
        title, body = psa.render_issue(analysis, mapped, uncertain, repository, min_kube)
        parts = re.split(r"^## (.+)$", body, flags=re.MULTILINE)
        issue = SimpleNamespace(analysis=analysis, mapped=mapped, uncertain=uncertain, title=title,
                                kind=psa.issue_kind(mapped, uncertain), body=body,
                                sections=dict(zip(parts[1::2], parts[2::2])))
        # Every issue kind shares the same structure, and never contradicts its own mapping.
        self.assertEqual(list(issue.sections), SECTIONS)
        self.assertIn(f"**Issue type:** {psa.KIND_NAMES[issue.kind]}", issue.sections["Summary"])
        self.assertNotIn("Every change maps", body)
        # Counts read as "1 fixture" or "3 files", never "file(s)".
        self.assertNotIn("(s)", body)
        self.assertEqual("**Not mapped.**" in issue.sections["Kyverno impact"], bool(uncertain))
        self.assertTrue(body.endswith(psa.marker(analysis.release)))
        return issue

    def change(self, files=None, message="Change", **options):
        """Releases v1.37.0, commits files, releases v1.38.0, and returns the issue for v1.38.0."""
        self.upstream.release("37.0", days_ago=3)
        self.upstream.release("38.0", days_ago=2, files=files, message=message)
        return self.issue(**options)

    def assertAllIn(self, texts, body):
        for text in texts:
            self.assertIn(text, body)


class ReleaseTest(unittest.TestCase):
    def test_parses_only_minor_releases_in_order(self):
        text = "\n".join(f"kubernetes-1.{v}\x1f2026-01-01T00:00:00+00:00" for v in
                         ["38.0", "38.0-rc.0", "37.0-alpha.1", "36.9", "37.0", "36.0", "37.1"]) + "\nv0.37.0\x1f\n"
        # Patches, pre-releases, and v0.* tags are skipped.
        self.assertEqual([r.version for r in psa.parse_releases(text)], ["v1.36.0", "v1.37.0", "v1.38.0"])
        self.assertEqual(psa.Release("kubernetes-1.37.0").psa_tag, "v0.37.0")
        with self.assertRaises(ValueError):
            psa.Release("kubernetes-1.37.1")

    def test_previous_release_is_the_previous_minor(self):
        releases = [psa.Release(f"kubernetes-1.{m}.0") for m in (36, 37, 39)]
        self.assertEqual(psa.previous_release(releases[1], releases).version, "v1.36.0")
        # A missing minor is never skipped over.
        self.assertIsNone(psa.previous_release(releases[2], releases))


class SignalsTest(unittest.TestCase):
    def test_ignores_comments_imports_and_check_ids(self):
        diff = "\n".join([
            "diff --git a/policy/check_x.go b/policy/check_x.go",
            "+/*",
            '+Licensed under the Apache License (the "License");',
            '+pods with "hostUsers: false" are allowed',
            "+*/",
            '+import "slices"',
            '+\tcorev1 "k8s.io/api/core/v1"',
            '+\tID: "procMount_restricted",',
            '+\t"net.ipv4.tcp_notsent_lowat", // safe',
            '-\t"net.ipv4.old",',
            '+\t{MinimumVersion: api.MajorMinorVersion(1, 37)},',
        ])
        signals = psa.diff_signals(diff, [], {"procMount_restricted"})
        self.assertEqual(signals["added_values"], ["net.ipv4.tcp_notsent_lowat"])
        self.assertEqual(signals["removed_values"], ["net.ipv4.old"])
        self.assertEqual(signals["gates"], ["1.37"])

    def test_removed_and_moved_version_gates(self):
        moved = "\n".join(["-\t{MinimumVersion: api.MajorMinorVersion(1, 37)},",
                           "+\t{MinimumVersion: api.MajorMinorVersion(1, 38)},"])
        signals = psa.diff_signals(moved, [])
        self.assertEqual((signals["gates"], signals["removed_gates"]), (["1.38"], ["1.37"]))
        # A gate on both sides only moved, like a value.
        same = "\n".join(["-\t{MinimumVersion: api.MajorMinorVersion(1, 37), CheckPod: a},",
                          "+\t{MinimumVersion: api.MajorMinorVersion(1, 37), CheckPod: b},"])
        signals = psa.diff_signals(same, [])
        self.assertEqual((signals["gates"], signals["removed_gates"]), ([], []))


class ChartTest(Base):
    def test_lookup(self):
        files = self.chart.lookup("restrict-sysctls")
        self.assertEqual(files["template"], f"{psa.TEMPLATES_DIR}/baseline/restrict-sysctls.cel.yaml")
        self.assertEqual(files["level"], "baseline")
        # labels is plumbing, podSecurity* selects a profile; nested {{ end }} doesn't cut the helper short.
        self.assertEqual(files["helpers"], ["allowedSysctls"])
        self.assertEqual(files["gates"], ["1.32", "1.37"])
        self.assertEqual(files["tests"], [f"{psa.CLI_TESTS_DIR}/pss-restrict-sysctls"])
        self.assertIn("net.ipv4.tcp_notsent_lowat", files["sources"])

    def test_test_lookup_matches_whole_names(self):
        self.assertEqual(self.chart.lookup("disallow-proc-mount")["tests"], [])
        self.assertIsNone(self.chart.lookup("no-such-policy"))

    def test_minimum_kubernetes_version_comes_from_chart_yaml(self):
        # The fake chart has no Chart.yaml, so it has no minimum.
        self.assertIsNone(self.chart.min_kube)
        (self.dir / "kyverno" / psa.CHART_FILE).write_text('name: x\nkubeVersion: ">=1.25.0-0"\n', encoding="utf-8")
        self.assertEqual(psa.Chart(self.dir / "kyverno").min_kube, "1.25")


class AnalyzeTest(Base):
    SYSCTLS = f"{psa.TEMPLATES_DIR}/baseline/restrict-sysctls.cel.yaml"
    SYSCTLS_TEST = f"{psa.CLI_TESTS_DIR}/pss-restrict-sysctls"
    # Text that only an issue asking for policy work may contain.
    IMPLEMENTATION = ["- [ ] Update `", "Handle `", "Add or update a CLI test", "semverCompare", "make test-cli",
                      "helm template", "Update the expected names"]

    def test_dependency_only_release_needs_no_action(self):
        issue = self.change()
        self.assertFalse(issue.analysis.relevant)
        self.assertIn("no change", psa.no_action_reason(issue.analysis))

    def test_test_code_only_is_a_confirm_issue_without_implementation_work(self):
        issue = self.change({"test/run.go": "package test // v2\n", "test/fixtures_test.go": "package test // v2\n",
                             "policy/check_sysctls_test.go": "package policy // v2\n"})
        # Never "no action needed": a person confirms test-only changes.
        self.assertTrue(issue.analysis.relevant)
        # fixtures_test.go tests the generator, so it isn't a check called "test".
        self.assertEqual(sorted(c["path"] for c in issue.analysis.groups["test-framework"]),
                         ["test/fixtures_test.go", "test/run.go"])
        self.assertEqual(sorted(e["group"] for e in issue.mapped), ["sysctls", "test-framework"])
        self.assertTrue(all(e["test_only"] for e in issue.mapped))
        self.assertEqual((issue.uncertain, issue.kind), ([], "confirm"))
        self.assertEqual(issue.title, "Pod Security Admission v1.38.0: confirm the PSS CEL policies need no change")
        self.assertIn("because only test code changed for `sysctls`, `test-framework`,", issue.sections["Summary"])
        self.assertIn("- **Test code:** 3 files for `sysctls`, `test-framework`", issue.sections["What changed"])
        # Test code isn't mapped to a template, and the issue says why without claiming nothing changed.
        impact = issue.sections["Kyverno impact"]
        self.assertIn("**Test code only:** `sysctls`, `test-framework`. The automation found no check code or "
                      "fixture change for them,", impact)
        self.assertIn("doesn't rule out a behavior change", impact)
        self.assertNotIn("Template:", impact)
        self.assertAllIn(["- [ ] Read the upstream test changes for `sysctls`, `test-framework`",
                          "- [ ] Comment with the outcome"], issue.sections["Required actions"])
        self.assertIn("None, unless the review finds a behavior change.", issue.sections["Validation"])
        # The evidence lists the test-only commit that caused the issue.
        self.assertEqual(len(issue.analysis.commits), 1)
        self.assertIn("`Change`", issue.sections["Upstream evidence"])
        for text in self.IMPLEMENTATION + ["verify-policies"]:
            self.assertNotIn(text, issue.body)

    def test_a_non_fixture_file_in_testdata_is_test_code_not_dropped(self):
        issue = self.change({"test/testdata/README.md": "How the fixtures are generated.\n"})
        self.assertTrue(issue.analysis.relevant)
        self.assertEqual([c["path"] for c in issue.analysis.groups["test-framework"]], ["test/testdata/README.md"])
        self.assertEqual(issue.kind, "confirm")

    def test_code_fixtures_and_tests_are_listed_apart_and_criteria_follow_the_changes(self):
        # Like upstream 1.35: every fixture generator changed, but only one check's code did.
        issue = self.change({
            "test/fixtures_sysctls.go": "package test // v2\n",
            "test/fixtures_capabilities_baseline.go": "package test // v2\n",
            "test/fixtures_privileged.go": "package test // v2\n",
            "test/testdata/baseline/v1.36/fail/capabilities_baseline0.yaml": FIXTURE.format(name="caps0-v2"),
            "policy/check_sysctls.go": check_sysctls(gate=37),
        })
        changed = issue.sections["What changed"]
        self.assertLess(changed.index("**Code:**"), changed.index("**Fixtures:**"))
        self.assertLess(changed.index("**Fixtures:**"), changed.index("**Test code:**"))
        self.assertAllIn(["  - `sysctls`: version gate `>=1.37`",
                          "  - `capabilities_baseline`: 1 fixture for `1.36` (baseline)",
                          "- **Test code:** 3 files for `capabilities_baseline`, `privileged`, `sysctls`"], changed)
        # sysctls already has the 1.37 gate; only the fixture change needs work.
        self.assertEqual(issue.kind, "update")
        self.assertIn("- `disallow-capabilities` (baseline), mapped from upstream check `capabilities_baseline`: "
                      "needs an update", issue.sections["Kyverno impact"])
        self.assertIn("**Test code only:** `privileged`. The automation found no check code or fixture change for "
                      "it, so it lists no CEL template for it.", issue.sections["Kyverno impact"])
        actions = issue.sections["Required actions"]
        self.assertIn("- [ ] Compare `disallow-capabilities` with the changed upstream fixtures for `1.36`", actions)
        # sysctls is handled: no sysctls work, only its check, and no gate to add.
        self.assertNotIn(psa.SYSCTLS_SCRIPT, actions)
        validation = issue.sections["Validation"]
        self.assertAllIn([
            "- [ ] The PSS CLI tests pass with `make test-cli-local-vpols`.",
            # No command covers the capabilities fixtures, and the issue says so.
            "- [ ] Each upstream `pass` and `fail` fixture in `test/testdata/*/v1.36/` for `capabilities_baseline` "
            "gets the same result from the updated policy. No command runs them against the PSS policies, so compare "
            "them by hand.",
            "- [ ] `make verify-policies-sysctls` passes, and its output lists `v1.37.0`.",
        ], validation)
        self.assertNotIn("semverCompare", validation)

    def test_copied_fixture_version_needs_no_action(self):
        issue = self.change({
            "test/testdata/baseline/v1.37/pass/sysctls0.yaml": FIXTURE.format(name="sysctls0"),
            "test/testdata/baseline/v1.37/fail/capabilities_baseline0.yaml": FIXTURE.format(name="caps0"),
        })
        self.assertFalse(issue.analysis.relevant)
        self.assertEqual((issue.analysis.copied, issue.analysis.new_versions), (2, ["baseline/v1.37"]))

    def test_a_fixture_dropped_from_a_new_version_is_listed_as_deleted(self):
        issue = self.change({"test/testdata/baseline/v1.37/pass/sysctls0.yaml": FIXTURE.format(name="sysctls0")})
        self.assertTrue(issue.analysis.relevant)
        self.assertEqual([(c["path"], c["status"]) for c in issue.analysis.groups["capabilities_baseline"]],
                         [("test/testdata/baseline/v1.37/fail/capabilities_baseline0.yaml", "D")])
        self.assertIn("`test/testdata/baseline/v1.37/fail/capabilities_baseline0.yaml` (D)",
                      issue.sections["Upstream evidence"])

    def test_a_value_is_matched_only_as_a_whole_literal(self):
        # The chart has "net.ipv4.tcp_rmem", which contains net.ipv4.tcp.
        issue = self.change({"policy/check_sysctls.go": check_sysctls(values=["net.ipv4.tcp"])})
        entry = issue.mapped[0]
        self.assertEqual((entry["missing_values"], entry["present_values"]), (["net.ipv4.tcp"], []))
        self.assertEqual(issue.kind, "update")

    def test_a_truncated_body_keeps_the_marker_last(self):
        self.upstream.release("37.0", days_ago=3)
        self.upstream.release("38.0", days_ago=2, files={"policy/check_sysctls.go": check_sysctls(
            gate=38, values=[f"net.ipv4.v{i}" for i in range(40)])})
        analysis = self.analyze("kubernetes-1.38.0", "kubernetes-1.37.0")
        mapped, uncertain = psa.map_changes(analysis, CONFIG, self.chart)
        with mock.patch.object(psa, "MAX_BODY_CHARS", 1500):
            _, body = psa.render_issue(analysis, mapped, uncertain, TEST_MIRROR, None)
        self.assertLessEqual(len(body), 1500)
        self.assertIn("_Truncated._", body)
        self.assertTrue(body.endswith(psa.marker(analysis.release)))

    def test_update_issue(self):
        issue = self.change({
            "policy/check_sysctls.go": check_sysctls(
                gate=37, values=["net.ipv4.tcp_notsent_lowat", "net.ipv4.tcp_slow_start_after_idle"]),
            "test/testdata/baseline/v1.37/pass/sysctls0.yaml": FIXTURE.format(name="sysctls0"),
            "test/testdata/baseline/v1.37/pass/sysctls1.yaml": FIXTURE.format(name="sysctls1"),
            "test/testdata/baseline/v1.37/fail/capabilities_baseline0.yaml": FIXTURE.format(name="caps0"),
        }, message="Add safe sysctls @someone fixes #12", min_kube="1.25")
        analysis = issue.analysis
        self.assertTrue(analysis.relevant)
        self.assertEqual(sorted(analysis.groups), ["sysctls"])
        self.assertEqual(sorted(c["kind"] for c in analysis.groups["sysctls"]), ["check", "fixture"])
        self.assertEqual(analysis.copied, 2)
        self.assertEqual(analysis.signals["sysctls"]["gates"], ["1.37"])
        self.assertEqual(analysis.signals["sysctls"]["added_values"],
                         ["net.ipv4.tcp_notsent_lowat", "net.ipv4.tcp_slow_start_after_idle"])
        self.assertEqual(analysis.commits[0][2], K8S_SHA)
        self.assertEqual((analysis.unmapped_checks, analysis.stale_checks), ([], []))

        entry = issue.mapped[0]
        self.assertEqual(entry["policy"], "restrict-sysctls")
        # tcp_notsent_lowat and the 1.37 gate are already in the helper.
        self.assertEqual(entry["missing_values"], ["net.ipv4.tcp_slow_start_after_idle"])
        self.assertEqual(entry["missing_gates"], [])
        self.assertFalse(entry["handled"])
        self.assertEqual((issue.uncertain, issue.kind), ([], "update"))

        self.assertEqual(issue.title, "Pod Security Admission v1.38.0: update the PSS CEL policies")
        self.assertIn("**Recommended action:** Update `restrict-sysctls` to match upstream", issue.sections["Summary"])
        self.assertAllIn([
            "| Release | [`v1.38.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.38.0) |",
            "| Previous release | [`v1.37.0`](https://github.com/kubernetes/kubernetes/releases/tag/v1.37.0) |",
            "https://github.com/kubernetes/kubernetes/tree/v1.38.0/staging/src/k8s.io/pod-security-admission",
            f"| Comparison | [`kubernetes-1.37.0...kubernetes-1.38.0`]({TEST_MIRROR}/compare/kubernetes-1.37.0..."
            "kubernetes-1.38.0) |",
        ], issue.sections["Release details"])
        self.assertIn("  - `sysctls`: version gate `>=1.37`; added `net.ipv4.tcp_notsent_lowat`, "
                      "`net.ipv4.tcp_slow_start_after_idle`", issue.sections["What changed"])
        self.assertAllIn([
            "- `restrict-sysctls` (baseline), mapped from upstream check `sysctls`: needs an update",
            f"  - Template: `{self.SYSCTLS}`",
            "  - Check helpers: `allowedSysctls`",
            "  - Version gates in the template: `>=1.32`, `>=1.37`",
            f"  - CLI tests: `{self.SYSCTLS_TEST}`",
            # A name match, so the issue doesn't claim that the version gate matches too.
            "  - Names already in the template or its helpers: `net.ipv4.tcp_notsent_lowat`. Under which version "
            "gate isn't checked.",
            "  - Not in the template or its helpers: `net.ipv4.tcp_slow_start_after_idle`",
        ], issue.sections["Kyverno impact"])
        actions = issue.sections["Required actions"]
        self.assertAllIn([f"- [ ] Update `{self.SYSCTLS}` for the upstream `sysctls` change:",
                          "  - [ ] Handle `net.ipv4.tcp_slow_start_after_idle`",
                          f"  - [ ] Add or update a CLI test under `{self.SYSCTLS_TEST}`.",
                          # Without a line for the version, make verify-policies-sysctls wouldn't check the change.
                          f"  - [ ] Update the expected names in `{psa.SYSCTLS_SCRIPT}`, with a `check v1.37.0` line, "
                          "so `make verify-policies-sysctls` checks this change."], actions)
        # Don't ask for work that the template already has.
        self.assertNotIn("Handle `net.ipv4.tcp_notsent_lowat`", actions)
        self.assertAllIn([
            "- [ ] Each upstream `pass` and `fail` fixture in `test/testdata/*/v1.37/` for `sysctls` gets the same "
            "result from the updated policy. `make verify-policies-sysctls` checks the sysctls allow-list; compare "
            "any other fixture by hand.",
            "- [ ] The PSS CLI tests pass with `make test-cli-local-vpols`.",
            "- [ ] Behavior that starts at Kubernetes `1.37` is gated with `semverCompare`",
            "- [ ] `make verify-policies-sysctls` passes, and its output lists `v1.37.0`.",
            "- [ ] `helm template charts/kyverno-policies` renders with `--kube-version v1.25.0` and with "
            "`--kube-version v1.38.0`.",
        ], issue.sections["Validation"])
        self.assertAllIn([f"https://github.com/kubernetes/kubernetes/commit/{K8S_SHA}",
                          # Upstream text can't ping people or link Kyverno issues.
                          "`Add safe sysctls @someone fixes #12`"], issue.sections["Upstream evidence"])

    def test_handled_change_is_a_confirm_issue(self):
        issue = self.change({"policy/check_sysctls.go": check_sysctls(gate=37, values=["net.ipv4.tcp_notsent_lowat"])})
        self.assertTrue(issue.mapped[0]["handled"])
        self.assertEqual((issue.uncertain, issue.kind), ([], "confirm"))
        self.assertIn("because the templates already seem to handle the upstream changes to `sysctls`,",
                      issue.sections["Summary"])
        # The "already handled" result is an inference, and the issue says so once.
        impact = issue.sections["Kyverno impact"]
        self.assertIn("mapped from upstream check `sysctls`: may already handle the change. This is an inference:",
                      impact)
        self.assertEqual(impact.count("may already handle"), 1)
        self.assertIn(f"- [ ] Confirm that `{self.SYSCTLS}` already matches the upstream `sysctls` change, and that "
                      f"`{self.SYSCTLS_TEST}` covers it.", issue.sections["Required actions"])
        # Checks that the template matches upstream, and no policy work.
        self.assertAllIn(["- [ ] Each upstream `pass` and `fail` fixture in `test/testdata/*/v1.38/` for `sysctls` "
                          "gets the same result from `restrict-sysctls`.",
                          "- [ ] `make verify-policies-sysctls` passes, and its output lists `v1.37.0`."],
                         issue.sections["Validation"])
        for text in self.IMPLEMENTATION:
            self.assertNotIn(text, issue.body)

        # A value that upstream removed but the template still has means it isn't handled.
        self.upstream.release("39.0", files={"policy/check_sysctls.go": check_sysctls(
            gate=37, values=["net.ipv4.tcp_notsent_lowat"]).replace('\t"kernel.shm_rmid_forced",\n', "")})
        issue = self.issue("kubernetes-1.39.0", "kubernetes-1.37.0")
        self.assertEqual(issue.analysis.signals["sysctls"]["removed_values"], ["kernel.shm_rmid_forced"])
        self.assertFalse(issue.mapped[0]["handled"])
        self.assertEqual(issue.kind, "update")
        self.assertAllIn(["  - Names already in the template or its helpers: `net.ipv4.tcp_notsent_lowat`.",
                          "  - Removed upstream, but still in the template: `kernel.shm_rmid_forced`"],
                         issue.sections["Kyverno impact"])
        self.assertIn("  - [ ] Decide whether `kernel.shm_rmid_forced`, which upstream removed, must stay",
                      issue.sections["Required actions"])

    def test_a_removed_value_absent_from_the_template_is_a_confirm_issue(self):
        self.upstream.release("37.0", days_ago=3, files={"policy/check_sysctls.go": check_sysctls(
            values=["net.ipv4.not_in_chart"])})
        self.upstream.release("38.0", days_ago=2, files={"policy/check_sysctls.go": check_sysctls()})
        issue = self.issue()
        self.assertEqual(issue.analysis.signals["sysctls"]["removed_values"], ["net.ipv4.not_in_chart"])
        self.assertEqual(issue.mapped[0]["kept_values"], [])
        self.assertTrue(issue.mapped[0]["handled"])
        self.assertEqual((issue.uncertain, issue.kind), ([], "confirm"))

    def test_a_failed_tree_listing_raises_instead_of_listing_nothing(self):
        self.upstream.release("37.0")
        repo, _ = self.releases()
        self.assertEqual(psa.tree_blobs("git", repo, "kubernetes-1.37.0", "test/testdata/missing"), {})
        with self.assertRaisesRegex(psa.ReleaseError, "git ls-tree -r kubernetes-1.99.0 failed"):
            psa.tree_blobs("git", repo, "kubernetes-1.99.0", "test/testdata")

    def test_a_confirm_issue_gives_every_reason(self):
        # Like upstream 1.37: a handled check change and test-only changes in one release.
        issue = self.change({"policy/check_sysctls.go": check_sysctls(gate=37, values=["net.ipv4.tcp_notsent_lowat"]),
                             "test/run.go": "package test // v2\n"})
        self.assertEqual(issue.kind, "confirm")
        self.assertIn("because the templates already seem to handle the upstream changes to `sysctls` and only test "
                      "code changed for `test-framework`,", issue.sections["Summary"])
        self.assertAllIn(["- [ ] Confirm that `", "- [ ] Read the upstream test changes for `test-framework`"],
                         issue.sections["Required actions"])

    def test_check_change_without_signals_needs_manual_review_not_an_update(self):
        issue = self.change({"policy/check_sysctls.go": check_sysctls() + "\nfunc helper() {}\n"})
        self.assertTrue(issue.mapped[0]["no_signals"])
        # A code diff alone isn't a confirmed behavior change.
        self.assertEqual(issue.kind, "manual")
        self.assertIn("(manual review)", issue.title)
        self.assertIn("  - `sysctls`: no version gate or value change found", issue.sections["What changed"])
        impact = issue.sections["Kyverno impact"]
        self.assertIn("mapped from upstream check `sysctls`: impact unknown, see **Not mapped**", impact)
        self.assertIn("- `sysctls`: The check code changed, but the diff shows no version gate or value change, so "
                      "the automation can't tell whether behavior changed.", impact)
        self.assertIn("- [ ] Read the `sysctls` code changes in the comparison, and decide whether `restrict-sysctls` "
                      "needs an update.", issue.sections["Required actions"])
        self.assertNotIn("- [ ] Update `", issue.body)

    def test_a_moved_version_gate_needs_manual_review_even_if_the_new_gate_exists(self):
        # Upstream moves the gate from 1.32 to 1.37; the chart has both, so the new gate alone looks handled.
        self.upstream.release("37.0", days_ago=3, files={"policy/check_sysctls.go": check_sysctls(gate=32)})
        self.upstream.release("38.0", days_ago=2, files={"policy/check_sysctls.go": check_sysctls(gate=37)})
        issue = self.issue()
        self.assertEqual(issue.analysis.signals["sysctls"]["removed_gates"], ["1.32"])
        self.assertFalse(issue.mapped[0]["handled"])
        self.assertEqual(issue.kind, "manual")
        self.assertIn("removed version gate `>=1.32`", issue.sections["What changed"])
        self.assertIn("- `sysctls`: Upstream removed the version gate `>=1.32`", issue.sections["Kyverno impact"])

    def test_new_gate_asks_for_a_semver_gate(self):
        issue = self.change({"policy/check_sysctls.go": check_sysctls(gate=39)})
        self.assertEqual(issue.mapped[0]["missing_gates"], ["1.39"])
        self.assertIn('  - [ ] Gate the change with `semverCompare ">=1.39.0-0"`, because upstream changes behavior '
                      'from 1.39.', issue.sections["Required actions"])
        self.assertIn("Behavior that starts at Kubernetes `1.39` is gated", issue.sections["Validation"])
        # The sysctls script must check the version where upstream gates the change.
        self.assertIn(f"  - [ ] Update the expected names in `{psa.SYSCTLS_SCRIPT}`, with a `check v1.39.0` line",
                      issue.sections["Required actions"])
        self.assertIn("- [ ] `make verify-policies-sysctls` passes, and its output lists `v1.39.0`.",
                      issue.sections["Validation"])
        # Without the chart's minimum version, the issue doesn't make one up.
        self.assertIn("renders with the oldest Kubernetes version the chart supports", issue.sections["Validation"])

    def test_manual_review_issue(self):
        issue = self.change({
            "policy/helpers.go": "package policy // changed\n",
            "api/helpers.go": "package api // changed\n",
            "policy/check_newThing.go": 'package policy\n\nfunc CheckNewThing() Check { return Check{ID: "newThing"} }\n',
        })
        self.assertEqual(sorted(issue.analysis.groups), ["api", "newThing", "shared"])
        self.assertEqual(issue.analysis.unmapped_checks, ["newThing"])
        # One item per cause: the new check (reported once), api, and shared.
        self.assertEqual([u["subject"] for u in issue.uncertain], ["newThing", "api", "shared"])
        self.assertEqual(issue.kind, "manual")
        self.assertIn("(manual review)", issue.title)
        self.assertIn("**Recommended action:** Resolve the 3 items under **Not mapped**", issue.sections["Summary"])
        impact = issue.sections["Kyverno impact"]
        self.assertAllIn(["- `newThing`: Upstream declares this check, but `checks`",
                          "- `api/`: Changes in `api/` can affect every check",
                          "- shared `policy/` code: Changes in shared `policy/` code or base fixtures"], impact)
        # Nothing maps to a template, so the impact lists none.
        self.assertNotIn("Template:", impact)
        self.assertAllIn(["- [ ] Map `newThing` to a policy in `checks`",
                          "- [ ] Read the `api/` changes in the comparison, and decide which of the 4 PSS CEL",
                          "- [ ] Read the shared `policy/` code and base fixture changes"],
                         issue.sections["Required actions"])
        self.assertAllIn(["- [ ] Each item under **Not mapped** is resolved",
                          "- [ ] After you change `checks`, `python3 scripts/test-psa-release-issues.py` passes.",
                          "If the review leads to a policy change"], issue.sections["Validation"])
        self.assertNotIn("- [ ] Update `", issue.body)

    def test_mapped_and_unmapped_changes_are_reported_without_contradiction(self):
        issue = self.change({"policy/check_sysctls.go": check_sysctls(values=["net.ipv4.new_value"]),
                             "policy/helpers.go": "package policy // changed\n"})
        impact = issue.sections["Kyverno impact"]
        # The mapped policy keeps its files and update, and the unmapped change is listed apart.
        self.assertIn(f"  - Template: `{self.SYSCTLS}`", impact)
        self.assertIn("- shared `policy/` code: Changes in shared `policy/` code", impact)
        self.assertEqual(issue.kind, "manual")
        self.assertAllIn(["- [ ] Update `", "  - [ ] Handle `net.ipv4.new_value`"], issue.sections["Required actions"])
        self.assertIn("- [ ] The PSS CLI tests pass with `make test-cli-local-vpols`.", issue.sections["Validation"])
        self.assertNotIn("If the review leads to a policy change", issue.sections["Validation"])

    def test_stale_mapping_and_renamed_check_need_manual_review(self):
        self.upstream.release("37.0", days_ago=3)
        self.upstream.run("mv", "policy/check_sysctls.go", "policy/check_sysctls_baseline.go")
        self.upstream.commit("Split sysctls")
        self.upstream.release("38.0", days_ago=2)
        issue = self.issue(config=dict(CONFIG, checks=dict(CONFIG["checks"], oldCheck="disallow-host-path")))
        self.assertEqual(issue.analysis.stale_checks, ["oldCheck"])
        problems = "\n".join(u["problem"] for u in issue.uncertain)
        self.assertIn("upstream no longer declares it", problems)
        self.assertIn("renamed or split", problems)
        self.assertAllIn(["- [ ] Find what replaced `oldCheck` upstream",
                          "- [ ] Confirm that `sysctls` still maps to `restrict-sysctls`"],
                         issue.sections["Required actions"])

    def test_mapped_policy_without_template_needs_manual_review(self):
        issue = self.change({"policy/check_capabilities_baseline.go": CHECK_CAPS + "// v2\n"},
                            config=dict(CONFIG, checks=dict(CONFIG["checks"], capabilities_baseline="gone-policy")))
        self.assertIn("no `gone-policy.cel.yaml` template exists", issue.uncertain[0]["problem"])
        # Missing mapping data never shows as a mapped file.
        self.assertNotIn("Template:", issue.sections["Kyverno impact"])
        self.assertIn("- [ ] Fix the policy name for `capabilities_baseline` in `checks`, or restore the template.",
                      issue.sections["Required actions"])

    def test_upstream_text_cannot_break_out_of_a_code_span(self):
        # A backtick in an upstream value would end the span and expose the mention.
        self.assertEqual(psa.code("x`@kyverno/maintainers`y"), "`x'@kyverno/maintainers'y`")
        issue = self.change({"policy/check_sysctls.go": check_sysctls(values=["net.ipv4.a`@someone`b"])})
        self.assertIn("added `net.ipv4.a'@someone'b`", issue.sections["What changed"])
        self.assertNotIn("`@someone`", issue.body)

    def test_upstream_code_stays_out_of_the_issue_and_files_link_to_the_comparison(self):
        issue = self.change({"policy/check_sysctls.go": check_sysctls() + "\n// ```\n// @someone\n"})
        # No diff in the body, so upstream code can't break the Markdown or ping anyone.
        self.assertNotIn("@someone", issue.body)
        self.assertNotIn("```", issue.body)
        compare = f"{TEST_MIRROR}/compare/kubernetes-1.37.0...kubernetes-1.38.0"
        anchor = hashlib.sha256(b"policy/check_sysctls.go").hexdigest()
        self.assertAllIn([f"- **Code:** [`policy/check_sysctls.go`]({compare}#diff-{anchor}) (M)",
                          f"See the [full comparison]({compare})."], issue.sections["Upstream evidence"])

    def test_a_test_mirror_is_marked_and_the_upstream_mirror_is_not(self):
        self.upstream.release("37.0", days_ago=3)
        self.upstream.release("38.0", days_ago=2, files={"policy/check_sysctls.go": check_sysctls(gate=39)})
        test = self.issue()
        self.assertIn(f"> This issue was generated from `{TEST_MIRROR}`, not `kubernetes/pod-security-admission`.",
                      test.sections["Summary"])
        real = self.issue(repository=psa.DEFAULT_REPOSITORY)
        for text in ["[!NOTE]", "example.com", "test release"]:
            self.assertNotIn(text, real.body)
        self.assertIn(f"({psa.DEFAULT_REPOSITORY}/compare/kubernetes-1.37.0...kubernetes-1.38.0)",
                      real.sections["Release details"])

    def test_published_time_is_utc(self):
        self.upstream.release("37.0", days_ago=3)
        self.upstream.release("38.0", days_ago=2, files={"policy/check_sysctls.go": check_sysctls(gate=39)})
        self.assertRegex(self.issue().sections["Release details"],
                         r"\| Published \| \d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC \|")
        india = timezone(timedelta(hours=5, minutes=30))
        self.assertEqual(psa.utc(datetime(2026, 8, 26, 16, 14, tzinfo=india)), "2026-08-26 10:44 UTC")
        self.assertEqual(psa.utc(None), "unknown")


class FakeGitHub(psa.GitHub):
    """Fakes the GitHub API behind GitHub.request, after the real allowlist check.

    fail maps a (method, path fragment) pair to an error message. on_create
    runs before each issue is stored, to simulate a concurrent run.
    """

    def __init__(self, bodies=(), fail=None, on_create=None, has_issues=True):
        super().__init__("owner/kyverno")
        self.store = [{"number": i + 1, "body": b, "state": "closed"} for i, b in enumerate(bodies)]
        self.labels, self.comments, self.calls = set(), [], []
        self.fail = fail or {}
        self.on_create = on_create
        self.has_issues = has_issues

    def add(self, body, **fields):
        issue = {"number": len(self.store) + 1, "body": body, "state": "open", **fields}
        self.store.append(issue)
        return issue

    def request(self, method, path, body=None, paginate=False):
        self.check_allowed(method, path, body)
        self.calls.append((method, path, body))
        for (fail_method, fragment), message in self.fail.items():
            if method == fail_method and fragment in path:
                raise psa.ReleaseError(message)
        if method == "GET" and path == f"repos/{self.repo}":
            return {"full_name": self.repo, "has_issues": self.has_issues}
        if method == "GET" and "/issues?" in path:
            return [dict(i) for i in self.store]
        if method == "GET" and "/labels/" in path:
            return {} if path.rsplit("/", 1)[1] in self.labels else None
        if method == "POST" and path.endswith("/labels"):
            self.labels.add(body["name"].replace("/", "%2F"))
            return {}
        if method == "POST" and path.endswith("/comments"):
            self.comments.append((int(path.split("/")[-2]), body["body"]))
            return {}
        if method == "POST" and path.endswith("/issues"):
            if self.on_create:
                self.on_create(self, body)
            issue = self.add(body["body"], title=body["title"], labels=body["labels"])
            return {"number": issue["number"], "html_url": f"https://github.com/owner/kyverno/issues/{issue['number']}"}
        if method == "PATCH":
            issue = self.store[int(path.rsplit("/", 1)[1]) - 1]
            issue.update(body)
            return {}
        raise AssertionError(f"unexpected {method} {path}")

    def created(self):
        return [i for i in self.store if "title" in i]


class RunTest(Base):
    """Runs the whole check against the local mirror and a fake GitHub."""

    V38_TITLE = "Pod Security Admission v1.38.0: update the PSS CEL policies"

    def setUp(self):
        super().setUp()
        self.upstream.release("36.0", days_ago=40)
        # The baseline. Every minor after it is checked, however old.
        self.upstream.release("37.0", days_ago=30)
        # A pre-release and a patch that both carry the sysctl change are skipped.
        self.upstream.release("38.0-alpha.1", days_ago=25, files={
            "policy/check_sysctls.go": check_sysctls(values=["net.ipv4.tcp_slow_start_after_idle"])})
        self.upstream.release("38.0", days_ago=20)
        self.upstream.release("37.1", days_ago=10, branch_from="kubernetes-1.38.0-alpha.1")
        self.upstream.release("36.1", days_ago=10, branch_from="kubernetes-1.36.0")

    def run_tool(self, github, **overrides):
        args = argparse.Namespace(repository=str(self.upstream.path), since="kubernetes-1.37.0",
                                  tag=None, dry_run=False, out=str(self.dir / "out"))
        vars(args).update(overrides)
        Path(args.out).mkdir(exist_ok=True)
        with redirect_stderr(io.StringIO()):
            return psa.run(args, github, self.chart, CONFIG)

    def test_checks_only_minor_releases_after_the_baseline(self):
        github = FakeGitHub()
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        # v1.38.0 is 20 days old: no time window can miss it.
        self.assertEqual([i["title"] for i in github.created()], [self.V38_TITLE])
        self.assertEqual(github.created()[0]["labels"], ["psa-cel-update", "area/pss"])
        self.assertEqual(github.labels, {"psa-cel-update", "area%2Fpss"})
        self.assertEqual(summary, [
            "- `v1.38.0` (since `v1.37.0`): opened [#1](https://github.com/owner/kyverno/issues/1)",
        ])

    def test_release_selection(self):
        github = FakeGitHub()
        # A later baseline leaves nothing to check.
        summary, _ = self.run_tool(github, since="kubernetes-1.38.0")
        self.assertEqual((github.created(), summary), ([], []))
        # --tag checks one release, whatever the baseline.
        self.run_tool(github, tag="kubernetes-1.38.0", since="kubernetes-1.38.0")
        self.assertEqual(len(github.created()), 1)
        with self.assertRaisesRegex(psa.ReleaseError, "baseline tag kubernetes-1.99.0 isn't in"):
            self.run_tool(github, since="kubernetes-1.99.0")
        with self.assertRaisesRegex(psa.ReleaseError, "tag kubernetes-1.99.0 isn't in"):
            self.run_tool(github, tag="kubernetes-1.99.0")

    def test_an_open_or_closed_issue_or_a_rerun_never_duplicates(self):
        marker = psa.marker(psa.Release("kubernetes-1.38.0"))
        github = FakeGitHub(bodies=[f"Closed earlier.\n{marker}"])
        summary, _ = self.run_tool(github)
        self.assertEqual(github.created(), [])
        self.assertIn("- `v1.38.0`: issue already exists (#1)", summary)
        github = FakeGitHub()
        for _ in range(3):
            self.run_tool(github)
        self.assertEqual(len(github.created()), 1)

    def test_a_marker_in_upstream_text_does_not_block_another_release(self):
        # The v1.39.0 issue quotes this subject, which holds the v1.40.0 marker.
        spoof = f"Note {psa.marker(psa.Release('kubernetes-1.40.0'))}"
        self.upstream.release("39.0", files={"policy/check_sysctls.go": check_sysctls(values=["net.ipv4.a"])},
                              message=spoof)
        self.upstream.release("40.0", files={"policy/check_sysctls.go": check_sysctls(values=["net.ipv4.b"])})
        github = FakeGitHub()
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        self.assertEqual([i["title"].split()[3].rstrip(":") for i in github.created()],
                         ["v1.38.0", "v1.39.0", "v1.40.0"])

    def test_a_concurrent_issue_before_create_is_reused(self):
        github = FakeGitHub()
        real_issues = github.issues
        calls = []

        def issues():
            calls.append(1)
            # Another run opens the issue after this run's first listing.
            if len(calls) == 2:
                github.add(f"Opened elsewhere.\n{psa.marker(psa.Release('kubernetes-1.38.0'))}")
            return real_issues()

        github.issues = issues
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        self.assertEqual(github.created(), [])
        self.assertIn("- `v1.38.0` (since `v1.37.0`): issue #1 (opened by another run meanwhile)", summary)

    def test_a_concurrent_create_closes_the_later_issue(self):
        def race(fake, body):
            # A concurrent run creates the same issue at the same moment, and wins.
            fake.add(body["body"], title=body["title"])

        github = FakeGitHub(on_create=race)
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        first, second = github.store
        self.assertEqual(first["state"], "open")
        self.assertEqual((second["state"], second["state_reason"]), ("closed", "duplicate"))
        self.assertEqual(github.comments, [(2, "Duplicate of #1, which another run of this workflow opened first.")])
        self.assertTrue(any("closed the duplicate #2 that a concurrent run caused" in line for line in summary))

    def test_issues_turned_off_fail_every_run_before_any_work(self):
        # A real fork run hit this: listing worked, but creating failed with HTTP 410.
        for dry_run in (True, False):
            github = FakeGitHub(has_issues=False)
            with self.assertRaisesRegex(psa.ReleaseError, "Issues are turned off in owner/kyverno"):
                self.run_tool(github, dry_run=dry_run)
            self.assertEqual([(m, p) for m, p, _ in github.calls], [("GET", "repos/owner/kyverno")])
            self.assertFalse((self.dir / "out/kubernetes-1.38.0.md").exists())

    def test_dry_run_writes_the_issue_and_changes_nothing(self):
        github = FakeGitHub()
        summary, _ = self.run_tool(github, dry_run=True)
        # Only reads: the repository settings and the issue list.
        self.assertEqual([m for m, _, _ in github.calls], ["GET", "GET"])
        body = (self.dir / "out/kubernetes-1.38.0.md").read_text(encoding="utf-8")
        self.assertIn("net.ipv4.tcp_slow_start_after_idle", body)
        self.assertIn("- `v1.38.0` (since `v1.37.0`): would open an issue (dry run, see kubernetes-1.38.0.md)",
                      summary)

    def test_a_release_without_the_previous_minor_fails_loudly(self):
        self.upstream.release("40.0", days_ago=1)
        github = FakeGitHub()
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, ["kubernetes-1.40.0"])
        self.assertIn("- `v1.40.0`: **error**: no previous release to compare with; check this release by hand",
                      summary)

    def test_labels_follow_the_issue_kind(self):
        # Test-only: an issue to confirm, without the manual-review label.
        self.upstream.release("39.0", days_ago=5, files={"policy/check_sysctls_test.go": "package policy // v2\n"})
        # Shared code: manual review.
        self.upstream.release("40.0", days_ago=4, files={"policy/helpers.go": "package policy // changed\n"})
        github = FakeGitHub()
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        by_tag = {i["title"].split()[3].rstrip(":"): i for i in github.created()}
        self.assertEqual(by_tag["v1.38.0"]["labels"], ["psa-cel-update", "area/pss"])
        self.assertEqual(by_tag["v1.39.0"]["labels"], ["psa-cel-update", "area/pss"])
        self.assertIn("confirm the PSS CEL policies need no change", by_tag["v1.39.0"]["title"])
        self.assertEqual(by_tag["v1.40.0"]["labels"], ["psa-cel-update", "area/pss", "needs-manual-review"])
        self.assertIn("needs-manual-review", github.labels)
        self.assertTrue(any("v1.39.0" in line and "(confirm only)" in line for line in summary))
        self.assertTrue(any("v1.40.0" in line and "(manual review)" in line for line in summary))

    def test_failed_create_is_reported_and_the_rest_continue(self):
        self.upstream.release("39.0", days_ago=5, files={"policy/check_sysctls.go": check_sysctls(
            values=["net.ipv4.tcp_slow_start_after_idle", "net.ipv4.other"])})
        calls = []

        def fail_first(fake, body):
            calls.append(body["title"])
            if len(calls) == 1:
                raise psa.ReleaseError("gh api POST repos/owner/kyverno/issues failed: HTTP 502")

        github = FakeGitHub(on_create=fail_first)
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, ["kubernetes-1.38.0"])
        self.assertEqual([i["title"] for i in github.created()],
                         ["Pod Security Admission v1.39.0: update the PSS CEL policies"])
        self.assertTrue(any("**error**: gh api POST" in line and "HTTP 502" in line for line in summary))
        # The next run retries the failed release.
        github.on_create = None
        _, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        self.assertEqual(len(github.created()), 2)

    def test_failed_label_creation_fails_the_release(self):
        github = FakeGitHub(fail={("POST", "/labels"): "HTTP 403: Resource not accessible by integration"})
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, ["kubernetes-1.38.0"])
        self.assertEqual(github.created(), [])
        self.assertTrue(any("HTTP 403" in line for line in summary))

    def test_failed_issue_listing_stops_the_run(self):
        github = FakeGitHub(fail={("GET", "/issues?"): "HTTP 500"})
        with self.assertRaisesRegex(psa.ReleaseError, "HTTP 500"):
            self.run_tool(github)
        self.assertEqual(github.created(), [])

    def test_caps_issues_per_run(self):
        for minor in range(39, 39 + psa.MAX_ISSUES_PER_RUN):
            self.upstream.release(f"{minor}.0", days_ago=5,
                                  files={"policy/check_sysctls.go": check_sysctls(values=[f"net.ipv4.value{minor}"])})
        github = FakeGitHub()
        summary, _ = self.run_tool(github)
        self.assertEqual(len(github.created()), psa.MAX_ISSUES_PER_RUN)
        self.assertTrue(any("waits for the next run" in line for line in summary))
        # The next run picks up the rest.
        self.run_tool(github)
        self.assertEqual(len(github.created()), psa.MAX_ISSUES_PER_RUN + 1)

    def test_an_issue_another_run_opened_does_not_count_toward_the_cap(self):
        for minor in range(39, 39 + psa.MAX_ISSUES_PER_RUN):
            self.upstream.release(f"{minor}.0", days_ago=5,
                                  files={"policy/check_sysctls.go": check_sysctls(values=[f"net.ipv4.value{minor}"])})
        github = FakeGitHub()
        real_issues = github.issues
        calls = []

        def issues():
            calls.append(1)
            # Another run opens the v1.38.0 issue after this run's first listing.
            if len(calls) == 2:
                github.add(f"Opened elsewhere.\n{psa.marker(psa.Release('kubernetes-1.38.0'))}")
            return real_issues()

        github.issues = issues
        summary, failures = self.run_tool(github)
        self.assertEqual(failures, [])
        self.assertEqual(len(github.created()), psa.MAX_ISSUES_PER_RUN)
        self.assertFalse(any("waits for the next run" in line for line in summary))


class GitHubTest(unittest.TestCase):
    def test_allows_only_issue_reads_creates_and_duplicate_closing(self):
        github = psa.GitHub("owner/kyverno")
        for method, path, body in [
            ("DELETE", "repos/owner/kyverno/issues/1", None),
            ("PATCH", "repos/owner/kyverno/issues/1", {"title": "changed"}),
            ("PATCH", "repos/owner/kyverno/issues/1", {"state": "open"}),
            ("POST", "repos/owner/kyverno/issues/1", {}),
            ("POST", "repos/owner/kyverno/pulls", {}),
            ("GET", "repos/other/repo/issues", None),
        ]:
            with self.assertRaises(psa.ReleaseError, msg=f"{method} {path} {body}"):
                github.check_allowed(method, path, body)
        github.check_allowed("PATCH", "repos/owner/kyverno/issues/1", {"state": "closed", "state_reason": "duplicate"})
        github.check_allowed("POST", "repos/owner/kyverno/issues/1/comments", {"body": "x"})
        github.check_allowed("GET", "repos/owner/kyverno/issues?labels=psa-cel-update&state=all", None)
        # The repository itself can be read, to check that Issues are on, but never changed.
        github.check_allowed("GET", "repos/owner/kyverno", None)
        for method in ("PATCH", "POST", "DELETE"):
            with self.assertRaises(psa.ReleaseError):
                github.check_allowed(method, "repos/owner/kyverno", {"has_issues": True})
        with self.assertRaises(psa.ReleaseError):
            psa.GitHub("not a repo")


class GhCommandTest(unittest.TestCase):
    """Runs the real GitHub.request with a faked `gh` process."""

    def run_gh(self, returncode=0, stdout="", stderr="", **kwargs):
        calls = []

        def fake(args):
            calls.append(args)
            return SimpleNamespace(returncode=returncode, stdout=stdout, stderr=stderr)

        with mock.patch.object(psa, "run_command", fake):
            result = psa.GitHub("owner/kyverno").request(**kwargs)
        return result, calls[0]

    def test_pagination_parses_one_issue_per_line_across_pages(self):
        # `gh api --paginate --jq '.[] | @json'` prints one JSON object per line, for every page.
        pages = "\n".join(json.dumps({"number": n, "body": f"body {n}\nsecond line"}) for n in range(1, 151))
        issues, command = self.run_gh(stdout=pages + "\n", method="GET", paginate=True,
                                      path="repos/owner/kyverno/issues?labels=psa-cel-update&state=all&per_page=100")
        self.assertEqual(len(issues), 150)
        self.assertEqual(issues[149]["body"], "body 150\nsecond line")
        self.assertEqual(command[:4], ["gh", "api", "-X", "GET"])
        self.assertIn("--paginate", command)
        self.assertEqual(command[command.index("--jq") + 1], ".[] | @json")

    def test_issues_skips_pull_requests(self):
        lines = [json.dumps({"number": 1, "body": "a"}), json.dumps({"number": 2, "body": "b", "pull_request": {}})]
        with mock.patch.object(psa, "run_command", lambda args: SimpleNamespace(
                returncode=0, stdout="\n".join(lines), stderr="")):
            self.assertEqual(psa.GitHub("owner/kyverno").issues(), [{"number": 1, "body": "a"}])

    def test_a_missing_label_is_none_and_other_errors_raise(self):
        result, _ = self.run_gh(returncode=1, stderr="gh: Not Found (HTTP 404)", method="GET",
                                path="repos/owner/kyverno/labels/psa-cel-update")
        self.assertIsNone(result)
        with self.assertRaisesRegex(psa.ReleaseError, "HTTP 502"):
            self.run_gh(returncode=1, stderr="gh: Bad Gateway (HTTP 502)", method="GET",
                        path="repos/owner/kyverno/labels/psa-cel-update")
        # A 404 on a write is an error, not "missing".
        with self.assertRaisesRegex(psa.ReleaseError, "HTTP 404"):
            self.run_gh(returncode=1, stderr="gh: Not Found (HTTP 404)", method="POST",
                        path="repos/owner/kyverno/issues", body={"title": "t"})

    def test_output_is_decoded_as_utf8_whatever_the_locale(self):
        # A real run against kyverno/kyverno issues failed on a Windows cp1252 locale before this.
        script = "import sys; sys.stdout.buffer.write('caf\\u00e9 \\u201cquoted\\u201d \\u2014 \\U0001f680'.encode())"
        result = psa.run_command([sys.executable, "-c", script])
        self.assertEqual(result.stdout, "café “quoted” — \U0001f680")

    def test_a_body_goes_through_a_file_not_the_command_line(self):
        _, command = self.run_gh(stdout="{}", method="POST", path="repos/owner/kyverno/issues",
                                 body={"title": "t", "body": "$(rm -rf /)"})
        self.assertIn("--input", command)
        self.assertNotIn("$(rm -rf /)", " ".join(command))


class WorkflowTest(unittest.TestCase):
    """Checks the workflow file for the security and safety rules it relies on."""

    @classmethod
    def setUpClass(cls):
        cls.path = ROOT / ".github/workflows/psa-release-issues.yaml"
        cls.workflow = yaml.safe_load(cls.path.read_text(encoding="utf-8"))
        # PyYAML reads the `on` key as True.
        cls.on = cls.workflow.get("on", cls.workflow.get(True))
        cls.jobs = cls.workflow["jobs"]

    def steps(self):
        return [step for job in self.jobs.values() for step in job.get("steps", [])]

    def test_least_privilege_permissions(self):
        self.assertEqual(self.workflow["permissions"], {})
        self.assertEqual(self.jobs["test"]["permissions"], {"contents": "read"})
        self.assertEqual(self.jobs["check"]["permissions"], {"contents": "read", "issues": "write"})
        self.assertEqual(self.jobs["sync-issue"]["permissions"], {"contents": "read", "issues": "write"})
        # No secret other than the built-in token.
        text = self.path.read_text(encoding="utf-8")
        self.assertNotIn("secrets.", text)

    def test_actions_are_pinned_and_checkouts_drop_credentials(self):
        for step in self.steps():
            uses = step.get("uses")
            # Local actions load from the repository itself, so they have no SHA.
            if not uses or uses.startswith(("./", "$/")):
                continue
            self.assertRegex(uses, r"^[\w.-]+/[\w./-]+@[0-9a-f]{40}$", uses)
            if uses.startswith("actions/checkout@"):
                self.assertIs(step["with"]["persist-credentials"], False)

    def test_every_job_has_a_timeout(self):
        for name, job in self.jobs.items():
            self.assertIsInstance(job.get("timeout-minutes"), int, name)

    def test_run_blocks_take_inputs_only_through_the_environment(self):
        # ${{ }} inside run: is a script injection risk.
        for step in self.steps():
            self.assertNotIn("${{", step.get("run", ""), step.get("name"))

    def test_safe_defaults(self):
        inputs = self.on["workflow_dispatch"]["inputs"]
        self.assertIs(inputs["dry-run"]["default"], True)
        # Quarterly: minor releases come out about every 4 months.
        self.assertEqual(self.on["schedule"], [{"cron": "23 6 1 1,4,7,10 *"}])
        check = self.jobs["check"]
        # Scheduled runs are opt-in, and only the default branch opens issues.
        self.assertIn("vars.PSA_CEL_UPDATE_ISSUES == 'true'", check["if"])
        self.assertIn("github.ref_name != github.event.repository.default_branch", check["env"]["DRY_RUN"])
        self.assertNotIn("pull_request", check["if"])
        self.assertEqual(check["needs"], ["test"])
        # The test-only repository input is empty by default, so scheduled runs read the upstream mirror.
        self.assertEqual(inputs["repository"]["default"], "")
        step = next(s for s in check["steps"] if s.get("name") == "Check releases")
        self.assertEqual(step["env"]["REPOSITORY"], "${{ inputs.repository }}")
        self.assertIn('[ -n "$REPOSITORY" ] && args+=(--repository "$REPOSITORY")', step["run"])

    def test_one_check_job_at_a_time_per_repository(self):
        concurrency = self.jobs["check"]["concurrency"]
        # The group must not include the ref, or runs of two branches could race.
        self.assertEqual(concurrency["group"], "psa-cel-update-issues-${{ github.repository }}")
        self.assertIs(concurrency["cancel-in-progress"], False)
        # Without it, a newly queued run cancels the pending one, and its tag is never checked.
        self.assertEqual(concurrency["queue"], "max")

    def test_local_action_loads_from_the_running_commit(self):
        # $/ loads the action at the running commit, not from files in the workspace.
        sync = next(s for s in self.jobs["sync-issue"]["steps"] if s.get("name") == "Sync issue")
        self.assertEqual(sync["uses"], "$/.github/actions/workflow/failure-issue")

    def test_pull_requests_run_the_tests_for_every_input_file(self):
        paths = set(self.on["pull_request"]["paths"])
        self.assertEqual(paths, {"scripts/psa-release-issues.py", "scripts/test-psa-release-issues.py",
                                 "scripts/psa-release-issues.yaml", ".github/workflows/psa-release-issues.yaml",
                                 "charts/kyverno-policies/**", "test/cli/test-validating-policy/**"})


class MainTest(unittest.TestCase):
    def test_accepts_only_a_github_repository_url(self):
        for url in ["file:///tmp/psa", "ext::sh -c id", "https://gitlab.com/owner/psa",
                    "https://github.com/owner/psa/extra", "https://github.com/owner", "http://github.com/owner/psa"]:
            with tempfile.TemporaryDirectory() as tmp, redirect_stdout(io.StringIO()) as out:
                code = psa.main(["--repo", "owner/kyverno", "--out", tmp, "--repository", url])
            self.assertEqual(code, 1, url)
            self.assertIn("must look like https://github.com/OWNER/REPO", out.getvalue(), url)

    def test_committed_config_maps_every_check_to_a_template(self):
        config = psa.load_config(psa.CONFIG_FILE)
        self.assertTrue(psa.TAG_RE.match(config["since"]))
        chart = psa.Chart(ROOT)
        missing = [p for p in config["checks"].values() if not chart.lookup(p)]
        self.assertEqual(missing, [])
        # The Helm validation step names the chart's real minimum version.
        self.assertRegex(chart.min_kube or "", r"^1\.\d+$")

    def test_rejects_an_invalid_config(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "config.yaml"
            for text, error in [
                ("- since\n", "must be a map"),
                ("kubernetes-1.37.0\n", "must be a map"),
                ("since: v0.37.0\nchecks: {sysctls: restrict-sysctls}\n", "since must be"),
                # Only a minor release can be the baseline.
                ("since: kubernetes-1.37.1\nchecks: {sysctls: restrict-sysctls}\n", "since must be"),
                ("since: kubernetes-1.37.0\n", "checks must be a non-empty map"),
                ("since: kubernetes-1.37.0\nchecks: {sysctls: ''}\n", "must map to a policy name"),
                ("since: kubernetes-1.37.0\nchecks: {sysctls: a, Sysctls: b}\n", "unique ignoring case"),
            ]:
                path.write_text(text, encoding="utf-8")
                with self.assertRaisesRegex(psa.ReleaseError, error):
                    psa.load_config(path)


if __name__ == "__main__":
    unittest.main()
