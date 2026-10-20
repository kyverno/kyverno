import contextlib
import copy
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

import unit_test_shards as shards


class PartitionTests(unittest.TestCase):
    def test_balances_heavy_packages_and_includes_new_packages(self):
        packages = ["heavy", "medium", "new", "light"]
        result = shards.partition(packages, {"heavy": 10, "medium": 8}, 2)
        self.assertEqual(sorted(p for group in result for p in group), sorted(packages))
        self.assertFalse(any({"heavy", "medium"} <= set(group) for group in result))
        self.assertEqual(result, shards.partition(list(reversed(packages)),
                                                {"heavy": 10, "medium": 8}, 2))

    def test_rejects_invalid_inputs(self):
        for packages, weights, count in [
            ([], {}, 1), (["a", "a"], {}, 1), (["a"], {}, 0),
            (["a"], {}, 2), (["a"], {"a": -1}, 1),
            (["a"], {"a": float("nan")}, 1),
            (["a"], {"a": float("inf")}, 1),
        ]:
            with self.subTest(packages=packages, weights=weights, count=count):
                with self.assertRaises(ValueError):
                    shards.partition(packages, weights, count)


class MergeTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.output = self.root / "coverage.out"
        self.manifests = [
            {"index": index, "count": 2, "inventory": ["a", "b"],
             "packages": [package], "success": True}
            for index, package in enumerate(["a", "b"])
        ]
        self.write_artifacts(self.manifests)

    def write_artifacts(self, manifests):
        for index, manifest in enumerate(manifests):
            directory = self.root / ("unit-shard-" + str(index))
            directory.mkdir(exist_ok=True)
            (directory / "manifest.json").write_text(json.dumps(manifest))
            (directory / "coverage.out").write_text(
                "mode: atomic\n{}.go:1.1,2.2 1 1\n".format(index))

    def test_merges_one_header_and_all_blocks(self):
        shards.merge(self.root, 2, self.output)
        self.assertEqual(self.output.read_text(),
                         "mode: atomic\n0.go:1.1,2.2 1 1\n1.go:1.1,2.2 1 1\n")

    def test_rejects_invalid_manifests(self):
        mutations = [
            lambda m: m[0].update(success=False),
            lambda m: m[0].update(index=1),
            lambda m: m[0].update(count=3),
            lambda m: m[0].update(inventory=["a", "b", "c"]),
            lambda m: m[0].update(packages=[]),
            lambda m: m[0].update(packages=["b"]),
            lambda m: m[0].update(packages=["a", "a"]),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                manifests = copy.deepcopy(self.manifests)
                mutation(manifests)
                self.write_artifacts(manifests)
                with self.assertRaises(ValueError):
                    shards.merge(self.root, 2, self.output)

    def test_rejects_missing_or_extra_artifacts(self):
        for count in [0, 1, 3]:
            with self.subTest(count=count):
                with self.assertRaises(ValueError):
                    shards.merge(self.root, count, self.output)

    def test_rejects_bad_profiles_and_overlaps(self):
        for profile in [
            "", "mode: set\n1.go:1.1,2.2 1 1\n",
            "mode: atomic\nmode: atomic\n",
            "mode: atomic\n1.go:1.1,2.2 1 -1\n",
            "mode: atomic\n0.go:1.1,2.2 1 1\n",
        ]:
            with self.subTest(profile=profile):
                self.write_artifacts(self.manifests)
                (self.root / "unit-shard-1" / "coverage.out").write_text(profile)
                with self.assertRaises(ValueError):
                    shards.merge(self.root, 2, self.output)

    def test_rejects_missing_profile(self):
        (self.root / "unit-shard-1" / "coverage.out").unlink()
        with self.assertRaises(FileNotFoundError):
            shards.merge(self.root, 2, self.output)


class ExecutionTests(unittest.TestCase):
    def test_missing_test_events_fail_even_with_zero_exit(self):
        with tempfile.TemporaryDirectory() as temporary:
            output = Path(temporary) / "shard"
            process = mock.MagicMock()
            process.stdout = iter([])
            process.wait.return_value = 0
            process.__enter__.return_value = process
            with mock.patch.object(subprocess, "check_output", return_value="a\n"), \
                    mock.patch.object(subprocess, "Popen", return_value=process):
                with self.assertRaises(ValueError):
                    shards.run_shard(0, 1, output, {})
            self.assertFalse(json.loads((output / "manifest.json").read_text())["success"])

    def test_real_go_shards_match_unsharded_coverage_and_propagate_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "go.mod").write_text("module example.com/sharding\n\ngo 1.20\n")
            for name in ["a", "b"]:
                directory = root / name
                directory.mkdir()
                (directory / "value.go").write_text(
                    "package " + name + "\nfunc Value() int { return 1 }\n")
                (directory / "value_test.go").write_text(
                    "package " + name + '\nimport "testing"\n'
                    "func TestValue(t *testing.T) { if Value() != 1 { t.Fatal(\"value\") } }\n")
            empty = root / "no_tests"
            empty.mkdir()
            (empty / "value.go").write_text("package no_tests\nfunc Value() int { return 2 }\n")
            old_directory = Path.cwd()
            try:
                os.chdir(root)
                with mock.patch.dict(os.environ, {
                    "GOWORK": "off", "GOFLAGS": "", "CGO_ENABLED": "1",
                }), contextlib.redirect_stdout(io.StringIO()):
                    for index in range(2):
                        shards.run_shard(index, 2, root / ("unit-shard-" + str(index)), {})
                    shards.merge(root, 2, root / "merged.out")
                    subprocess.run([
                        "go", "test", "-race", "-covermode=atomic",
                        "-coverprofile=baseline.out", "./...",
                    ], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                    baseline = (root / "baseline.out").read_text().splitlines()
                    merged = (root / "merged.out").read_text().splitlines()
                    self.assertEqual(sorted(baseline), sorted(merged))
                    subprocess.run(["go", "tool", "cover", "-func=merged.out"],
                                   check=True, stdout=subprocess.PIPE)
                    (root / "a" / "value_test.go").write_text(
                        'package a\nimport "testing"\n'
                        'func TestFailure(t *testing.T) { t.Fatal("expected") }\n')
                    with self.assertRaises(subprocess.CalledProcessError):
                        shards.run_shard(0, 1, root / "failed", {})
                    self.assertFalse(json.loads(
                        (root / "failed" / "manifest.json").read_text())["success"])
            finally:
                os.chdir(old_directory)


if __name__ == "__main__":
    unittest.main()
