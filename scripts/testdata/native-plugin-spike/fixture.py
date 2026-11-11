"""Generate scratch-only A/B marketplaces and app links; never install anything."""

import argparse
import json
from pathlib import Path
from urllib.parse import quote

ROOT = Path(__file__).resolve().parent
SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
PLUGIN = "native-plugin-probe"
MARKETPLACE = "native-probe-local"


def install_links():
    app = "ghapp://plugins/install?source=" + quote(
        f"{PLUGIN}@{MARKETPLACE}", safe=""
    )
    return app, "https://github.com/copilot/app/launch?open=" + quote(app, safe="")


def generate(destination, variant):
    if variant not in ("A", "B"):
        raise ValueError("variant must be A or B")
    destination = Path(destination).resolve()
    if destination == ROOT or ROOT in destination.parents or destination in ROOT.parents:
        raise ValueError("choose a scratch destination outside the fixture")
    # Refuse reuse so a lifecycle trial cannot silently overwrite evidence.
    destination.mkdir(parents=True, exist_ok=False)
    for source in sorted((ROOT / "plugin").rglob("*")):
        if source.is_file():
            target = destination / "plugin" / source.relative_to(ROOT / "plugin")
            target.parent.mkdir(parents=True, exist_ok=True)
            text = source.read_text(encoding="utf-8").replace("-A-", f"-{variant}-")
            if source.name == "plugin.json" and variant == "B":
                text = text.replace('"0.0.1"', '"0.0.2"')
            target.write_text(text, encoding="utf-8")
    catalog = {
        "name": MARKETPLACE,
        "owner": {"name": "Local diagnostic fixture"},
        "plugins": [{"name": PLUGIN, "source": "./plugin",
                     "version": "0.0.1" if variant == "A" else "0.0.2"}],
    }
    (destination / "marketplace.json").write_text(
        json.dumps(catalog, indent=2) + "\n", encoding="utf-8"
    )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("destination", type=Path)
    parser.add_argument("--variant", choices=("A", "B"), default="A")
    args = parser.parse_args()
    generate(args.destination, args.variant)
    print("\n".join(install_links()))
