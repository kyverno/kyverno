import importlib.util
import json
from pathlib import Path
import unittest


SPEC = importlib.util.spec_from_file_location(
    "prepare_retarget", Path(__file__).with_name("prepare-retarget.py")
)
prepare = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(prepare)


class ManifestTest(unittest.TestCase):
    def row(self, **changes):
        return {
            "probe": "OK", "draft": False, "title": "fix: legacy bug",
            "shared_files": [], "overlap_prs": [], "category": "LEGACY_ONLY",
            **changes,
        }

    def test_recommendation_is_not_approval(self):
        self.assertEqual(prepare.recommend(self.row())[0], "RETARGET")

    def test_unsafe_or_unreviewed_candidates_are_held(self):
        for changes in (
            {"draft": True}, {"title": "Feat/background metrics"},
            {"shared_files": ["pkg/config/config.go"]},
            {"overlap_prs": [{"number": 123}]}, {"probe": "CONFLICT"},
            {"overlap_candidates": [{"number": 456}]},
            {"probe": "ERROR:stale"}, {"category": "MIXED"},
            {"labels": ["type_shared"]},
        ):
            with self.subTest(changes=changes):
                self.assertEqual(prepare.recommend(self.row(**changes))[0], "NEEDS_AUTHOR")

    def test_explicit_scope_overrides_are_preserved(self):
        for override in ("KEEP_MAIN", "SPLIT"):
            with self.subTest(override=override):
                self.assertEqual(prepare.recommend(self.row(override=override))[0], override)

    def test_already_landed_requires_manual_disposition_not_retarget(self):
        self.assertEqual(prepare.recommend(self.row(probe="EMPTY"))[0], "KEEP_MAIN")

    def test_overlap_is_based_on_paths_not_title(self):
        prs = [
            {"number": 1, "title": "different fix", "files": ["pkg/engine/a.go"]},
            {"number": 2, "title": "same title", "files": ["pkg/cel/b.go"]},
        ]
        self.assertEqual(
            prepare.overlap(["pkg/engine/a.go"], prs),
            [{"number": 1, "title": "different fix", "files": ["pkg/engine/a.go"]}],
        )

    def test_identity_checks_metadata_and_state(self):
        row = {"head_sha": "abc", "base_ref": "main", "updated_at": "now"}
        pr = {
            "state": "open", "head": {"sha": "abc"}, "base": {"ref": "main"},
            "updated_at": "now",
        }
        self.assertTrue(prepare.same_identity(row, pr))
        self.assertFalse(prepare.same_identity(row, {**pr, "updated_at": "later"}))
        self.assertFalse(prepare.same_identity(row, {**pr, "state": "closed"}))

    def test_dated_snapshot_is_unapproved_and_conservative(self):
        snapshot = Path(__file__).with_name("snapshots") / "2026-10-20.json"
        manifest = json.loads(snapshot.read_text())
        rows = manifest["candidates"]
        self.assertEqual(len(rows), len({r["number"] for r in rows}))
        for row in rows:
            with self.subTest(number=row["number"]):
                self.assertIsNone(row["approval"])
                self.assertEqual(row["strategy"], "sibling")
                self.assertEqual(row["recommendation"], prepare.recommend(row)[0])
                self.assertTrue(prepare.SHA.fullmatch(row["head_sha"]))
                if row["probe"] == "CONFLICT":
                    self.assertTrue(row["conflict_files"])
                if row["probe"] == "OK":
                    self.assertTrue(prepare.SHA.fullmatch(row["rebased_sha"]))
                    self.assertIn("range_diff", row["evidence"])
                    self.assertIn("final_diff", row["evidence"])


if __name__ == "__main__":
    unittest.main()
