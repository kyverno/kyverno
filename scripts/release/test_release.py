import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import unittest
from unittest.mock import patch
import uuid
import zipfile

import yaml

import assets
import artifacts
import krew
import release
import retained
import report
import tag


ROOT = Path(__file__).resolve().parents[2]
SHA = "a" * 40
MAPPING = {"lines": {"1.20": {"kyverno": "3.10", "kyverno-policies": "4.2"}}}


class Versions(unittest.TestCase):
    def test_stable_and_independent_minor_lines(self):
        result = release.plan("v1.20.2", SHA, MAPPING)
        self.assertEqual(result["charts"], {"kyverno": "3.10.2", "kyverno-policies": "4.2.2"})
        self.assertFalse(result["prerelease"])

    def test_both_rc_spellings_preserve_exact_image_and_app_tag(self):
        for suffix in ("-rc3", "-rc.3", "-alpha.1"):
            with self.subTest(suffix=suffix):
                result = release.plan(f"v1.20.2{suffix}", SHA, MAPPING)
                self.assertEqual(result["charts"]["kyverno"], f"3.10.2{suffix}")
                self.assertTrue(result["prerelease"])
                self.assertTrue(all(ref.endswith(f":v1.20.2{suffix}")
                                    for ref in result["images"].values()))

    def test_missing_mapping_fails(self):
        with self.assertRaisesRegex(ValueError, "No release mapping"):
            release.plan("v1.21.0", SHA, MAPPING)

    def test_reject_invalid_versions_and_shas(self):
        for value in ("1.20.1", "v01.20.1", "v1.20.01", "v1.20.0-rc.01", "v1.20.1+build",
                      "v1.20.1;echo bad", "v1.20.1/other", "v1.20"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                release.plan(value, SHA, MAPPING)
        with self.assertRaises(ValueError):
            release.plan("v1.20.1", "main", MAPPING)

    def test_safe_event_identity_and_optional_pretag_invocation(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        release.validate_event(metadata, "push", "refs/tags/v1.20.2", SHA)
        release.validate_event(metadata, "workflow_dispatch", "refs/heads/main", SHA)
        release.validate_event(metadata, "workflow_dispatch", "refs/heads/release-1.20", SHA)
        release.validate_event(metadata, "pull_request", "refs/pull/1/merge", SHA, True)
        for event, ref, sha, rehearsal in (
            ("workflow_dispatch", "refs/heads/feature", SHA, False),
            ("workflow_dispatch", "refs/heads/release-1.19", SHA, False),
            ("push", "refs/tags/v1.20.1", SHA, False),
            ("push", "refs/heads/main", SHA, False),
            ("pull_request_target", "refs/heads/main", SHA, True),
            ("workflow_dispatch", "refs/heads/main", "b" * 40, False),
        ):
            with self.subTest(event=event, ref=ref), self.assertRaises(ValueError):
                release.validate_event(metadata, event, ref, sha, rehearsal)

    def test_all_image_value_paths_exist_and_every_image_has_a_source(self):
        values = release.read(ROOT / "charts/kyverno/values.yaml")
        for keys, name in release.IMAGE_VALUES.items():
            value = values
            for key in keys.split("."):
                value = value[key]
            self.assertIn(name, release.IMAGES)
            self.assertIn("tag", value)
        self.assertEqual(set(release.IMAGE_VALUES.values()), set(release.IMAGES))
        configured = release.read(ROOT / ".ko.yaml")["builds"]
        self.assertEqual({item["main"].removeprefix("./") for item in configured},
                         set(release.IMAGES.values()))

    def test_rendered_background_and_other_controller_images_cannot_be_stale(self):
        metadata = release.plan("v1.20.2-rc3", SHA, MAPPING)
        rendered = yaml.safe_dump({
            "spec": {"containers": [{"image": image} for image in metadata["images"].values()]}
        })
        self.assertEqual(release.verify_images(rendered, metadata), set(release.IMAGES))
        for reference in metadata["images"].values():
            with self.subTest(image=reference), self.assertRaisesRegex(ValueError, "Stale image"):
                release.verify_images(rendered.replace(reference, reference.rsplit(":", 1)[0] + ":latest"), metadata)
        with self.assertRaisesRegex(ValueError, "Missing rendered"):
            release.verify_images("kind: ConfigMap", metadata)

    def test_preserved_chart_registry_mirrors_use_the_exact_application_tag(self):
        metadata = release.plan("v1.20.2-rc3", SHA, MAPPING)
        metadata["chart_images"] = release.chart_image_references(
            release.read(ROOT / "charts/kyverno/values.yaml"), metadata
        )
        self.assertEqual(metadata["chart_images"]["backgroundController.image"],
                         "reg.kyverno.io/kyverno/background-controller:v1.20.2-rc3")
        rendered = yaml.safe_dump({"containers": [
            {"image": reference} for reference in metadata["chart_images"].values()
        ]})
        self.assertEqual(release.verify_images(rendered, metadata), set(release.IMAGES))
        with self.assertRaisesRegex(ValueError, "Stale image"):
            release.verify_images(rendered.replace("v1.20.2-rc3", "latest"), metadata)


class LocalArtifacts(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.path = ROOT / f".release-test-{uuid.uuid4().hex}"
        self.path.mkdir()
        os.chdir(self.path)
        self.addCleanup(self.cleanup)

    def cleanup(self):
        os.chdir(self.previous)
        shutil.rmtree(self.path)

    def test_seal_rejects_changed_missing_and_additional_artifacts(self):
        root = Path("candidate")
        root.mkdir()
        path = root / "chart.tgz"
        path.write_bytes(b"immutable bytes")
        release.seal(root)
        release.check_seal(root)
        path.write_bytes(b"conflict")
        with self.assertRaisesRegex(ValueError, "Checksum mismatch"):
            release.check_seal(root)
        path.write_bytes(b"immutable bytes")
        (root / "extra").touch()
        with self.assertRaisesRegex(ValueError, "unsealed"):
            release.check_seal(root)
        (root / "extra").unlink()
        path.unlink()
        with self.assertRaises(FileNotFoundError):
            release.check_seal(root)

    def test_scalar_updates_preserve_comments(self):
        path = Path("values.yaml")
        path.write_text("# description\nimage:\n  # tag docs\n  tag: ~ # inline\n")
        release.set_scalar(path, "image.tag", "v1.20.1-rc3")
        self.assertIn("# tag docs", path.read_text())
        self.assertIn("# inline", path.read_text())
        self.assertEqual(release.read(path)["image"]["tag"], "v1.20.1-rc3")

    def test_preparation_preserves_each_kube_version_and_external_lock(self):
        shutil.copytree(ROOT / "charts", "charts")
        compatibility = {
            str(path): release.read(path).get("kubeVersion")
            for path in Path("charts").rglob("Chart.yaml")
        }
        metadata = release.plan("v1.20.2-rc.3", SHA, MAPPING)
        registries = {}
        values = release.read("charts/kyverno/values.yaml")
        for keys in release.IMAGE_VALUES:
            value = values
            for key in keys.split("."):
                value = value[key]
            registries[keys] = value["registry"]
        rendered = yaml.safe_dump({"spec": {"containers": [
            {"image": image} for image in metadata["images"].values()
        ]}})
        with patch.object(release, "run"), patch.object(release, "output", return_value=rendered):
            release.prepare(metadata)
        for path, expected in compatibility.items():
            self.assertEqual(release.read(path).get("kubeVersion"), expected)
        parent = release.read("charts/kyverno/Chart.yaml")
        self.assertEqual(parent["version"], "3.10.2-rc.3")
        self.assertEqual(parent["appVersion"], "v1.20.2-rc.3")
        self.assertEqual(parent["annotations"]["artifacthub.io/prerelease"], "true")
        values = release.read("charts/kyverno/values.yaml")
        for keys, registry in registries.items():
            value = values
            for key in keys.split("."):
                value = value[key]
            self.assertEqual(value["registry"], registry)
        for item in parent["dependencies"]:
            if not item.get("repository"):
                self.assertEqual(item["version"], parent["version"])
        self.assertEqual(release.external_dependencies(parent),
                         release.external_dependencies(release.read("charts/kyverno/Chart.lock")))

    def package(self, metadata, stale=None):
        directory = Path("packages")
        directory.mkdir(exist_ok=True)
        for name, version in metadata["charts"].items():
            content = {"apiVersion": "v2", "name": name, "version": version,
                       "appVersion": metadata["tag"]}
            files = {f"{name}/Chart.yaml": content}
            if name == "kyverno":
                values = {}
                for keys, image in release.IMAGE_VALUES.items():
                    parent = values
                    for key in keys.split("."):
                        parent = parent.setdefault(key, {})
                    parent.update(registry="ghcr.io", repository=f"kyverno/{image}", tag=metadata["tag"])
                content["dependencies"] = [
                    {"name": subchart, "version": version} for subchart in ("crds", "grafana")
                ]
                files[f"{name}/values.yaml"] = values
                for subchart in ("crds", "grafana"):
                    files[f"{name}/charts/{subchart}/Chart.yaml"] = {
                        "version": version, "name": subchart,
                    }
            if stale:
                stale(name, files)
            with tarfile.open(directory / f"{name}-{version}.tgz", "w:gz") as archive:
                for filename, value in files.items():
                    data = yaml.safe_dump(value).encode()
                    info = tarfile.TarInfo(filename)
                    info.size = len(data)
                    archive.addfile(info, io.BytesIO(data))
        return directory

    def test_packaged_metadata_not_just_environment_is_verified(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        release.verify_packages(metadata, self.package(metadata))

        def stale(name, files):
            files[f"{name}/Chart.yaml"]["appVersion"] = "v1.19.1"

        with self.assertRaisesRegex(ValueError, "Stale chart metadata"):
            release.verify_packages(metadata, self.package(metadata, stale))

    def test_packaged_background_hook_and_dependency_references(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        for key in release.IMAGE_VALUES:
            def stale(name, files):
                if name == "kyverno":
                    value = files["kyverno/values.yaml"]
                    for part in key.split("."):
                        value = value[part]
                    value["tag"] = "latest"

            with self.subTest(key=key), self.assertRaisesRegex(ValueError, "Stale image"):
                release.verify_packages(metadata, self.package(metadata, stale))

        def stale_dependency(name, files):
            if name == "kyverno":
                files["kyverno/Chart.yaml"]["dependencies"][0]["version"] = "0.0.0"

        with self.assertRaisesRegex(ValueError, "Stale dependency"):
            release.verify_packages(metadata, self.package(metadata, stale_dependency))

    def test_krew_precomputed_hashes_do_not_fetch_draft_assets(self):
        shutil.copyfile(ROOT / ".krew.yaml", ".krew.yaml")
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        directory = Path(".release/assets")
        directory.mkdir(parents=True)
        for os_name, arch, suffix in (("linux", "x86_64", "tar.gz"), ("darwin", "x86_64", "tar.gz"),
                                      ("darwin", "arm64", "tar.gz"), ("windows", "x86_64", "zip")):
            (directory / f"kyverno-cli_v1.20.2_{os_name}_{arch}.{suffix}").write_bytes(
                f"retained {os_name}/{arch} archive".encode()
            )
        expected = krew.manifest(metadata)
        self.assertEqual(expected["spec"]["version"], metadata["tag"])
        for platform in expected["spec"]["platforms"]:
            archive = directory / platform["uri"].rsplit("/", 1)[1]
            self.assertEqual(platform["sha256"], release.sha256(archive))

    def test_diagnostic_download_refuses_path_traversal(self):
        stream = io.BytesIO()
        with zipfile.ZipFile(stream, "w") as archive:
            archive.writestr("../outside", "not allowed")
        with patch.object(artifacts.subprocess, "check_output", return_value=stream.getvalue()):
            with self.assertRaisesRegex(ValueError, "Invalid diagnostic"):
                artifacts.download({"archive_download_url": "https://api.github.com/artifact"}, "reports")
        self.assertFalse(Path("outside").exists())

    def test_aggregate_per_test_results_and_first_failed_operation(self):
        artifact = {"name": "release-conformance-assert-v1.36.1-0-1", "id": 1}
        root = Path("release-report")
        directory = root / "test-reports" / artifact["name"]
        directory.mkdir(parents=True)
        release.write_json(directory / "results.json", {"tests": [
            {"name": "failed-case", "basePath": "test/fixture", "startTime": "2026-01-01",
             "steps": [{"name": "assert", "operations": [
                 {"name": "check", "failure": {"error": "denied"}, "startTime": "2026-01-01"}
             ]}]},
            {"name": "passing-case", "steps": []},
            {"name": "skipped-case", "skipped": True},
        ]})
        with patch.object(artifacts, "available", return_value=[artifact]), \
                patch.object(artifacts, "download"), \
                patch.dict(os.environ, {"GITHUB_REPOSITORY": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}):
            lines = report.test_reports(root)
        results = json.loads((root / "per-test.json").read_text())
        self.assertEqual([row["result"] for row in results], ["failure", "success", "skipped"])
        self.assertIn("failed-case", "\n".join(lines))
        self.assertIn("denied", "\n".join(lines))


class RetryAndGates(unittest.TestCase):
    def test_k6_retry_restores_latest_each_scenario_not_lexical_or_other_prefix(self):
        values = [
            {"id": 1, "name": "k6-reports-simple-9"},
            {"id": 2, "name": "k6-reports-kyverno-1"},
            {"id": 3, "name": "k6-reports-kyverno-pss-1"},
            {"id": 4, "name": "k6-reports-simple-10"},
        ]
        with patch.object(artifacts, "available", return_value=values), \
                patch.object(artifacts, "download") as download:
            artifacts.restore_k6()
        self.assertEqual([call.args[0]["id"] for call in download.call_args_list], [4, 2, 3])

    def test_partial_retry_accepts_identical_bytes_but_never_conflicts(self):
        self.assertFalse(release.same_or_absent(None, "new", "chart"))
        self.assertTrue(release.same_or_absent("same", "same", "chart"))
        with self.assertRaisesRegex(ValueError, "Refusing to overwrite"):
            release.same_or_absent("old", "new", "chart")

    def test_remote_tag_must_still_target_the_validated_commit(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        with patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno"}), \
                patch.object(release, "gh", return_value=json.dumps({
                    "object": {"type": "commit", "sha": "b" * 40},
                })):
            with self.assertRaisesRegex(ValueError, "no longer points"):
                release.verify_tag(metadata)

    def test_annotated_remote_tag_is_peeled_to_the_validated_commit(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        with patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno"}), \
                patch.object(release, "gh", side_effect=[
                    json.dumps({"object": {"type": "tag", "sha": "c" * 40}}),
                    json.dumps({"object": {"type": "commit", "sha": SHA}}),
                ]) as read:
            release.verify_tag(metadata)
            self.assertIn("git/tags/", read.call_args.args[-1])

    def test_public_release_cannot_reenter_mutating_publication(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        with patch.object(release, "existing_release", return_value={
            "isDraft": False, "isPrerelease": False,
        }), patch.object(release, "run") as mutate:
            with self.assertRaisesRegex(ValueError, "release is public"):
                release.ensure_draft(metadata)
            with self.assertRaisesRegex(ValueError, "not a draft"):
                release.require_draft(metadata)
            mutate.assert_not_called()

    def test_retry_report_keeps_prior_success_and_latest_retried_outcome(self):
        pages = [{"jobs": [
            {"id": 1, "name": "unit", "conclusion": "success"},
            {"id": 2, "name": "charts", "conclusion": "failure"},
        ]}, {"jobs": [
            {"id": 3, "name": "charts", "conclusion": "success"},
        ]}]
        self.assertEqual(report.latest_jobs(pages), [pages[0]["jobs"][0], pages[1]["jobs"][0]])

    def test_expired_retained_artifact_never_rebuilds(self):
        pages = [{"artifacts": [{"name": "release-candidate", "expired": True}]}]
        with patch.object(retained, "output", return_value=json.dumps(pages)), \
                patch.dict(os.environ, {"GITHUB_REPOSITORY": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}), \
                patch.object(retained.subprocess, "run") as mock:
            with self.assertRaisesRegex(ValueError, "expired"):
                retained.main()
            mock.assert_not_called()

    def test_existing_asset_compares_api_download_not_graphql_id_and_never_clobbers(self):
        data = b"immutable archive"
        asset = {"name": "test.tar.gz", "id": "RA_graphql_id",
                 "apiUrl": "https://api.github.com/repos/kyverno/kyverno/releases/assets/123"}
        with patch.object(release, "existing_release", return_value={"assets": [asset]}), \
                patch.object(release.Path, "open", return_value=io.BytesIO(data)), \
                patch.object(release.subprocess, "check_output", return_value=data) as download, \
                patch.object(release, "run") as upload:
            release.upload_asset("v1.20.2", "test.tar.gz")
            self.assertEqual(download.call_args.args[0][-1], asset["apiUrl"])
            upload.assert_not_called()

    def test_all_failure_states_block_the_validation_gate(self):
        for name in ("release-validate.yaml", "release-publish.yaml"):
            job = self.workflow(name)["jobs"]["gate" if name == "release-validate.yaml" else "report"]
            for state in ("success", "failure", "cancelled", "skipped"):
                results = {dependency: {"result": "success"} for dependency in job["needs"]}
                results[job["needs"][0]]["result"] = state
                process = subprocess.run(
                    ["bash", "-e", "-c", job["steps"][-1]["run"]],
                    env={**os.environ, "RESULTS": json.dumps(results)}, capture_output=True,
                )
                self.assertEqual(process.returncode == 0, state == "success", (name, state))

    def test_tag_retry_never_moves_existing_tag(self):
        env = {"RELEASE_TAG": "v1.20.1", "RELEASE_SHA": SHA,
               "GITHUB_SHA": SHA, "GH_REPO": "kyverno/kyverno"}
        result = type("Result", (), {"returncode": 0, "stdout": json.dumps({
            "object": {"type": "commit", "sha": SHA}
        })})()
        with patch.dict(os.environ, env), patch.object(tag.subprocess, "run", return_value=result) as mock:
            tag.main()
            self.assertEqual(mock.call_count, 1)
        result.stdout = json.dumps({"object": {"type": "commit", "sha": "b" * 40}})
        with patch.dict(os.environ, env), patch.object(tag.subprocess, "run", return_value=result):
            with self.assertRaisesRegex(ValueError, "never move"):
                tag.main()

    def test_dispatch_ref_must_match_exact_sha(self):
        with patch.dict(os.environ, {"RELEASE_TAG": "v1.20.1", "RELEASE_SHA": SHA,
                                     "GITHUB_SHA": "b" * 40}):
            with self.assertRaisesRegex(ValueError, "Dispatch SHA changed"):
                tag.main()

    def workflow(self, name):
        # BaseLoader keeps the YAML 1.2 workflow key "on" out of YAML 1.1's bool conversion.
        return yaml.load((ROOT / ".github/workflows" / name).read_text(), Loader=yaml.BaseLoader)

    def test_tag_and_pretag_use_identical_validation_and_direct_publication(self):
        push = self.workflow("release.yaml")
        dispatch = self.workflow("prepare-release.yaml")
        for workflow in (push, dispatch):
            self.assertEqual(workflow["jobs"]["validate"]["uses"], "./.github/workflows/release-validate.yaml")
            self.assertEqual(workflow["jobs"]["publish"]["uses"], "./.github/workflows/release-publish.yaml")
            self.assertEqual(workflow["concurrency"]["cancel-in-progress"], "false")
            self.assertIn("validate", workflow["jobs"]["publish"]["needs"])
        self.assertIn("validate", dispatch["jobs"]["tag"]["needs"])
        self.assertNotIn("workflow_dispatch", push["on"])
        self.assertEqual(list(self.workflow("helm-release.yaml")["on"]), ["workflow_call"])

    def test_every_validation_is_a_mandatory_gate(self):
        jobs = self.workflow("release-validate.yaml")["jobs"]
        expected = {"codegen", "unit", "candidate", "cli", "charts", "artifacthub",
                    "conformance", "policy-library", "k6", "performance", "manifests"}
        self.assertEqual(set(jobs["gate"]["needs"]), expected)
        last = jobs["gate"]["steps"][-1]
        self.assertIn('j["result"]=="success"', last["run"])
        for job in expected:
            self.assertNotIn("continue-on-error", jobs[job])
        self.assertIn("--charts charts/kyverno,charts/kyverno-policies",
                      jobs["charts"]["steps"][-1]["run"])
        for job in ("conformance", "policy-library", "k6"):
            self.assertEqual(jobs[job]["with"]["release-candidate"], "true")
        scan = next(step for step in jobs["candidate"]["steps"]
                    if step.get("uses", "").startswith("aquasecurity/trivy-action@"))
        self.assertGreater(len(scan["env"]["TRIVY_DB_REPOSITORY"].split(",")), 1)

    def test_draft_until_publication_provenance_remote_verification_and_krew(self):
        jobs = self.workflow("release-publish.yaml")["jobs"]
        self.assertEqual(set(jobs["verify"]["needs"]), {"images", "provenance", "charts", "assets"})
        self.assertEqual(set(jobs["complete"]["needs"]), {"verify", "krew"})
        self.assertEqual(jobs["krew"]["needs"], "verify")
        self.assertEqual(jobs["provenance"]["strategy"]["fail-fast"], "false")
        self.assertEqual(set(jobs["provenance"]["strategy"]["matrix"]["image"]), set(release.IMAGES))

    def test_release_checkout_code_is_explicitly_pinned_to_validated_sha(self):
        for name in ("prepare-release.yaml", "release-validate.yaml",
                     "release-publish.yaml", "helm-release.yaml"):
            for job in self.workflow(name)["jobs"].values():
                for step in job.get("steps", []):
                    if step.get("uses", "").startswith("actions/checkout@"):
                        self.assertEqual(step["with"]["ref"], "${{ inputs.sha }}", name)
                        self.assertEqual(step["with"]["persist-credentials"], "false", name)

    def test_special_conformance_suites_retain_per_test_reports(self):
        for name in ("custom-sigstore", "check-tests"):
            steps = self.workflow("tests-conformance.yaml")["jobs"][name]["steps"]
            self.assertTrue(any("--report-format JSON" in step.get("run", "") for step in steps))
            self.assertTrue(any(step.get("with", {}).get("name", "").startswith(
                "release-conformance-") for step in steps))

    def test_prerelease_never_submits_krew(self):
        metadata = release.plan("v1.20.2-rc.3", SHA, MAPPING)
        with patch.object(krew, "check_seal"), patch.object(krew.Path, "read_text",
                return_value=json.dumps(metadata)), patch.object(krew, "existing") as mock, \
                patch.object(krew.sys, "argv", ["krew.py", "verify"]):
            krew.main()
            mock.assert_not_called()

    def test_finalize_prerelease_is_not_latest(self):
        metadata = release.plan("v1.20.2-rc3", SHA, MAPPING)
        responses = [{"isDraft": True}, {"isDraft": False, "isPrerelease": True}]
        with patch.object(assets, "existing_release", side_effect=responses), \
                patch.object(assets, "verify_tag"), \
                patch.object(assets.Path, "read_text", return_value=""), \
                patch.object(assets.Path, "mkdir"), patch.object(assets.Path, "write_text"), \
                patch.object(assets, "run") as mock, \
                patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}):
            assets.complete(metadata)
            self.assertIn("--latest=false", mock.call_args.args)
            self.assertIn("--prerelease=true", mock.call_args.args)


if __name__ == "__main__":
    unittest.main()
