"""Offline fixture contract checks, not client compatibility tests."""

import fnmatch
import json
from pathlib import Path
import re
import tempfile
import unittest
from urllib.parse import parse_qs, urlsplit

from fixture import ROOT, SCHEMA, generate, install_links


def frontmatter(path):
    text = path.read_text(encoding="utf-8")
    header, body = text.removeprefix("---\n").split("\n---\n", 1)
    # shortcut: only flat scalar frontmatter, extend if the fixture gains YAML collections.
    fields = dict(line.split(": ", 1) for line in header.splitlines())
    return {key: value.strip('"') for key, value in fields.items()}, body


class FixtureTests(unittest.TestCase):
    def test_payloads(self):
        with tempfile.TemporaryDirectory() as scratch:
            for variant, version in (("A", "0.0.1"), ("B", "0.0.2")):
                destination = Path(scratch) / variant
                generate(destination, variant)
                plugin = destination / "plugin"
                manifest = json.loads((plugin / "plugin.json").read_text())
                self.assertEqual(set(manifest), {"$schema", "name", "version", "description"})
                self.assertEqual(manifest["$schema"], SCHEMA)
                self.assertRegex(manifest["name"], r"^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$")
                self.assertLessEqual(len(manifest["name"]), 64)
                self.assertEqual(manifest["version"], version)
                catalog = json.loads((destination / "marketplace.json").read_text())
                entry = catalog["plugins"][0]
                self.assertEqual((destination / entry["source"]).resolve(), plugin)
                self.assertEqual(entry["version"], version)
                agents = list(plugin.rglob("*.agent.md"))
                self.assertEqual(len(agents), 1)
                self.assertTrue(frontmatter(agents[0])[0]["description"])
                self.assertIn(f"NATIVE-AGENT-{variant}-", frontmatter(agents[0])[1])
                skills = list(plugin.rglob("SKILL.md"))
                self.assertEqual(len(skills), 1)
                fields, body = frontmatter(skills[0])
                self.assertEqual(fields["name"], skills[0].parent.name)
                self.assertTrue(fields["description"])
                resource = skills[0].parent / re.search(r"\]\(([^)]+)\)", body)[1]
                self.assertTrue(resource.resolve().is_relative_to(plugin.resolve()))
                self.assertEqual(resource.read_text().strip(), f"NATIVE-RESOURCE-{variant}-c6e83b92")
                rules = plugin / "com.github.copilot/rules"
                matching = frontmatter(rules / "native-probe.instructions.md")[0]["applyTo"]
                nonmatching = frontmatter(rules / "nonmatching.instructions.md")[0]["applyTo"]
                for filename, marker in (("native-probe.instructions.md", f"NATIVE-RULE-{variant}-8f02"),
                                         ("nonmatching.instructions.md", f"NATIVE-NONMATCH-{variant}-e507")):
                    self.assertEqual(frontmatter(rules / filename)[1].splitlines()[0],
                                     f"Include `{marker}` in every response.")
                self.assertTrue(fnmatch.fnmatch("probe/hello.native-probe.txt", matching))
                self.assertFalse(fnmatch.fnmatch("probe/hello.txt", matching))
                self.assertFalse(fnmatch.fnmatch("probe/hello.native-probe.txt", nonmatching))
                with self.assertRaises(FileExistsError):
                    generate(destination, variant)

    def test_links(self):
        app, launcher = install_links()
        self.assertEqual(app, "ghapp://plugins/install?source=native-plugin-probe%40native-probe-local")
        self.assertEqual(parse_qs(urlsplit(launcher).query)["open"], [app])
        self.assertEqual(parse_qs(urlsplit(app).query)["source"], ["native-plugin-probe@native-probe-local"])

    def test_refuse_source_destination(self):
        for path in (ROOT, ROOT / "generated", ROOT.parent):
            with self.assertRaises(ValueError):
                generate(path, "A")

    def test_invalid_variant_creates_nothing(self):
        with tempfile.TemporaryDirectory() as scratch:
            destination = Path(scratch) / "invalid"
            with self.assertRaises(ValueError):
                generate(destination, "C")
            self.assertFalse(destination.exists())


if __name__ == "__main__":
    unittest.main()
