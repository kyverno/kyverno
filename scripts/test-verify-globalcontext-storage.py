#!/usr/bin/env python3
"""Offline tests for the GlobalContextEntry storage report writer."""

import importlib.util
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
SPEC = importlib.util.spec_from_file_location(
    "verify_storage", Path(__file__).with_name("verify-globalcontext-storage.py")
)
storage = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(storage)


class ReportTest(unittest.TestCase):
    @unittest.skipUnless(os.name == "posix", "requires POSIX file permissions")
    def test_new_report_is_private_under_permissive_umask(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "report.json"
            previous = os.umask(0o022)
            try:
                storage.write_report(path, {"objects": {"cache": {"spec": "private"}}})
            finally:
                os.umask(previous)
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertEqual(json.loads(path.read_text(encoding="utf-8")),
                             {"objects": {"cache": {"spec": "private"}}})

    def test_existing_report_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "report.json"
            path.write_text("original", encoding="utf-8")
            path.chmod(0o644)
            with self.assertRaises(FileExistsError):
                storage.write_report(path, {"objects": "private"})
            self.assertEqual(path.read_text(encoding="utf-8"), "original")

    @unittest.skipUnless(os.name == "posix", "requires POSIX symlinks")
    def test_symlink_target_is_not_overwritten(self):
        with tempfile.TemporaryDirectory() as directory:
            target = Path(directory) / "target.json"
            target.write_text("original", encoding="utf-8")
            path = Path(directory) / "report.json"
            path.symlink_to(target)
            with self.assertRaises(FileExistsError):
                storage.write_report(path, {"objects": "private"})
            self.assertEqual(target.read_text(encoding="utf-8"), "original")


if __name__ == "__main__":
    unittest.main()
