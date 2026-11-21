import importlib.util
from pathlib import Path
import unittest


SPEC = importlib.util.spec_from_file_location(
    "branch_advisory", Path(__file__).with_name("branch-advisory.py")
)
advisory = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(advisory)


class BranchAdvisoryTest(unittest.TestCase):
    def test_legacy_guidance_does_not_recommend_base_flip(self):
        result = advisory.advisory({"category": "LEGACY_ONLY", "base_ref": "main"})
        self.assertIn("preparing and reviewing a rebased branch", result)

    def test_cel_backports_are_allowed(self):
        result = advisory.advisory({"category": "CEL_ONLY", "base_ref": "release-1.19"})
        self.assertIn("CEL fixes are valid", result)

    def test_mixed_and_migration_are_manual(self):
        for category in ("MIXED", "REVIEW-MIGRATION"):
            self.assertIn(
                "maintainer", advisory.advisory({"category": category, "base_ref": "main"})
            )


if __name__ == "__main__":
    unittest.main()
