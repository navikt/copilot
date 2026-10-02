import shutil
import tempfile
import unittest
from pathlib import Path

from check import ROOT, apply_control, evaluate, evaluate_attempt
from prepare import prepare, hashes


class EvaluatorTests(unittest.TestCase):
    def test_prepared_attempts_are_identical_and_withhold_controls(self):
        task = ROOT / "typescript/incident"
        with self.assertRaisesRegex(ValueError, "overlap source fixtures"):
            prepare("typescript/incident", ROOT / "kotlin/cursor-debug/workspace/new-attempt", "gpt-6-sol", "medium")
        with tempfile.TemporaryDirectory(dir=ROOT, prefix=".nav-benchmark-test-") as temporary:
            first = Path(temporary) / "first"
            second = Path(temporary) / "second"
            manifest = prepare("typescript/incident", first, "gpt-6-sol", "medium")
            prepare("typescript/incident", second, "gpt-6-luna", "medium")
            self.assertEqual(hashes(first / "workspace"), hashes(second / "workspace"))
            self.assertEqual(hashes(first / "workspace"), manifest["workspace_files"])
            self.assertEqual(hashes(first / "evaluator/fixture"), manifest["fixture_files"])
            self.assertFalse((first / "workspace/controls").exists())
            self.assertFalse((first / "workspace/prompt.md").exists())
            self.assertTrue((first / "evaluator/fixture/controls/acceptance/invalid.test.ts").is_file())
            self.assertFalse((task / "workspace/.github").exists())
            with self.assertRaises(FileExistsError):
                prepare("typescript/incident", first, "gpt-6-sol", "medium")
            direct = Path(temporary) / "direct"
            direct_manifest = prepare("typescript/feature", direct, "gpt-6-luna", "medium", direct_worker=True)
            self.assertEqual(direct_manifest["agent"], "default")
            self.assertFalse((direct / "workspace/.github/agents").exists())
            self.assertEqual(len(list((direct / "workspace/.github/instructions").glob("*.instructions.md"))), 16)
            result = evaluate_attempt(first)
            self.assertEqual(result["status"], "failed")
            self.assertEqual(result["task"], "typescript/incident")
            (first / "evaluator/fixture/prompt.md").write_text("tampered")
            with self.assertRaisesRegex(ValueError, "Frozen fixture"):
                evaluate_attempt(first)
            (first / "evaluator/fixture/prompt.md").unlink()
            (first / "evaluator/fixture/prompt.md").symlink_to(task / "prompt.md")
            with self.assertRaisesRegex(ValueError, "symlinks"):
                evaluate_attempt(first)

    def test_generated_dependency_symlinks_are_not_candidate_source(self):
        task = ROOT / "typescript/incident"
        with tempfile.TemporaryDirectory(dir=ROOT, prefix=".nav-benchmark-test-") as temporary:
            candidate = Path(temporary) / "workspace"
            shutil.copytree(task / "workspace", candidate)
            (candidate / "node_modules/.bin").mkdir(parents=True)
            (candidate / "node_modules/.bin/tsc").symlink_to("../typescript/bin/tsc")
            result = evaluate(task, candidate)
            self.assertEqual(result["status"], "failed")
            (candidate / "lib/saker.ts").unlink()
            (candidate / "lib/saker.ts").symlink_to(task / "workspace/lib/saker.ts")
            with self.assertRaisesRegex(ValueError, "symlinks"):
                evaluate(task, candidate)

    def test_control_rejects_paths_outside_task(self):
        with tempfile.TemporaryDirectory(dir=ROOT, prefix=".nav-benchmark-test-") as temporary:
            root = Path(temporary)
            patch = root / "escape.patch"
            patch.write_text("*** Update File: ../outside\n@@\n-old\n+new\n")
            with self.assertRaises(ValueError):
                apply_control(root, patch)

    def test_agent_editable_regression_cannot_hide_behavior_loss(self):
        task = ROOT / "typescript/incident"
        with tempfile.TemporaryDirectory(dir=ROOT, prefix=".nav-benchmark-test-") as temporary:
            candidate = Path(temporary) / "workspace"
            shutil.copytree(task / "workspace", candidate)
            route = candidate / "app/api/saker/route.ts"
            route.write_text(
                'export async function GET() { return Response.json({error: "invalid"}, {status: 400}); }\n'
            )
            (candidate / "tests/regression.test.ts").write_text(
                'import { test } from "node:test"; test("pretend", () => {});\n'
            )
            result = evaluate(task, candidate)
            self.assertEqual(result["status"], "failed")
            self.assertEqual(result["checks"]["regression"]["passed"], 0)


if __name__ == "__main__":
    unittest.main()
