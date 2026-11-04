import json
from pathlib import Path
import re
import tempfile
import unittest
from unittest.mock import patch

import export


class BetaExportTests(unittest.TestCase):
    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.package = export.export(Path(scratch.name) / "nav-design-beta")

    def test_all_canonical_runtime_files_are_byte_identical(self):
        canonical = export.files(export.ROOT / "skills/aksel-builder")
        del canonical["README.md"]
        del canonical["metadata.json"]
        self.assertEqual(export.files(self.package / "skills/aksel-builder"),
                         canonical)
        self.assertEqual(export.files(self.package / "com.github.copilot"),
                         export.files(export.SOURCE / "com.github.copilot"))

    def test_skill_reference_links_resolve(self):
        skill = self.package / "skills/aksel-builder"
        for path in [skill / "SKILL.md", *(skill / "references").glob("*.md")]:
            for link in re.findall(r"\]\(([^)]+)\)", path.read_text()):
                if ":" in link or link.startswith("#"):
                    continue
                target = (path.parent / link.split("#")[0]).resolve()
                self.assertTrue(target.is_relative_to(skill), (path, link))
                self.assertTrue(target.is_file(), (path, link))

    def test_exact_native_manifests(self):
        self.assertEqual(json.loads((self.package / "plugin.json").read_text()), {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": "nav-design-beta", "version": "0.1.0-beta.1",
            "description": "Opt-in Aksel and Figma prototyping in existing projects",
            "license": "MIT",
        })
        self.assertEqual(json.loads((self.package / "mcp.json").read_text()), {
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {
                "io.github.navikt/aksel-mcp": {
                    "type": "streamable-http", "url": "https://aksel-mcp.nav.no/mcp",
                },
                "com.figma/figma-mcp": {
                    "type": "streamable-http", "url": "https://mcp.figma.com/mcp",
                },
            },
        })
        self.assertFalse((self.package / "marketplace.json").exists())
        self.assertFalse((self.package / ".github").exists())
        self.assertFalse((self.package / "com.github.copilot/hooks").exists())

    def test_agent_frontmatter_tools_and_setup_prohibition(self):
        text = (self.package / "com.github.copilot/agents/nav-design-beta.agent.md").read_text()
        _, frontmatter, body = text.split("---", 2)
        self.assertEqual(frontmatter.splitlines()[1:4], [
            "name: nav-design-beta",
            "description: Betatesting av Aksel- og Figma-prototyper i eksisterende prosjekter etter samtykke",
            "tools:",
        ])
        self.assertEqual([line.removeprefix("  - ") for line in frontmatter.splitlines()[4:]], [
            "read", "edit", "search", "web", "execute",
            "io.github.navikt/aksel-mcp/aksel_find_docs",
            "io.github.navikt/aksel-mcp/aksel_get_doc",
            "io.github.navikt/aksel-mcp/aksel_get_component_info",
            "io.github.navikt/aksel-mcp/aksel_get_token_details",
            "io.github.navikt/aksel-mcp/aksel_find_icons",
            "com.figma/figma-mcp/get_design_context",
            "com.figma/figma-mcp/get_screenshot",
            "com.figma/figma-mcp/get_metadata",
        ])
        self.assertNotIn("model:", frontmatter)
        for required in (
            "Load the bundled `aksel-builder` skill", "overrides skill installation guidance",
            "Do not execute installation or configuration snippets", "write client configuration",
            "If Figma authentication is missing or denied, stop", "normal client approvals",
            "or background execution", "allowlist is not global policy", "no sandbox guarantees",
        ):
            self.assertIn(required, body)

    def test_marketplace_matches_package(self):
        catalog = json.loads((export.ROOT / ".github/plugin/marketplace.json").read_text())
        self.assertEqual(catalog["name"], "nav-pilot")
        self.assertEqual(len(catalog["plugins"]), 1)
        entry = catalog["plugins"][0]
        manifest = json.loads((self.package / "plugin.json").read_text())
        self.assertEqual(entry["name"], manifest["name"])
        self.assertEqual(entry["version"], manifest["version"])
        self.assertEqual((export.ROOT / entry["source"]).resolve(), export.OUTPUT)

    def test_check_detects_drift_without_writing(self):
        self.assertEqual(export.export(self.package, check=True), self.package)
        (self.package / "plugin.json").write_text("user edit")
        before = export.files(self.package)
        with self.assertRaises(ValueError):
            export.export(self.package, check=True)
        self.assertEqual(export.files(self.package), before)

    def test_managed_regeneration_and_overwrite_refusal(self):
        expected = export.payload()
        expected["README.md"] += b"\nChanged source\n"
        with patch.object(export, "OUTPUT", self.package), patch.object(export, "payload", return_value=expected):
            export.export(self.package)
        self.assertEqual((self.package / "README.md").read_bytes(), expected["README.md"])
        for name in ("README.md", "user.txt"):
            with self.subTest(name=name):
                (self.package / name).write_text("user work")
                before = export.files(self.package)
                with patch.object(export, "OUTPUT", self.package), self.assertRaises(ValueError):
                    export.export(self.package)
                self.assertEqual(export.files(self.package), before)

    def test_refuses_arbitrary_existing_output_and_symlinks(self):
        (self.package / "README.md").write_text("user work")
        with self.assertRaises(FileExistsError):
            export.export(self.package)
        link = self.package.parent / "link"
        link.symlink_to(self.package, target_is_directory=True)
        with self.assertRaises(ValueError):
            export.export(link)
        for path in (export.ROOT, export.ROOT / "scratch-beta", export.ROOT.parent):
            with self.assertRaises(ValueError):
                export.export(path)


if __name__ == "__main__":
    unittest.main()
