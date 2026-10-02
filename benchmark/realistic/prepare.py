#!/usr/bin/env python3
"""Prepare one offline attempt. Never starts an agent or contacts a model."""

import argparse
import hashlib
import json
import shutil
from datetime import datetime, timezone
from pathlib import Path

from check import ROOT

REPO = ROOT.parent.parent
OMIT = shutil.ignore_patterns("build", ".gradle", ".next", "node_modules")
TASKS = {str(path.relative_to(ROOT)): path for language in ("kotlin", "typescript")
         for path in (ROOT / language).iterdir() if (path / "workspace").is_dir()}


def hashes(directory: Path) -> dict[str, str]:
    return {str(path.relative_to(directory)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in sorted(directory.rglob("*")) if path.is_file()}


def prepare(task_name: str, output: Path, model: str, effort: str, direct_worker: bool = False) -> dict:
    task = TASKS[task_name]
    output = output.resolve()
    if output.is_relative_to(ROOT / "kotlin") or output.is_relative_to(ROOT / "typescript") or ROOT.is_relative_to(output):
        raise ValueError("Attempt directory must not overlap source fixtures")
    if output.exists():
        raise FileExistsError(output)
    for source in (task, REPO / "agents/nav-pilot.agent.md", REPO / "instructions"):
        if source.is_symlink() or (source.is_dir() and any(p.is_symlink() for p in source.rglob("*"))):
            raise ValueError(f"Symlink in attempt input: {source}")

    output.mkdir(parents=True)
    try:
        fixture = output / "evaluator/fixture"
        fixture.parent.mkdir()
        shutil.copytree(task, fixture, ignore=OMIT)
        workspace = output / "workspace"
        shutil.copytree(fixture / "workspace", workspace)
        if not direct_worker:
            agent_dir = workspace / ".github/agents"
            agent_dir.mkdir(parents=True)
            shutil.copy2(REPO / "agents/nav-pilot.agent.md", agent_dir / "nav-pilot.agent.md")
        instructions_dir = workspace / ".github/instructions"
        instructions_dir.mkdir(parents=True)
        for instruction in sorted((REPO / "instructions").glob("*.instructions.md")):
            shutil.copy2(instruction, instructions_dir / instruction.name)
        manifest = {
            "task": task_name,
            "prepared_at": datetime.now(timezone.utc).isoformat(),
            "requested_model": model,
            "requested_effort": effort,
            "agent": "default" if direct_worker else "nav-pilot",
            "agent_inputs": ("all repo instructions/*.instructions.md; no workspace agents or skills" if direct_worker
                             else "nav-pilot.agent.md and all repo instructions/*.instructions.md; no other workspace agents or skills"),
            "prompt": (fixture / "prompt.md").read_text(),
            "fixture_files": hashes(fixture),
            "workspace_files": hashes(workspace),
            "cli_version": None,
            "observed_model": None,
            "observed_effort": None,
            "credits": None,
            "usage_verified": False,
        }
        (output / "manifest.json").write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n")
        return manifest
    except BaseException:
        shutil.rmtree(output)
        raise


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("task", choices=sorted(TASKS))
    parser.add_argument("output", type=Path, help="New attempt directory outside the source fixture")
    parser.add_argument("--model", choices=("gpt-6-sol", "gpt-6-luna"), required=True)
    parser.add_argument("--effort", choices=("low", "medium"), required=True)
    parser.add_argument("--direct-worker", action="store_true", help="Use Copilot's default agent without nav-pilot's phase gates")
    args = parser.parse_args()
    if args.model == "gpt-6-luna" and args.effort != "medium":
        parser.error("Luna is only compared at medium effort")
    prepare(args.task, args.output, args.model, args.effort, args.direct_worker)
    print(args.output / "manifest.json")


if __name__ == "__main__":
    main()
