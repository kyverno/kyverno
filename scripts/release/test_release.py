import io
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
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
import build
import krew
import release
import retained
import report
import scaffolding
import tag


ROOT = Path(__file__).resolve().parents[2]
SHA = "a" * 40
MAPPING = {
    "lines": {"1.20": {"kyverno": "3.10", "kyverno-policies": "4.2"}},
    "dependencies": {
        f"{item['name']}-{item['version']}.tgz": hashlib.sha256(b"locked archive").hexdigest()
        for item in release.external_dependencies(release.read(ROOT / "charts/kyverno/Chart.lock"))
    },
}


class Versions(unittest.TestCase):
    def test_committed_dependency_pins_match_the_source_lock(self):
        pins = release.read(ROOT / "config/releases.yaml")["dependencies"]
        expected = {f"{item['name']}-{item['version']}.tgz"
                    for item in release.external_dependencies(release.read(ROOT / "charts/kyverno/Chart.lock"))}
        self.assertEqual(set(pins), expected)
        for digest in pins.values():
            self.assertRegex(digest, r"^[a-f0-9]{64}$")

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

    def test_sbom_publication_preserves_identical_attachment_without_rewriting(self):
        sbom = Path("candidate.cdx.json")
        sbom.write_bytes(b'{"bomFormat":"CycloneDX"}\n')
        subject = "ghcr.io/kyverno/kyverno@sha256:" + "a" * 64
        attachment = "ghcr.io/kyverno/sbom:sha256-" + "a" * 64 + ".sbom"
        env = {**os.environ, "COSIGN_REPOSITORY": "ghcr.io/kyverno/sbom"}
        with patch.object(
            release.subprocess, "check_output",
            side_effect=[attachment, sbom.read_bytes()],
        ) as check_output, patch.object(
            release, "inspect_digest", return_value="sha256:" + "b" * 64,
        ), patch.object(release, "run") as run:
            release.publish_sbom(subject, sbom, "kyverno")
        run.assert_not_called()
        self.assertEqual(check_output.call_args_list[0].args[0],
                         ["cosign", "triangulate", "--type", "sbom", subject])
        self.assertEqual(check_output.call_args_list[0].kwargs["env"], env)
        self.assertEqual(check_output.call_args_list[1].args[0],
                         ["cosign", "download", "sbom", subject])

    def test_sbom_publication_rejects_conflicting_attachment_without_rewriting(self):
        sbom = Path("candidate.cdx.json")
        sbom.write_bytes(b'{"bomFormat":"CycloneDX"}\n')
        subject = "ghcr.io/kyverno/kyverno@sha256:" + "a" * 64
        attachment = "ghcr.io/kyverno/sbom:sha256-" + "a" * 64 + ".sbom"
        with patch.object(
            release.subprocess, "check_output",
            side_effect=[attachment, b'{"bomFormat":"SPDX"}'],
        ), patch.object(
            release, "inspect_digest", return_value="sha256:" + "b" * 64,
        ), patch.object(release, "run") as run:
            with self.assertRaisesRegex(ValueError, "Refusing to overwrite conflicting"):
                release.publish_sbom(subject, sbom, "kyverno")
        run.assert_not_called()

    def test_sbom_publication_rejects_attachment_with_different_whitespace(self):
        sbom = Path("candidate.cdx.json")
        sbom.write_bytes(b'{"bomFormat":"CycloneDX"}\n')
        subject = "ghcr.io/kyverno/kyverno@sha256:" + "a" * 64
        attachment = "ghcr.io/kyverno/sbom:sha256-" + "a" * 64 + ".sbom"
        with patch.object(
            release.subprocess, "check_output",
            side_effect=[attachment, b'{"bomFormat":"CycloneDX"}'],
        ), patch.object(
            release, "inspect_digest", return_value="sha256:" + "b" * 64,
        ), patch.object(release, "run") as run:
            with self.assertRaisesRegex(ValueError, "Refusing to overwrite conflicting"):
                release.publish_sbom(subject, sbom, "kyverno")
        run.assert_not_called()

    def test_sbom_publication_attaches_only_when_absent_and_verifies_bytes(self):
        sbom = Path("candidate.cdx.json")
        sbom.write_bytes(b'{"bomFormat":"CycloneDX"}\n')
        subject = "ghcr.io/kyverno/kyverno@sha256:" + "a" * 64
        attachment = "ghcr.io/kyverno/sbom:sha256-" + "a" * 64 + ".sbom"
        env = {**os.environ, "COSIGN_REPOSITORY": "ghcr.io/kyverno/sbom"}
        with patch.object(
            release.subprocess, "check_output",
            side_effect=[attachment, sbom.read_bytes()],
        ) as check_output, patch.object(
            release, "inspect_digest", return_value=None,
        ), patch.object(release, "run") as run:
            release.publish_sbom(subject, sbom, "kyverno")
        run.assert_called_once_with(
            "cosign", "attach", "sbom", "--type", "cyclonedx", "--sbom", sbom, subject, env=env,
        )
        self.assertEqual(check_output.call_count, 2)

    def test_sbom_publication_rejects_different_bytes_written_during_attach(self):
        sbom = Path("candidate.cdx.json")
        sbom.write_bytes(b'{"bomFormat":"CycloneDX"}\n')
        subject = "ghcr.io/kyverno/kyverno@sha256:" + "a" * 64
        attachment = "ghcr.io/kyverno/sbom:sha256-" + "a" * 64 + ".sbom"
        with patch.object(
            release.subprocess, "check_output",
            side_effect=[attachment, b'{"bomFormat":"SPDX"}'],
        ), patch.object(
            release, "inspect_digest", return_value=None,
        ), patch.object(release, "run"):
            with self.assertRaisesRegex(ValueError, "Refusing to overwrite conflicting"):
                release.publish_sbom(subject, sbom, "kyverno")

    def test_remote_verification_rejects_sbom_with_missing_trailing_bytes(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        name = next(iter(release.IMAGES))
        image = "ghcr.io/kyverno/" + name + ":v1.20.2"
        metadata["images"] = {name: image}
        metadata["chart_images"] = {}
        sbom = Path(f".release/sboms/{name}-bom.cdx.json")
        sbom.parent.mkdir(parents=True)
        sbom.write_bytes(b'{"bomFormat":"CycloneDX"}\n')
        raw_image = b"image manifest"
        digest = "sha256:" + hashlib.sha256(raw_image).hexdigest()
        with patch.object(release, "verify_remote_charts"), \
                patch.object(release, "output", return_value="rendered manifest"), \
                patch.object(release, "verify_images"), \
                patch.object(release.subprocess, "check_output",
                             side_effect=[raw_image, sbom.read_bytes().rstrip()]), \
                patch.object(release, "inspect_digest", return_value=digest), \
                patch.object(release, "verify_signature"), \
                patch.object(release, "verify_provenance"):
            with self.assertRaisesRegex(ValueError, "Remote SBOM mismatch"):
                release.verify_remote(metadata)

    def stage_dependencies(self, *args, **kwargs):
        if args[:3] == ("helm", "dependency", "build"):
            for dependency in release.external_dependencies(release.read("charts/kyverno/Chart.lock")):
                (Path("charts/kyverno/charts") /
                 f"{dependency['name']}-{dependency['version']}.tgz").write_bytes(b"locked archive")

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
        with patch.object(release, "run", side_effect=self.stage_dependencies), \
                patch.object(release, "output", return_value=rendered):
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

    def test_preparation_rejects_changed_bytes_for_the_same_external_version(self):
        shutil.copytree(ROOT / "charts", "charts")
        metadata = release.plan("v1.20.2", SHA, MAPPING)

        def run(*args, **kwargs):
            self.stage_dependencies(*args, **kwargs)
            if args[:3] == ("helm", "dependency", "update"):
                next(Path("charts/kyverno/charts").glob("*.tgz")).write_bytes(b"replaced archive")

        with patch.object(release, "run", side_effect=run):
            with self.assertRaisesRegex(ValueError, "changed external dependency bytes"):
                release.prepare(metadata)

    def test_preparation_rejects_first_download_that_disagrees_with_committed_pins(self):
        shutil.copytree(ROOT / "charts", "charts")
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        metadata["locked_dependencies"] = dict.fromkeys(MAPPING["dependencies"], "0" * 64)
        with patch.object(release, "run", side_effect=self.stage_dependencies):
            with self.assertRaisesRegex(ValueError, "committed checksum pin"):
                release.prepare(metadata)

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

    def test_packaged_external_dependency_lock_archive_and_expanded_files_are_verified(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        dependency = {"name": "external", "version": "1.0.0", "repository": "https://example.test"}
        metadata.update(external_dependencies=[dependency], lock_digest="sha256:locked")
        header = {"apiVersion": "v2", "name": "external", "version": "1.0.0"}
        values = {"locked": "unchanged"}
        retained = Path(".release/dependencies/external-1.0.0.tgz")
        retained.parent.mkdir(parents=True)
        with tarfile.open(retained, "w:gz") as archive:
            for name, value in (("Chart.yaml", header), ("values.yaml", values)):
                data = yaml.safe_dump(value).encode()
                info = tarfile.TarInfo(f"external/{name}")
                info.size = len(data)
                archive.addfile(info, io.BytesIO(data))
        original = retained.read_bytes()
        metadata["dependency_checksums"] = {retained.name: release.sha256(retained)}
        metadata["locked_dependencies"] = dict(metadata["dependency_checksums"])

        def dependencies(name, files, mismatch=None):
            if name != "kyverno":
                return
            files["kyverno/Chart.yaml"]["dependencies"].append(dependency)
            files["kyverno/Chart.lock"] = {
                "dependencies": [dependency],
                "digest": "sha256:wrong" if mismatch == "lock" else metadata["lock_digest"],
            }
            files["kyverno/charts/external/Chart.yaml"] = header
            files["kyverno/charts/external/values.yaml"] = (
                {"locked": "changed"} if mismatch == "content" else values
            )

        release.verify_packages(metadata, self.package(metadata, dependencies))
        for mismatch in ("lock", "archive", "content"):
            retained.write_bytes(b"replaced" if mismatch == "archive" else original)
            with self.subTest(mismatch=mismatch), self.assertRaisesRegex(ValueError, "Changed .* dependency"):
                release.verify_packages(metadata, self.package(
                    metadata, lambda name, files: dependencies(name, files, mismatch)
                ))

    def index(self, metadata):
        directory = self.package(metadata)
        destination = Path(".release/packages")
        destination.mkdir(parents=True)
        index = {"entries": {}}
        for name, version in metadata["charts"].items():
            source = directory / f"{name}-{version}.tgz"
            shutil.copyfile(source, destination / source.name)
            with tarfile.open(source) as archive:
                entry = yaml.safe_load(archive.extractfile(f"{name}/Chart.yaml"))
            entry.update(digest=release.sha256(source), urls=[source.name], created="now")
            index["entries"][name] = [entry]
        return index

    def test_index_checks_client_urls_digest_and_exact_packaged_metadata(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        index = self.index(metadata)
        entry = index["entries"]["kyverno"][0]
        self.assertEqual(release.verify_index_entry("kyverno", entry, metadata),
                         "https://kyverno.github.io/kyverno/kyverno-3.10.2.tgz")
        for mutation in ({"urls": []}, {"urls": ["https://example.test/wrong.tgz"]},
                         {"urls": [None]}, {"digest": None},
                         {"appVersion": "v1.19.0"}, {"deprecated": True}):
            changed = {**entry, **mutation}
            with self.subTest(mutation=mutation), self.assertRaisesRegex(ValueError, "Conflicting Helm index"):
                release.verify_index_entry("kyverno", changed, metadata)

    def test_transient_oci_chart_read_failure_uses_bounded_retry(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        index = yaml.safe_dump(self.index(metadata)).encode()
        parent = Path(".release/packages/kyverno-3.10.2.tgz").read_bytes()
        policies = Path(".release/packages/kyverno-policies-4.2.2.tgz").read_bytes()
        responses = [io.BytesIO(data) for data in (index, parent, index, parent, policies)]
        calls = []

        def pull(*args):
            calls.append(args)
            if len(calls) == 1:
                raise subprocess.CalledProcessError(1, args)
            name = args[2].rsplit("/", 1)[-1]
            package = f"{name}-{metadata['charts'][name]}.tgz"
            shutil.copyfile(Path(".release/packages") / package, Path(args[-1]) / package)

        with patch.object(release.urllib.request, "urlopen", side_effect=responses), \
                patch.object(release, "run", side_effect=pull), \
                patch.object(release, "inspect_digest", return_value="sha256:remote"), \
                patch.object(release, "verify_signature"), patch.object(release.time, "sleep") as wait:
            release.verify_remote_charts(metadata)
            self.assertEqual(len(calls), 3)
            wait.assert_called_once_with(20)

    def test_remote_chart_retry_exhaustion_fails_closed(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        self.index(metadata)
        with patch.object(release.urllib.request, "urlopen",
                          side_effect=release.urllib.error.URLError("unavailable")) as fetch, \
                patch.object(release.time, "sleep"):
            with self.assertRaisesRegex(ValueError, "Remote chart verification failed"):
                release.verify_remote_charts(metadata)
            self.assertEqual(fetch.call_count, 12)

    def test_helm_test_does_not_resolve_dependencies_in_release_mode(self):
        helm = self.path / "helm"
        helm.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> helm-commands\n')
        helm.chmod(0o755)
        for mode in ("true", ""):
            commands = self.path / "helm-commands"
            commands.unlink(missing_ok=True)
            process = subprocess.run(
                ["make", "-f", str(ROOT / "Makefile"), "helm-test",
                 f"HELM={helm}", f"RELEASE_MODE={mode}"], capture_output=True, text=True, timeout=30,
            )
            self.assertEqual(process.returncode, 0, process.stderr)
            expected = ["test --namespace kyverno kyverno"]
            if not mode:
                expected.insert(0, "dependency build ./charts/kyverno")
            self.assertEqual(commands.read_text().splitlines(), expected)

    def test_make_image_overrides_match_the_native_aliases_retained_by_the_builder(self):
        metadata = release.plan("v1.20.2-rc.3", SHA, MAPPING)
        helm = self.path / "helm"
        helm.touch()
        process = subprocess.run(
            ["make", "-n", "-f", str(ROOT / "Makefile"), "kind-install-kyverno",
             f"HELM={helm}", "BUILD_WITH=ko", f"GIT_SHA={SHA}", "RELEASE_MODE=true"],
            capture_output=True, text=True, timeout=30,
        )
        self.assertEqual(process.returncode, 0, process.stderr)
        words = shlex.split(process.stdout)
        values = dict(word.split("=", 1) for word in words if "=" in word)
        native = build.native_images(metadata)
        references = {}
        for key, name in release.IMAGE_VALUES.items():
            if f"{key}.tag" in values:
                reference = (f"{values[key + '.registry']}/{values[key + '.repository']}:"
                             f"{values[key + '.tag']}")
                self.assertEqual(reference, native[name])
                references[key] = reference
        self.assertEqual(len(references), 8)
        self.assertEqual(set(references.values()), set(native.values()))

    def test_k6_policies_use_the_retained_package_without_repository_fallback(self):
        steps = release.read(ROOT / ".github/workflows/tests-k6.yaml")["jobs"]["k6"]["steps"]
        script = next(step["run"] for step in steps if step.get("name") == "Deploy policies")
        helm = self.path / "helm"
        helm.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> helm-commands\n')
        helm.chmod(0o755)
        package = Path(".release/packages/kyverno-policies-3.10.2-rc.3.tgz")
        package.parent.mkdir(parents=True)
        package.touch()
        for candidate in ("true", "false"):
            commands = self.path / "helm-commands"
            commands.unlink(missing_ok=True)
            process = subprocess.run(
                ["bash", "-e", "-c", script], capture_output=True, text=True, timeout=30,
                env={**os.environ, "PATH": f"{self.path}:{os.environ['PATH']}",
                     "RELEASE_CANDIDATE": candidate},
            )
            self.assertEqual(process.returncode, 0, process.stderr)
            command = commands.read_text()
            if candidate == "true":
                self.assertNotIn("--repo", command)
                self.assertIn(str(package), command)
            else:
                self.assertIn("--repo https://kyverno.github.io/kyverno/", command)
        package.unlink()
        with patch.dict(os.environ, PATH=f"{self.path}:{os.environ['PATH']}",
                        RELEASE_CANDIDATE="true"):
            process = subprocess.run(["bash", "-e", "-c", script], capture_output=True, timeout=30)
        self.assertNotEqual(process.returncode, 0)

    def test_workspace_restore_is_posix_and_uses_busybox_checksum_flags(self):
        restore = release.read(ROOT / ".github/actions/release/restore/action.yaml")["runs"]["steps"][-1]
        self.assertEqual(restore["shell"], "sh")
        checksum = self.path / "sha256sum"
        checksum.write_text('#!/bin/sh\n[ "$1" = "-c" ]\n')
        checksum.chmod(0o755)
        with tarfile.open("workspace.tar", "w") as archive:
            archive.addfile(tarfile.TarInfo(".release/packages/kyverno-3.10.2-rc.3.tgz"))
        Path("release-workspace.sha256").write_text(
            f"{release.sha256('workspace.tar')}  workspace.tar\n"
        )
        environment = self.path / "environment"
        process = subprocess.run(
            ["sh", "-eu", "-c", restore["run"]], capture_output=True, text=True, timeout=30,
            env={**os.environ, "PATH": f"{self.path}:{os.environ['PATH']}",
                 "GITHUB_ENV": str(environment)},
        )
        self.assertEqual(process.returncode, 0, process.stderr)
        self.assertIn("KYVERNO_TEST_CHART=.release/packages/kyverno-3.10.2-rc.3.tgz",
                      environment.read_text())

    def test_all_chainsaw_entrypoints_create_their_json_report_directory(self):
        generic = release.read(ROOT / ".github/actions/tests/conformance/run/action.yaml")
        conformance = release.read(ROOT / ".github/workflows/tests-conformance.yaml")["jobs"]
        entries = [
            (generic["runs"]["steps"], "conformance-results"),
            (conformance["custom-sigstore"]["steps"], "release-reports"),
            (conformance["check-tests"]["steps"], "release-reports"),
        ]
        chainsaw = self.path / "chainsaw"
        chainsaw.write_text(
            '#!/usr/bin/env python3\nimport sys\nfrom pathlib import Path\n'
            'directory=Path(sys.argv[sys.argv.index("--report-path")+1])\n'
            'if not directory.is_dir(): sys.exit(37)\n'
            '(directory/"results.json").write_text("{}")\n'
        )
        chainsaw.chmod(0o755)
        for path in ("test/conformance/chainsaw/reports-proof",
                     "test/conformance/chainsaw/custom-sigstore",
                     "test/conformance/chainsaw/cli", "policies"):
            Path(path).mkdir(parents=True)
        for steps, directory in entries:
            shutil.rmtree(directory, ignore_errors=True)
            script = next(step["run"] for step in steps if step.get("name") == "Test with Chainsaw")
            for key, value in {"tests-path": "reports-proof", "chainsaw-tests": "",
                               "shard-index": "0", "shard-count": "0"}.items():
                script = script.replace("${{ inputs." + key + " }}", value)
            script = script.replace("${{ matrix.shard-index }}", "0")
            process = subprocess.run(
                ["bash", "-e", "-c", script], capture_output=True, text=True, timeout=30,
                env={**os.environ, "PATH": f"{self.path}:{os.environ['PATH']}",
                     "GITHUB_WORKSPACE": str(self.path)},
            )
            self.assertEqual(process.returncode, 0, process.stderr)
            self.assertTrue((Path(directory) / "results.json").is_file())

    def test_pinned_sigstore_adapter_covers_the_actual_supported_node_matrix(self):
        jobs = release.read(ROOT / ".github/workflows/tests-conformance.yaml")["jobs"]
        self.assertEqual(set(scaffolding.NODE_IMAGES), set(jobs["deferred"]["strategy"]["matrix"]["k8s-version"]))
        source = (
            b'#!/bin/bash\nKIND_VERSION="v0.31.0"\ncase ${K8S_VERSION} in\n*) exit 9;;\nesac\n'
            b'printf "%s\\n%s\\n%s\\n%s\\n" "$K8S_VERSION" "$KIND_IMAGE" "$KNATIVE_VERSION" "$KIND_VERSION"\n'
        )
        with patch.object(scaffolding, "CHECKSUM", hashlib.sha256(source).hexdigest()):
            for version, digest in scaffolding.NODE_IMAGES.items():
                self.assertIn(version, jobs["custom-sigstore"]["strategy"]["matrix"]["k8s-version"])
                script = scaffolding.prepare_script(source, version)
                process = subprocess.run(
                    ["bash", "-e", "-c", script], capture_output=True, text=True, timeout=30,
                    env={**os.environ, "K8S_VERSION": version},
                )
                self.assertEqual(process.returncode, 0, process.stderr)
                self.assertEqual(process.stdout.splitlines(), [
                    version[1:], f"kindest/node:{version}@{digest}", "1.19.0", scaffolding.KIND_VERSION,
                ])
        setup = next(step for step in jobs["custom-sigstore"]["steps"]
                     if step.get("name") == "Install pinned Sigstore on the full-matrix cluster")
        routine = next(step for step in jobs["custom-sigstore"]["steps"]
                       if step.get("name") == "Create kind cluster and setup Sigstore Scaffolding")
        self.assertEqual(setup["uses"], routine["uses"])
        self.assertEqual(setup["with"]["version"], routine["with"]["version"])
        self.assertTrue(setup["with"]["sigstore-only"])
        kind = next(step for step in jobs["custom-sigstore"]["steps"]
                    if step.get("name") == "Install pinned full-matrix Kind")
        self.assertEqual(kind["with"]["version"], scaffolding.KIND_VERSION)

    def test_sigstore_adapter_rejects_source_drift_unknown_nodes_and_changed_structure(self):
        with self.assertRaisesRegex(ValueError, "committed checksum"):
            scaffolding.prepare_script(b"changed upstream source", "v1.36.1")
        with self.assertRaisesRegex(ValueError, "Missing pinned"):
            scaffolding.prepare_script(b"anything", "v1.37.0")
        source = b"no expected version map"
        with patch.object(scaffolding, "CHECKSUM", hashlib.sha256(source).hexdigest()):
            with self.assertRaisesRegex(ValueError, "Unexpected"):
                scaffolding.prepare_script(source, "v1.36.1")

    def stage_assets(self):
        environment = patch.dict(os.environ, GH_REPO="kyverno/kyverno")
        environment.start()
        self.addCleanup(environment.stop)
        metadata = release.plan("v1.20.2-rc.3", SHA, MAPPING)
        for name in (".release/assets/cli.tar.gz", ".release/install.yaml",
                     "config/crds/kyverno.io_policies.yaml", ".release/SHA256SUMS",
                     *[f".release/packages/{name}-{version}.tgz"
                       for name, version in metadata["charts"].items()],
                     *[f".release/sboms/{name}-bom.cdx.json" for name in release.IMAGES]):
            path = Path(name)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(f"retained {name}".encode())
        release.write_json(".release/plan.json", metadata)
        release.write_json(".release/manifests/index.json", {"manifests": [{
            "digest": "sha256:" + "c" * 64,
            "annotations": {"org.opencontainers.image.ref.name": metadata["tag"]},
        }]})
        Path(".release/manifest.tar.gz").write_bytes(b"retained manifest content")
        remote = {}
        for path in assets.all_assets():
            remote[path.name] = path.read_bytes()
            remote[f"{path.name}.sigstore.json"] = b"valid retained bundle"
        return metadata, remote

    def asset_commands(self, metadata, digest, content=b"retained manifest content"):
        def execute(*args, **kwargs):
            if args[:2] == ("cosign", "sign-blob"):
                Path(args[args.index("--bundle") + 1]).write_bytes(b"valid retained bundle")
            elif args[:2] == ("cosign", "verify-blob"):
                if Path(args[args.index("--bundle") + 1]).read_bytes() == b"invalid bundle":
                    raise subprocess.CalledProcessError(1, args)
            elif args[:2] == ("oras", "pull"):
                target = Path(args[args.index("--output") + 1])
                target.mkdir(parents=True)
                (target / "manifest.tar.gz").write_bytes(content)
            elif args[:2] == ("oras", "cp"):
                digest[0] = assets.manifest_digest(metadata)
            else:
                self.fail(f"Unexpected publication command (must not rebuild): {args}")
        return execute

    def test_remote_assets_verify_bytes_bundles_manifest_digest_and_content(self):
        metadata, remote = self.stage_assets()
        digest = [assets.manifest_digest(metadata)]
        with patch.object(assets, "asset_bytes", side_effect=lambda tag, name: remote.get(name)), \
                patch.object(assets, "remote_manifest", side_effect=lambda ref: digest[0]), \
                patch.object(assets, "verify_signature") as signature, \
                patch.object(assets, "run", side_effect=self.asset_commands(metadata, digest)) as commands:
            assets.verify(metadata)
        signature.assert_called_once_with(
            f"ghcr.io/kyverno/manifests/kyverno@{digest[0]}", metadata,
        )
        bundles = [call.args for call in commands.call_args_list
                   if call.args[:2] == ("cosign", "verify-blob")]
        self.assertEqual(len(bundles), len(assets.all_assets()))
        self.assertTrue(all(args[args.index("--certificate-github-workflow-sha") + 1] == SHA
                            for args in bundles))

    def test_remote_assets_fail_for_missing_conflicting_or_unverifiable_assets(self):
        metadata, original = self.stage_assets()
        first = assets.all_assets()[0].name
        for name, value, error in (
            (first, None, ValueError),
            (first, b"conflicting bytes", ValueError),
            (f"{first}.sigstore.json", None, ValueError),
            (f"{first}.sigstore.json", b"invalid bundle", subprocess.CalledProcessError),
        ):
            remote = {**original, name: value}
            with self.subTest(name=name, value=value), \
                    patch.object(assets, "asset_bytes", side_effect=lambda tag, name: remote.get(name)), \
                    patch.object(assets, "remote_manifest") as manifest, \
                    patch.object(assets, "run", side_effect=self.asset_commands(metadata, [])):
                with self.assertRaises(error):
                    assets.verify(metadata)
                manifest.assert_not_called()

    def test_final_asset_verification_requires_every_chart_and_sbom_copy(self):
        metadata, original = self.stage_assets()
        expected = [
            *[f"{name}-{version}.tgz" for name, version in metadata["charts"].items()],
            *[f"{name}-bom.cdx.json" for name in release.IMAGES],
        ]
        self.assertTrue(set(expected).issubset({path.name for path in assets.all_assets()}))
        for name in expected:
            for value in (None, b"conflicting remote copy"):
                remote = {**original, name: value}
                with self.subTest(name=name, value=value), \
                        patch.object(assets, "asset_bytes", side_effect=lambda tag, asset: remote.get(asset)), \
                        patch.object(assets, "run", side_effect=self.asset_commands(metadata, [])):
                    with self.assertRaisesRegex(ValueError, "Missing or conflicting remote asset"):
                        assets.verify(metadata)

    def test_remote_manifest_verification_rejects_digest_and_content_mismatches(self):
        metadata, remote = self.stage_assets()
        expected = assets.manifest_digest(metadata)
        for digest, content in (
            (None, b"retained manifest content"),
            ("sha256:" + "d" * 64, b"retained manifest content"),
            (expected, b"different manifest content"),
        ):
            with self.subTest(digest=digest, content=content), \
                    patch.object(assets, "asset_bytes", side_effect=lambda tag, name: remote.get(name)), \
                    patch.object(assets, "remote_manifest", return_value=digest), \
                    patch.object(assets, "verify_signature"), \
                    patch.object(assets, "run", side_effect=self.asset_commands(metadata, [], content)):
                with self.assertRaises(ValueError):
                    assets.verify(metadata)

    def test_interrupted_asset_publication_reuses_remote_bytes_bundles_and_oci_object(self):
        metadata, remote = self.stage_assets()
        remote.clear()
        digest = [None]
        uploaded = []
        interrupt = [True]

        def upload(tag, path):
            uploaded.append(path.name)
            if interrupt[0] and len(uploaded) == 3:
                raise ConnectionError("interrupted after retaining the first signed asset")
            data = path.read_bytes()
            existing = remote.get(path.name)
            release.same_or_absent(
                hashlib.sha256(existing).hexdigest() if existing is not None else None,
                hashlib.sha256(data).hexdigest(), path.name,
            )
            remote[path.name] = data

        with patch.object(assets, "upload_asset", side_effect=upload), \
                patch.object(assets, "asset_bytes", side_effect=lambda tag, name: remote.get(name)), \
                patch.object(assets, "remote_manifest", side_effect=lambda ref: digest[0]), \
                patch.object(assets, "sign") as sign, \
                patch.object(assets, "run", side_effect=self.asset_commands(metadata, digest)) as commands:
            with self.assertRaises(ConnectionError):
                assets.publish(metadata)
            retained = dict(remote)
            interrupt[0] = False
            assets.publish(metadata)
            self.assertTrue(all(remote[name] == data for name, data in retained.items()))
            count = len(commands.call_args_list)
            snapshot = dict(remote)
            assets.publish(metadata)
            self.assertEqual(remote, snapshot)
            self.assertTrue(all(call.args[:2] == ("cosign", "verify-blob")
                                for call in commands.call_args_list[count:]))
            self.assertEqual(sign.call_count, 2)

    def test_publication_rejects_existing_conflicting_asset_and_invalid_signature_bundle(self):
        metadata, remote = self.stage_assets()
        first = assets.all_assets()[0]

        def upload(tag, path):
            if remote.get(path.name) != path.read_bytes():
                raise ValueError("Refusing to overwrite conflicting published content")

        remote[first.name] = b"conflicting bytes"
        with patch.object(assets, "upload_asset", side_effect=upload), \
                patch.object(assets, "run") as commands:
            with self.assertRaisesRegex(ValueError, "conflicting"):
                assets.publish(metadata)
            commands.assert_not_called()
        remote[first.name] = first.read_bytes()
        remote[f"{first.name}.sigstore.json"] = b"invalid bundle"
        with patch.object(assets, "upload_asset", side_effect=upload), \
                patch.object(assets, "asset_bytes", side_effect=lambda tag, name: remote.get(name)), \
                patch.object(assets, "run", side_effect=self.asset_commands(metadata, [])) as commands:
            with self.assertRaises(subprocess.CalledProcessError):
                assets.publish(metadata)
            self.assertFalse(any(call.args[:2] == ("cosign", "sign-blob")
                                 for call in commands.call_args_list))
            self.assertEqual(remote[f"{first.name}.sigstore.json"], b"invalid bundle")
        remote[f"{first.name}.sigstore.json"] = b"valid retained bundle"
        with patch.object(assets, "upload_asset", side_effect=upload), \
                patch.object(assets, "asset_bytes", side_effect=lambda tag, name: remote.get(name)), \
                patch.object(assets, "remote_manifest", return_value="sha256:" + "e" * 64), \
                patch.object(assets, "sign") as sign, \
                patch.object(assets, "run", side_effect=self.asset_commands(metadata, [])) as commands:
            with self.assertRaisesRegex(ValueError, "conflicting"):
                assets.publish(metadata)
            sign.assert_not_called()
            self.assertFalse(any(call.args[:2] == ("oras", "cp") for call in commands.call_args_list))

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

    def test_first_failure_uses_completion_time_not_parallel_test_start_order(self):
        artifact = {"name": "release-conformance-parallel-v1.36.1-0-1", "id": 1}
        root = Path("release-report")
        directory = root / "test-reports" / artifact["name"]
        directory.mkdir(parents=True)
        tests = [
            {"name": name, "startTime": start, "steps": [{"operations": [
                {"name": "assert", "startTime": start, "endTime": end,
                 "failure": {"error": name}},
            ]}]}
            for name, start, end in (
                ("early-start-late-failure", "2026-01-01T00:00:00Z", "2026-01-01T00:03:00Z"),
                ("late-start-early-failure", "2026-01-01T00:01:00Z", "2026-01-01T00:02:00Z"),
            )
        ]
        release.write_json(directory / "results.json", {"tests": tests})
        with patch.object(artifacts, "available", return_value=[artifact]), \
                patch.object(artifacts, "download"), \
                patch.dict(os.environ, {"GITHUB_REPOSITORY": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}):
            summary = "\n".join(report.test_reports(root))
        self.assertIn("late-start-early-failure", summary)
        self.assertNotIn("early-start-late-failure", summary)


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

    def test_annotated_remote_tag_target_mismatch_is_rejected(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        with patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno"}), \
                patch.object(release, "gh", side_effect=[
                    json.dumps({"object": {"type": "tag", "sha": "c" * 40}}),
                    json.dumps({"object": {"type": "commit", "sha": "b" * 40}}),
                ]):
            with self.assertRaisesRegex(ValueError, "no longer points"):
                release.verify_tag(metadata)

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

    def test_job_report_retries_only_the_failed_small_page(self):
        first = {"total_count": 27, "jobs": [{"id": i} for i in range(25)]}
        second = {"total_count": 27, "jobs": [{"id": 25}, {"id": 26}]}
        with patch.object(report.subprocess, "check_output", side_effect=[
            json.dumps(first), subprocess.CalledProcessError(1, ["gh", "api"]),
            json.dumps(second),
        ]) as fetch, patch.object(report.time, "sleep") as sleep:
            self.assertEqual(report.job_pages("kyverno/kyverno", "1"), [first, second])
        urls = [call.args[0][-1] for call in fetch.call_args_list]
        self.assertTrue(all("filter=all&per_page=25" in url for url in urls))
        self.assertTrue(urls[0].endswith("page=1"))
        self.assertEqual(urls[1], urls[2])
        self.assertTrue(urls[1].endswith("page=2"))
        self.assertTrue(all(call.kwargs["timeout"] == 60 for call in fetch.call_args_list))
        sleep.assert_called_once_with(2)

    def test_job_report_fails_closed_on_exhausted_api_retries_or_incomplete_matrix(self):
        with patch.object(report.subprocess, "check_output",
                          side_effect=subprocess.CalledProcessError(1, ["gh", "api"])) as fetch, \
                patch.object(report.time, "sleep"):
            with self.assertRaises(subprocess.CalledProcessError):
                report.job_pages("kyverno/kyverno", "1")
            self.assertEqual(fetch.call_count, 3)
        with patch.object(report.subprocess, "check_output",
                          return_value=json.dumps({"total_count": 2, "jobs": []})):
            with self.assertRaisesRegex(ValueError, "Incomplete release job matrix"):
                report.job_pages("kyverno/kyverno", "1")

    def test_expired_retained_artifact_never_rebuilds(self):
        pages = [{"artifacts": [{"name": "release-candidate", "expired": True}]}]
        with patch.object(retained, "output", return_value=json.dumps(pages)), \
                patch.dict(os.environ, {"GITHUB_REPOSITORY": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}), \
                patch.object(retained.subprocess, "run") as mock:
            with self.assertRaisesRegex(ValueError, "expired"):
                retained.main()
            mock.assert_not_called()

    def test_orphaned_derived_artifacts_never_mix_with_a_rebuilt_candidate(self):
        for name in retained.NAMES[1:]:
            pages = [{"artifacts": [{"name": name, "expired": False}]}]
            with self.subTest(name=name), patch.object(retained, "output", return_value=json.dumps(pages)), \
                    patch.dict(os.environ, {"GITHUB_REPOSITORY": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}), \
                    patch.object(retained.subprocess, "run") as rebuild:
                with self.assertRaisesRegex(ValueError, "without their sealed candidate"):
                    retained.main()
                rebuild.assert_not_called()

    def test_missing_candidate_after_successful_retention_never_rebuilds(self):
        history = [{"jobs": [{"steps": [{
            "name": "Retain immutable release candidate", "conclusion": "success",
        }]}]}]
        with patch.object(retained, "output", side_effect=[json.dumps([{"artifacts": []}]), json.dumps(history)]), \
                patch.dict(os.environ, {"GITHUB_REPOSITORY": "kyverno/kyverno",
                                        "GITHUB_RUN_ID": "1", "GITHUB_RUN_ATTEMPT": "2"}), \
                patch.object(retained.subprocess, "run") as rebuild:
            with self.assertRaisesRegex(ValueError, "candidate disappeared"):
                retained.main()
            rebuild.assert_not_called()

    def test_new_dispatch_for_prior_tag_and_sha_never_rebuilds(self):
        prior_run = {
            "id": 1,
            "display_title": f"Prepare release v1.20.2 at {SHA}",
        }
        with patch.object(
            retained, "output",
            side_effect=[
                json.dumps([{"artifacts": []}]),
                json.dumps([{"workflow_runs": [prior_run]}]),
            ],
        ) as output, patch.dict(os.environ, {
            "GITHUB_REPOSITORY": "kyverno/kyverno",
            "GITHUB_RUN_ID": "2",
            "GITHUB_EVENT_NAME": "workflow_dispatch",
            "RELEASE_TAG": "v1.20.2",
            "RELEASE_SHA": SHA,
        }), patch.object(retained.subprocess, "run") as rebuild:
            with self.assertRaisesRegex(ValueError, "previous dispatch"):
                retained.main()
            self.assertIn("workflows/prepare-release.yaml/runs", output.call_args.args[-1])
            rebuild.assert_not_called()

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
                for optimized in ("", "1"):
                    process = subprocess.run(
                        ["bash", "-e", "-c", job["steps"][-1]["run"]],
                        env={**os.environ, "RESULTS": json.dumps(results),
                             "PYTHONOPTIMIZE": optimized, "PYTHONDONTWRITEBYTECODE": "1"},
                        capture_output=True,
                    )
                    self.assertEqual(process.returncode == 0, state == "success",
                                     (name, state, optimized))

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
        self.assertEqual(dispatch["jobs"]["validate"]["needs"], "identity")
        self.assertEqual(dispatch["run-name"],
                         "Prepare release ${{ inputs.tag }} at ${{ inputs.sha }}")
        self.assertIn('test "$RELEASE_SHA" = "$GITHUB_SHA"',
                      dispatch["jobs"]["identity"]["steps"][0]["run"])
        self.assertNotIn("workflow_dispatch", push["on"])
        self.assertEqual(list(self.workflow("helm-release.yaml")["on"]), ["workflow_call"])

    def test_every_validation_is_a_mandatory_gate(self):
        jobs = self.workflow("release-validate.yaml")["jobs"]
        expected = {"codegen", "unit", "candidate", "cli", "charts", "artifacthub",
                    "conformance", "k6", "performance", "manifests"}
        self.assertEqual(set(jobs["gate"]["needs"]), expected)
        self.assertNotIn("policy-library", jobs)
        last = jobs["gate"]["steps"][-1]
        self.assertIn('j["result"]=="success"', last["run"])
        for job in expected:
            self.assertNotIn("continue-on-error", jobs[job])
        self.assertIn("--charts charts/kyverno,charts/kyverno-policies",
                      jobs["charts"]["steps"][-1]["run"])
        self.assertIn("--skip-helm-dependencies", jobs["charts"]["steps"][-1]["run"])
        self.assertIn("helm lint --strict .release/packages/*.tgz", jobs["charts"]["steps"][-1]["run"])
        for job in ("conformance", "k6"):
            self.assertEqual(jobs[job]["with"]["release-candidate"], "true")
            self.assertEqual(jobs[job]["with"]["release-sha"], "${{ inputs.sha }}")
        scan = next(step for step in jobs["candidate"]["steps"]
                    if step.get("uses", "").startswith("aquasecurity/trivy-action@"))
        self.assertGreater(len(scan["env"]["TRIVY_DB_REPOSITORY"].split(",")), 1)

    def test_draft_until_publication_provenance_remote_verification_and_krew(self):
        jobs = self.workflow("release-publish.yaml")["jobs"]
        self.assertEqual(set(jobs["verify"]["needs"]), {"images", "provenance", "charts", "assets"})
        self.assertEqual(set(jobs["complete"]["needs"]), {"verify", "krew"})
        self.assertEqual(jobs["complete"]["concurrency"]["cancel-in-progress"], "false")
        self.assertNotIn("tag", jobs["complete"]["concurrency"]["group"])
        self.assertEqual(jobs["krew"]["needs"], "verify")
        submission = next(step for step in jobs["krew"]["steps"]
                          if step.get("uses", "").startswith("rajatjindal/krew-release-bot@"))
        self.assertEqual(submission["with"]["krew_template_file"], ".release-publication/krew.yaml")
        self.assertEqual(jobs["provenance"]["strategy"]["fail-fast"], "false")
        self.assertEqual(set(jobs["provenance"]["strategy"]["matrix"]["image"]), set(release.IMAGES))

    def test_slsa_verifier_uses_the_generator_identity_not_the_application_workflow_sha(self):
        workflow = self.workflow("release-publish.yaml")["jobs"]["provenance"]["uses"]
        repository, version = workflow.split("@")
        expected = f"https://github.com/{repository}@refs/tags/{version}"
        subject = "ghcr.io/kyverno/kyverno@sha256:" + "a" * 64
        with patch.object(release, "run") as run:
            release.verify_provenance(subject)
        args = run.call_args.args
        self.assertEqual(args[args.index("--certificate-identity") + 1], expected)
        self.assertNotIn("--certificate-github-workflow-sha", args)
        self.assertEqual(args[-1], subject)

    def test_signature_identity_accepts_only_source_bound_local_reusable_workflow_shas(self):
        metadata = release.plan("v1.20.2-rc.3", SHA, MAPPING)
        prefix = "https://github.com/kyverno/kyverno/.github/workflows/"
        with patch.dict(os.environ, GH_REPO="kyverno/kyverno"):
            identity = release.workflow_identity(metadata)
            blob = assets.blob_identity(metadata)
            for ref in (SHA, "refs/tags/v1.20.2-rc.3", "refs/heads/main", "refs/heads/release-1.20"):
                self.assertRegex(f"{prefix}release-publish.yaml@{ref}", identity)
                self.assertRegex(f"{prefix}release-publish.yaml@{ref}", blob)
            for url in (
                f"{prefix}release-publish.yaml@{'b' * 40}",
                f"{prefix}arbitrary.yaml@{SHA}",
                f"{prefix}release-publish.yaml@refs/heads/untrusted-feature",
                f"{prefix}release-publish.yaml@refs/heads/release-candidate",
                f"{prefix}release-publish.yaml@refs/heads/release-1x20",
                f"{prefix}release-publish.yaml@refs/heads/release-1.20-fix",
                f"https://github.com/other/repo/.github/workflows/release-publish.yaml@{SHA}",
            ):
                self.assertIsNone(re.fullmatch(identity, url))
                self.assertIsNone(re.fullmatch(blob, url))
            self.assertRegex(f"{prefix}helm-release.yaml@{SHA}", identity)
            self.assertIsNone(re.fullmatch(blob, f"{prefix}helm-release.yaml@{SHA}"))
            with patch.object(release, "run") as run:
                release.verify_signature("image@sha256:digest", metadata)
            args = run.call_args.args
            self.assertEqual(args[args.index("--certificate-github-workflow-sha") + 1], SHA)

    def test_release_checkout_code_is_explicitly_pinned_to_validated_sha(self):
        for name in ("prepare-release.yaml", "release-validate.yaml",
                     "release-publish.yaml", "helm-release.yaml"):
            for job in self.workflow(name)["jobs"].values():
                for step in job.get("steps", []):
                    if step.get("uses", "").startswith("actions/checkout@"):
                        self.assertEqual(step["with"]["ref"], "${{ inputs.sha }}", name)
                        self.assertEqual(step["with"]["persist-credentials"], "false", name)
        for filename, job in (("release-validate.yaml", "codegen"),
                              ("release-publish.yaml", "draft"),
                              ("helm-release.yaml", "publish")):
            self.assertEqual(self.workflow(filename)["jobs"][job]["steps"][0]["run"],
                             'test "$RELEASE_SHA" = "$GITHUB_SHA"')

    def test_special_conformance_suites_retain_per_test_reports(self):
        for name in ("custom-sigstore", "check-tests"):
            steps = self.workflow("tests-conformance.yaml")["jobs"][name]["steps"]
            self.assertTrue(any("--report-format JSON" in step.get("run", "") for step in steps))
            self.assertTrue(any(step.get("with", {}).get("name", "").startswith(
                "release-conformance-") for step in steps))

    def test_reusable_suites_pin_kyverno_code_and_keep_nonrelease_event_sha_default(self):
        for name in ("tests-conformance.yaml", "tests-k6.yaml"):
            workflow = self.workflow(name)
            self.assertEqual(workflow["on"]["workflow_call"]["inputs"]["release-sha"]["default"], "")
            for job in workflow["jobs"].values():
                for step in job.get("steps", []):
                    if step.get("uses", "").startswith("actions/checkout@") and \
                            step.get("with", {}).get("repository", "") != "kyverno/policies":
                        self.assertEqual(step["with"]["ref"], "${{ inputs.release-sha || github.sha }}", name)
    def test_prerelease_never_submits_krew(self):
        metadata = release.plan("v1.20.2-rc.3", SHA, MAPPING)
        with patch.object(krew, "check_seal"), patch.object(krew.Path, "read_text",
                return_value=json.dumps(metadata)), patch.object(krew, "existing") as mock, \
                patch.object(krew.sys, "argv", ["krew.py", "verify"]):
            krew.main()
            mock.assert_not_called()

    def test_finalize_prerelease_is_not_latest(self):
        metadata = release.plan("v1.20.2-rc3", SHA, MAPPING)
        responses = [{"isDraft": True}, {"isDraft": True}, {"isDraft": False, "isPrerelease": True}]
        with patch.object(assets, "existing_release", side_effect=responses), \
                patch.object(assets, "verify_tag"), \
                patch.object(assets.Path, "read_text", return_value=""), \
                patch.object(assets.Path, "mkdir"), patch.object(assets.Path, "write_text"), \
                patch.object(assets, "run") as mock, \
                patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}):
            assets.complete(metadata)
            self.assertIn("--latest=false", mock.call_args.args)
            self.assertIn("--prerelease=true", mock.call_args.args)

    def test_finalize_rechecks_draft_after_latest_selection_and_tag_verification(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        for current in (None, {"isDraft": False}):
            with self.subTest(current=current), \
                    patch.object(assets, "existing_release", side_effect=[{"isDraft": True}, current]), \
                    patch.object(assets, "verify_tag"), \
                    patch.object(assets, "should_be_latest", return_value=True), \
                    patch.object(assets.Path, "read_text", return_value=""), \
                    patch.object(assets.Path, "mkdir"), patch.object(assets.Path, "write_text"), \
                    patch.object(assets, "run") as mutate, \
                    patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}):
                with self.assertRaisesRegex(ValueError, "no longer a draft"):
                    assets.complete(metadata)
                mutate.assert_not_called()

    def test_finalize_rechecks_tag_after_reading_current_draft(self):
        metadata = release.plan("v1.20.2", SHA, MAPPING)
        with patch.object(assets, "existing_release", return_value={"isDraft": True}), \
                patch.object(assets, "verify_tag", side_effect=[None, ValueError("tag target changed")]), \
                patch.object(assets, "should_be_latest", return_value=True), \
                patch.object(assets.Path, "read_text", return_value=""), \
                patch.object(assets.Path, "mkdir"), patch.object(assets.Path, "write_text"), \
                patch.object(assets, "run") as mutate, \
                patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno", "GITHUB_RUN_ID": "1"}):
            with self.assertRaisesRegex(ValueError, "tag target changed"):
                assets.complete(metadata)
            mutate.assert_not_called()

    def test_latest_stable_uses_all_stable_tags_not_a_misclassified_rc_latest(self):
        pages = [[
            {"tag_name": "v1.19.2-rc.2", "draft": False, "prerelease": False},
            {"tag_name": "v1.20.0-rc.1", "draft": False, "prerelease": True},
            {"tag_name": "v1.21.0", "draft": True, "prerelease": False},
        ], [
            {"tag_name": "v1.19.1", "draft": False, "prerelease": False},
            {"tag_name": "v1.18.5", "draft": False, "prerelease": False},
        ]]
        with patch.dict(os.environ, {"GH_REPO": "kyverno/kyverno"}), \
                patch.object(assets.subprocess, "check_output", return_value=json.dumps(pages)):
            for tag_name, expected in (("v1.18.6", False), ("v1.19.1", True), ("v1.19.2", True)):
                self.assertEqual(assets.should_be_latest({
                    "tag": tag_name, "prerelease": False,
                }), expected, tag_name)


if __name__ == "__main__":
    unittest.main()
