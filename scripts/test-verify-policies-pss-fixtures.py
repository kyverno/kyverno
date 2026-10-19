#!/usr/bin/env python3
"""Offline tests for scripts/verify-policies-pss-fixtures.py (requires PyYAML).

The tests need no network, helm, or kubectl-kyverno: commands are faked by
replacing run_command.
"""

import importlib.util
import io
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from unittest import mock

import yaml


ROOT = Path(__file__).resolve().parents[1]
# Keep the import from writing scripts/__pycache__.
sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location(
    "verify_pss_fixtures", ROOT / "scripts/verify-policies-pss-fixtures.py"
)
verify = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(verify)

CHECKS = {"sysctls": "restrict-sysctls", "procMount": "disallow-proc-mount",
          "procMount_restricted": "disallow-proc-mount-strict"}
RUN = verify.Run("baseline", 38, False)


def pod(name, volumes=None):
    doc = {"apiVersion": "v1", "kind": "Pod", "metadata": {"name": name},
           "spec": {"containers": [{"name": "c", "image": "registry.k8s.io/pause"}]}}
    if volumes is not None:
        doc["spec"]["volumes"] = volumes
    return doc


def finding(fixture, kind="pass", run=RUN, error=False):
    return verify.Finding(run, kind, fixture, "differs", error=error)


class CheckIDTest(unittest.TestCase):
    def test_strips_number_and_lowercases(self):
        self.assertEqual(verify.check_id("sysctls0"), "sysctls")
        self.assertEqual(verify.check_id("procmount_restricted12"), "procmount_restricted")
        self.assertEqual(verify.check_id("base_linux"), "base_linux")


class NormalizePodTest(unittest.TestCase):
    def test_volume_without_source_gets_empty_dir(self):
        doc = pod("p", [{"name": "v0"}, {"name": "v1", "secret": {"secretName": "s"}}])
        self.assertEqual(verify.normalize_pod(doc), 1)
        self.assertEqual(doc["spec"]["volumes"][0], {"name": "v0", "emptyDir": {}})
        self.assertEqual(doc["spec"]["volumes"][1], {"name": "v1", "secret": {"secretName": "s"}})

    def test_pod_without_volumes_is_unchanged(self):
        doc = pod("p")
        self.assertEqual(verify.normalize_pod(doc), 0)
        self.assertNotIn("volumes", doc["spec"])

    def test_null_sources_count_as_no_source(self):
        doc = pod("p", [{"name": "v0", "hostPath": None}, {"name": "v1", "hostPath": {"path": "/"}}])
        self.assertEqual(verify.normalize_pod(doc), 1)
        self.assertEqual(doc["spec"]["volumes"][0], {"name": "v0", "emptyDir": {}})
        self.assertEqual(doc["spec"]["volumes"][1], {"name": "v1", "hostPath": {"path": "/"}})


class PrepareFixturesTest(unittest.TestCase):
    def test_writes_normalized_fixtures_and_returns_names(self):
        with tempfile.TemporaryDirectory() as tmp:
            src, dst = Path(tmp) / "src", Path(tmp) / "dst"
            src.mkdir()
            (src / "restrictedvolumes0.yaml").write_text(yaml.safe_dump(pod("restrictedvolumes0", [{"name": "v0"}])))
            names = verify.prepare_fixtures(src, dst)
            self.assertEqual(names, ["restrictedvolumes0"])
            written = yaml.safe_load((dst / "restrictedvolumes0.yaml").read_text(encoding="utf-8"))
            self.assertEqual(written["spec"]["volumes"], [{"name": "v0", "emptyDir": {}}])

    def test_empty_directory_is_an_error(self):
        with tempfile.TemporaryDirectory() as tmp:
            (Path(tmp) / "src").mkdir()
            with self.assertRaises(verify.VerifyError):
                verify.prepare_fixtures(Path(tmp) / "src", Path(tmp) / "dst")


class ResolveVersionsTest(unittest.TestCase):
    def test_latest_and_deduplication(self):
        minors, errors = verify.resolve_versions(["1.25", "1.38", "latest"], {25, 37, 38})
        self.assertEqual(minors, [25, 38])
        self.assertEqual(errors, [])

    def test_missing_version_is_an_error(self):
        minors, errors = verify.resolve_versions(["1.99"], {38})
        self.assertEqual(minors, [])
        self.assertEqual(errors, ["no fixture directory for v1.99"])

    def test_available_minors_needs_every_level(self):
        with tempfile.TemporaryDirectory() as tmp:
            for path in ("baseline/v1.37", "baseline/v1.38", "restricted/v1.38"):
                (Path(tmp) / path).mkdir(parents=True)
            self.assertEqual(verify.available_minors(Path(tmp)), {38})


class CompareTest(unittest.TestCase):
    rendered = {"restrict-sysctls", "disallow-proc-mount"}

    def compare(self, kind, results, rendered=None):
        return verify.compare(RUN, kind, sorted(results), results,
                              self.rendered if rendered is None else rendered, CHECKS)

    def test_pass_fixture_that_passes(self):
        self.assertEqual(self.compare("pass", {"base": {"restrict-sysctls": "pass"}}), [])

    def test_pass_fixture_rejected_or_errored(self):
        findings = self.compare("pass", {"sysctls1": {"restrict-sysctls": "fail"},
                                         "base": {"disallow-proc-mount": "error"}})
        self.assertEqual([(f.fixture, f.error) for f in findings], [("base", False), ("sysctls1", False)])
        self.assertEqual(findings[0].message, "errored on disallow-proc-mount")
        self.assertEqual(findings[1].message, "rejected by restrict-sysctls")

    def test_fail_fixture_caught_by_mapped_policy(self):
        self.assertEqual(self.compare("fail", {"sysctls0": {"restrict-sysctls": "fail"}}), [])

    def test_fail_fixture_missed(self):
        findings = self.compare("fail", {"sysctls0": {"restrict-sysctls": "pass", "disallow-proc-mount": "fail"}})
        self.assertEqual(len(findings), 1)
        self.assertEqual(findings[0].message, "expected restrict-sysctls to fail, got disallow-proc-mount")

    def test_mapped_policy_error_is_not_a_rejection(self):
        findings = self.compare("fail", {"sysctls0": {"restrict-sysctls": "error"}})
        self.assertEqual([f.message for f in findings],
                         ["errored on restrict-sysctls", "expected restrict-sysctls to fail, got none"])

    def test_unrendered_mapped_policy_accepts_any_failure(self):
        results = {"procmount_restricted0": {"disallow-proc-mount": "fail"}}
        self.assertEqual(self.compare("fail", results), [])
        results = {"procmount_restricted0": {"disallow-proc-mount": "pass"}}
        self.assertIn("isn't rendered", self.compare("fail", results)[0].message)

    def test_error_never_satisfies_unrendered_fallback(self):
        findings = self.compare("fail", {"procmount_restricted0": {"disallow-proc-mount": "error"}})
        self.assertEqual([f.message for f in findings],
                         ["errored on disallow-proc-mount",
                          "expected a failure (disallow-proc-mount-strict isn't rendered), got none"])

    def test_error_on_fail_fixture_is_reported(self):
        findings = self.compare("fail", {"sysctls0": {"restrict-sysctls": "fail", "disallow-proc-mount": "error"}})
        self.assertEqual([f.message for f in findings], ["errored on disallow-proc-mount"])

    def test_unmapped_check_id_is_an_error(self):
        findings = self.compare("fail", {"newcheck0": {"restrict-sysctls": "pass"}})
        self.assertTrue(findings[0].error)
        self.assertIn('unmapped check ID "newcheck"', findings[0].message)

    def test_fixture_without_result_is_an_error(self):
        findings = verify.compare(RUN, "pass", ["base"], {}, self.rendered, CHECKS)
        self.assertTrue(findings[0].error)


class KnownDifferencesTest(unittest.TestCase):
    entry = {"level": "baseline", "kind": "pass", "fixtures": ["procmount1"], "minVersion": "1.35",
             "values": {"podSecurityUserNamespaces": False}, "reason": "opt-in"}

    def test_matching_finding_is_known(self):
        unexpected, known, stale = verify.apply_known_differences([finding("procmount1")], [self.entry])
        self.assertEqual((unexpected, len(known), stale), ([], 1, []))

    def test_version_range_and_values_must_match(self):
        old = finding("procmount1", run=verify.Run("baseline", 34, False))
        userns = finding("procmount1", run=verify.Run("baseline", 38, True))
        unexpected, known, stale = verify.apply_known_differences([old, userns], [self.entry])
        self.assertEqual(unexpected, [old, userns])
        self.assertEqual(known, [])
        self.assertEqual(len(stale), 1)

    def test_errors_are_never_excused(self):
        error = finding("procmount1", error=True)
        unexpected, _, _ = verify.apply_known_differences([error], [self.entry])
        self.assertEqual(unexpected, [error])

    def test_each_listed_fixture_must_match(self):
        entry = dict(self.entry, fixtures=["procmount0", "procmount1"])
        _, _, stale = verify.apply_known_differences([finding("procmount1")], [entry])
        self.assertEqual(stale, ["knownDifferences[0]: baseline/pass/procmount0 matched nothing"])


class ConfigTest(unittest.TestCase):
    base = {"checks": dict(CHECKS), "versions": ["1.25", "latest"], "knownDifferences": []}

    def assertInvalid(self, **changes):
        with self.assertRaises(verify.VerifyError):
            verify.load_config(dict(self.base, **changes))

    def test_valid(self):
        self.assertEqual(verify.load_config(self.base)["versions"], ["1.25", "latest"])

    def test_invalid(self):
        self.assertInvalid(checks={})
        self.assertInvalid(checks={"sysctls": "a", "SYSCTLS": "b"})
        self.assertInvalid(versions=["v1.25"])
        entry = KnownDifferencesTest.entry
        self.assertInvalid(knownDifferences=[dict(entry, reason=" ")])
        self.assertInvalid(knownDifferences=[dict(entry, level="privileged")])
        self.assertInvalid(knownDifferences=[dict(entry, kind="skip")])
        self.assertInvalid(knownDifferences=[dict(entry, fixtures=[])])
        self.assertInvalid(knownDifferences=[dict(entry, maxVersion="latest")])
        self.assertInvalid(knownDifferences=[dict(entry, values={"policyType": "ClusterPolicy"})])

    def test_pin_needs_full_sha(self):
        with self.assertRaises(verify.VerifyError):
            verify.load_pin({"repository": "https://example.com/r", "commit": "36a049e"})

    def test_pin_needs_https_repository(self):
        for url in ("ext::sh -c id", "file:///tmp/repo", "git@github.com:k/p.git"):
            with self.assertRaises(verify.VerifyError):
                verify.load_pin({"repository": url, "commit": "0" * 40})


class ParseReportTest(unittest.TestCase):
    def test_results_by_fixture_and_policy(self):
        report = {"results": [
            {"policy": "restrict-sysctls", "result": "fail", "resources": [{"name": "sysctls0"}]},
            {"policy": "disallow-proc-mount", "result": "pass", "resources": [{"name": "sysctls0"}]},
        ]}
        self.assertEqual(verify.parse_report(json.dumps(report)),
                         {"sysctls0": {"restrict-sysctls": "fail", "disallow-proc-mount": "pass"}})

    def test_unparsable_report_is_an_error(self):
        with self.assertRaises(verify.VerifyError):
            verify.parse_report("not json")

    def test_real_cli_report_shape(self):
        # Captured from kubectl-kyverno apply --policy-report --output-format json.
        # The OpenReports Go field is Subjects, but its JSON name is "resources".
        report = """{"kind": "ClusterReport", "apiVersion": "openreports.io/v1alpha1", "results": [
            {"source": "KyvernoValidatingPolicy", "policy": "restrict-sysctls", "rule": "validation",
             "category": "Pod Security Standards (Baseline)", "severity": "medium",
             "timestamp": {"seconds": 1792307753, "nanos": 0}, "result": "fail", "scored": true,
             "resources": [{"kind": "Pod", "namespace": "default", "name": "sysctls0", "apiVersion": "v1"}],
             "message": "Sysctls must be restricted to safe values.",
             "properties": {"cel.validationIndex": "0", "process": "background scan"}}]}"""
        self.assertEqual(verify.parse_report(report), {"sysctls0": {"restrict-sysctls": "fail"}})


class RunCommandTest(unittest.TestCase):
    def test_missing_command_is_named(self):
        with self.assertRaises(verify.VerifyError) as ctx:
            verify.run_command(["pss-fixtures-no-such-command"])
        self.assertEqual(str(ctx.exception), "command not found: pss-fixtures-no-such-command")

    def test_command_that_cant_run_is_named(self):
        # Windows raises OSError (WinError 193) for a binary built for another OS.
        error = OSError(8, "Exec format error")
        with mock.patch.object(verify.subprocess, "run", side_effect=error):
            with self.assertRaises(verify.VerifyError) as ctx:
                verify.run_command(["/tools/helm", "template"])
        self.assertTrue(str(ctx.exception).startswith("can't run /tools/helm: "), str(ctx.exception))
        self.assertIn("Exec format error", str(ctx.exception))


class FetchErrorTest(unittest.TestCase):
    def test_git_never_prompts_for_credentials(self):
        completed = subprocess.CompletedProcess([], 0, "", "")
        with mock.patch.object(verify.subprocess, "run", return_value=completed) as run:
            verify.run_command(["git", "fetch"])
        self.assertEqual(run.call_args.kwargs["env"]["GIT_TERMINAL_PROMPT"], "0")

    def test_missing_repository_is_explained(self):
        # What git prints on a GitHub runner for a repository that doesn't exist.
        stderr = "fatal: could not read Username for 'https://github.com': No such device or address\n"
        message = verify.describe_fetch_error(stderr, "https://github.com/x/missing", "commit abc")
        self.assertIn("can't fetch commit abc from https://github.com/x/missing", message)
        self.assertIn("doesn't exist, isn't public", message)

    def test_other_errors_keep_the_git_output(self):
        message = verify.describe_fetch_error("fatal: unable to access: SSL error\n", "https://example.com/r",
                                              "commit c")
        self.assertEqual(message, "can't fetch commit c from https://example.com/r: fatal: unable to access: SSL error")


class RepositoryDataTest(unittest.TestCase):
    """Checks the real data files against the chart."""

    def test_mapping_and_pin_are_valid(self):
        config = verify.load_config(yaml.safe_load((verify.DATA_DIR / "mapping.yaml").read_text(encoding="utf-8")))
        verify.load_pin(yaml.safe_load((verify.DATA_DIR / "upstream.yaml").read_text(encoding="utf-8")))
        self.assertEqual(len(config["checks"]), 19)
        self.assertIn("latest", config["versions"])

    def test_every_mapped_policy_has_a_chart_template(self):
        config = verify.load_config(yaml.safe_load((verify.DATA_DIR / "mapping.yaml").read_text(encoding="utf-8")))
        templates = ROOT / "charts/kyverno-policies/templates"
        names = set()
        for path in templates.glob("*/*.cel.yaml"):
            match = re.search(r'\$name := "([^"]+)"', path.read_text(encoding="utf-8"))
            if match:
                names.add(match.group(1))
        missing = sorted(set(config["checks"].values()) - names)
        self.assertEqual(missing, [], f"mapped policies without a .cel.yaml template: {missing}")


class VerifyTest(unittest.TestCase):
    """Runs verify() end to end with fake helm and kubectl-kyverno commands."""

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.fixtures = Path(self.tmp.name)
        for level in verify.LEVELS:
            for kind, names in (("pass", ["base", "sysctls1"]), ("fail", ["sysctls0"])):
                path = self.fixtures / level / "v1.38" / kind
                path.mkdir(parents=True)
                for name in names:
                    (path / f"{name}.yaml").write_text(yaml.safe_dump(pod(name)))
        # Rejected pass fixtures, by fixture name.
        self.rejected = set()

    def tearDown(self):
        self.tmp.cleanup()

    def fake_run(self, args):
        if args[1] == "template":
            policy = {"apiVersion": "policies.kyverno.io/v1beta1", "kind": "ValidatingPolicy",
                      "metadata": {"name": "restrict-sysctls"}}
            return subprocess.CompletedProcess(args, 0, yaml.safe_dump(policy), "")
        resources = Path(args[args.index("--resource") + 1])
        results = []
        for path in resources.glob("*.yaml"):
            name = path.stem
            failed = name in self.rejected or resources.name == "fail"
            results.append({"policy": "restrict-sysctls", "result": "fail" if failed else "pass",
                            "resources": [{"name": name}]})
        return subprocess.CompletedProcess(args, 1, json.dumps({"results": results}), "")

    def run_verify(self, known=()):
        args = mock.Mock(cli="kubectl-kyverno", helm="helm", chart="chart", fixtures=str(self.fixtures), jobs=2,
                         config={"checks": {"sysctls": "restrict-sysctls"}, "versions": ["latest"],
                                 "knownDifferences": list(known)})
        out = io.StringIO()
        with mock.patch.object(verify, "run_command", self.fake_run), redirect_stdout(out):
            code = verify.verify(args)
        return code, out.getvalue()

    def test_all_match(self):
        code, out = self.run_verify()
        self.assertEqual(code, 0, out)
        self.assertIn("checked 12 fixture results across 4 renders: 0 unexpected, 0 known, 0 stale", out)

    def test_unexpected_difference_fails(self):
        self.rejected = {"sysctls1"}
        code, out = self.run_verify()
        self.assertEqual(code, 1)
        self.assertIn("FAIL: baseline v1.38 userns=false: pass/sysctls1 rejected by restrict-sysctls", out)

    def test_known_difference_passes_and_stale_entry_fails(self):
        self.rejected = {"sysctls1"}
        known = [{"level": level, "kind": "pass", "fixtures": ["sysctls1"], "reason": "test"}
                 for level in verify.LEVELS]
        code, out = self.run_verify(known)
        self.assertEqual(code, 0, out)
        self.assertEqual(out.count("KNOWN: "), 4)
        self.rejected = set()
        code, out = self.run_verify(known)
        self.assertEqual(code, 1)
        self.assertIn("STALE: knownDifferences[0]: baseline/pass/sysctls1 matched nothing", out)

    def test_missing_version_fails(self):
        args = mock.Mock(cli="c", helm="h", chart="chart", fixtures=str(self.fixtures), jobs=1,
                         config={"checks": {}, "versions": ["1.99"], "knownDifferences": []})
        out = io.StringIO()
        with redirect_stdout(out):
            self.assertEqual(verify.verify(args), 1)
        self.assertIn("ERROR: no fixture directory for v1.99", out.getvalue())


if __name__ == "__main__":
    unittest.main(verbosity=2)
