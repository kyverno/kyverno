import contextlib
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

import setup_benchmark as benchmark


class BenchmarkTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def write_arms(self):
        for arm in benchmark.ARMS:
            directory = self.root / ("setup-" + arm)
            directory.mkdir()
            for name, timestamp in [("start", 1), ("cleaned", 3), ("ready", 6)]:
                (directory / (name + ".json")).write_text(json.dumps({
                    "monotonic": timestamp, "checkout": "same-sha",
                    "free_bytes": 10 * 2**30,
                    "cpu_count": 4, "image_os": "ubuntu24",
                    "image_version": "20261012.1.0",
                }))
            (directory / "test.json").write_text(json.dumps({
                "seconds": 7, "exit_code": 0, "sampled_min_free_bytes": 2**30,
            }))
            (directory / "coverage.out").write_text("mode: atomic\na.go:1.1,2.2 3 1\n")

    def test_snapshot_has_disk_and_time(self):
        data = benchmark.snapshot()
        self.assertGreater(data["monotonic"], 0)
        self.assertGreater(data["free_bytes"], 0)
        self.assertGreater(data["temp_free_bytes"], 0)

    def test_marker_records_checkout(self):
        with mock.patch.object(benchmark.subprocess, "check_output", return_value="sha\n"):
            benchmark.mark(self.root, "start")
        self.assertEqual(json.loads((self.root / "start.json").read_text())["checkout"], "sha")

    def test_records_command_failure(self):
        result = benchmark.run(self.root, [sys.executable, "-c", "raise SystemExit(7)"])
        self.assertEqual(result, 7)
        data = json.loads((self.root / "test.json").read_text())
        self.assertEqual(data["exit_code"], 7)
        self.assertGreaterEqual(data["seconds"], 0)
        self.assertGreater(data["sampled_min_free_bytes"], 0)

    def test_records_command_success(self):
        self.assertEqual(benchmark.run(self.root, [sys.executable, "-c", "pass"]), 0)

    def test_comparison_preserves_all_arms(self):
        self.write_arms()
        result = benchmark.report(self.root)
        self.assertEqual([row["arm"] for row in result["arms"]], list(benchmark.ARMS))
        self.assertTrue(all(row["measured_seconds"] == 12 for row in result["arms"]))
        self.assertTrue(all(row["covered_statements"] == 3 for row in result["arms"]))

    def test_reports_coverage_variation_without_hiding_it(self):
        self.write_arms()
        (self.root / "setup-no-cleanup" / "coverage.out").write_text(
            "mode: atomic\na.go:1.1,2.2 3 0\n")
        result = benchmark.report(self.root)
        self.assertEqual(result["arms"][1]["covered_statements"], 0)

    def test_rejects_changed_coverage_inventory(self):
        self.write_arms()
        (self.root / "setup-no-cleanup" / "coverage.out").write_text(
            "mode: atomic\na.go:1.1,2.2 4 1\n")
        with self.assertRaises(ValueError):
            benchmark.report(self.root)

    def test_rejects_failed_test_arm(self):
        self.write_arms()
        path = self.root / "setup-minimal-setup" / "test.json"
        data = json.loads(path.read_text())
        data["exit_code"] = 1
        path.write_text(json.dumps(data))
        with self.assertRaises(ValueError):
            benchmark.report(self.root)

    def test_rejects_mismatched_checkout(self):
        self.write_arms()
        path = self.root / "setup-no-build-cache" / "ready.json"
        data = json.loads(path.read_text())
        data["checkout"] = "different-sha"
        path.write_text(json.dumps(data))
        with self.assertRaises(ValueError):
            benchmark.report(self.root)

    def test_rejects_missing_arm(self):
        self.write_arms()
        (self.root / "setup-no-cleanup" / "test.json").unlink()
        with self.assertRaises(FileNotFoundError):
            benchmark.report(self.root)

    def test_marks_different_runner_identity_non_comparable(self):
        self.write_arms()
        paths = [self.root / "setup-no-cleanup" / (name + ".json")
                 for name in ("start", "cleaned", "ready")]
        originals = [json.loads(path.read_text()) for path in paths]
        for key, value in [("cpu_count", 8), ("image_os", "ubuntu26"),
                           ("image_version", "different-image")]:
            with self.subTest(key=key, value=value):
                for path, original in zip(paths, originals):
                    data = dict(original)
                    data[key] = value
                    path.write_text(json.dumps(data))
                self.assertFalse(benchmark.report(self.root)["comparable"])

    def test_rejects_incomplete_or_unstable_runner_identity(self):
        self.write_arms()
        path = self.root / "setup-no-cleanup" / "start.json"
        original = json.loads(path.read_text())
        for key, value in [("cpu_count", 8), ("image_os", "ubuntu26"),
                           ("image_version", "different-image"), ("image_version", None)]:
            with self.subTest(key=key, value=value):
                data = dict(original)
                data[key] = value
                path.write_text(json.dumps(data))
                with self.assertRaises(ValueError):
                    benchmark.report(self.root)

    def test_rejects_bad_profiles(self):
        path = self.root / "coverage.out"
        for text in ["", "mode: set\n", "mode: atomic\n",
                     "mode: atomic\nbad\n",
                     "mode: atomic\na.go:1.1,2.2 3 1\na.go:1.1,2.2 3 2\n"]:
            with self.subTest(text=text):
                path.write_text(text)
                with self.assertRaises(ValueError):
                    benchmark.coverage(path)

    def test_report_cli_writes_json_and_markdown(self):
        self.write_arms()
        with mock.patch.object(sys, "argv", ["setup_benchmark.py", "report", str(self.root)]), \
                contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(benchmark.main(), 0)
        self.assertIn("baseline", output.getvalue())
        self.assertTrue((self.root / "comparison.json").is_file())
        self.assertTrue(json.loads((self.root / "comparison.json").read_text())["comparable"])

    def test_report_cli_warns_on_non_comparable_workers(self):
        self.write_arms()
        for name in ("start", "cleaned", "ready"):
            path = self.root / "setup-no-cleanup" / (name + ".json")
            data = json.loads(path.read_text())
            data["image_version"] = "different-image"
            path.write_text(json.dumps(data))
        with mock.patch.object(sys, "argv", ["setup_benchmark.py", "report", str(self.root)]), \
                contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(benchmark.main(), 0)
        self.assertIn("NON-COMPARABLE", output.getvalue())
        self.assertFalse(json.loads((self.root / "comparison.json").read_text())["comparable"])


if __name__ == "__main__":
    unittest.main()
