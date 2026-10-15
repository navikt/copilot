#!/usr/bin/env python3
"""Check line and token budgets for skills, agents, instructions, and harness files.

Usage:
  python3 scripts/check-harness-budget.py
  python3 scripts/check-harness-budget.py --diff origin/main
  python3 scripts/check-harness-budget.py --selftest
"""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

# ── Ceilings ─────────────────────────────────────────────────────────────────

SKILL_MAX_LINES = 500
SKILL_WARN_TOKENS = 4500
SKILL_MAX_TOKENS = 5000
SKILL_DESC_MAX_CHARS = 1024
SKILL_TOTAL_DESC_WARN_CHARS = 6800
SKILL_TOTAL_DESC_MAX_CHARS = 8000

AGENT_DESC_MAX_CHARS = 400
AGENT_TOTAL_DESC_WARN_CHARS = 2000
AGENT_TOTAL_DESC_MAX_CHARS = 2400

PRIMARY_AGENT_WARN_LINES = 350
PRIMARY_AGENT_MAX_LINES = 400
PRIMARY_AGENT_WARN_TOKENS = 6500
PRIMARY_AGENT_MAX_TOKENS = 7000

WORKER_AGENTS = {"worker", "local-worker"}
WORKER_AGENT_WARN_LINES = 100
WORKER_AGENT_MAX_LINES = 120
WORKER_AGENT_WARN_TOKENS = 1000
WORKER_AGENT_MAX_TOKENS = 1200

SPECIALIST_AGENT_WARN_LINES = 750
SPECIALIST_AGENT_MAX_LINES = 1000
SPECIALIST_AGENT_WARN_TOKENS = 6500
SPECIALIST_AGENT_MAX_TOKENS = 8000

ALWAYS_ON_MAX_FILES = 4
ALWAYS_ON_WARN_LINES = 130
ALWAYS_ON_MAX_LINES = 150
ALWAYS_ON_WARN_TOKENS = 2200
ALWAYS_ON_MAX_TOKENS = 2500

SCOPED_INSTR_DESC_MAX_CHARS = 300
SCOPED_INSTR_WARN_LINES = 650
SCOPED_INSTR_MAX_LINES = 800
SCOPED_INSTR_WARN_TOKENS = 5000
SCOPED_INSTR_MAX_TOKENS = 6000

GITHUB_INSTR_MAX_LINES = 120
GITHUB_INSTR_MAX_TOKENS = 1500

HOOK_PY_WARN_LINES = 550
HOOK_PY_MAX_LINES = 650
BRIDGE_JS_WARN_LINES = 450
BRIDGE_JS_MAX_LINES = 550

DIFF_WARN_DELTA_LINES = 100
DIFF_WARN_DELTA_TOKENS = 750


@dataclass
class FileMetric:
    rel_path: str
    category: str
    lines: int
    tokens: int
    max_lines: int
    max_tokens: int


@dataclass
class Report:
    errors: list[str] = field(default_factory=list)
    warnings: list[str] = field(default_factory=list)
    metrics: dict[str, FileMetric] = field(default_factory=dict)
    summary_rows: list[tuple[str, str, str]] = field(default_factory=list)

    def error(self, msg: str) -> None:
        self.errors.append(msg)

    def warn(self, msg: str) -> None:
        self.warnings.append(msg)


def parse_markdown(text: str) -> tuple[dict[str, str], str]:
    """Split YAML frontmatter from markdown body and parse top-level string fields."""
    lines = text.splitlines(keepends=True)
    if not lines or lines[0].strip() != "---":
        return {}, text

    end_idx = None
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            end_idx = i
            break
    if end_idx is None:
        return {}, text

    fm_lines = lines[1:end_idx]
    body = "".join(lines[end_idx + 1 :])
    fm: dict[str, str] = {}
    current_key: str | None = None
    block_parts: list[str] = []

    def flush_block() -> None:
        nonlocal current_key, block_parts
        if current_key is not None:
            fm[current_key] = " ".join(p.strip() for p in block_parts if p.strip())
            current_key = None
            block_parts = []

    for raw in fm_lines:
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if raw[0].isspace() and current_key is not None:
            block_parts.append(stripped)
            continue
        flush_block()
        if ":" not in raw:
            continue
        key, val = raw.split(":", 1)
        key = key.strip()
        val = val.strip()
        if val in (">", "|", ">-", "|-"):
            current_key = key
            block_parts = []
        else:
            if len(val) >= 2 and val[0] == val[-1] and val[0] in ("'", '"'):
                val = val[1:-1]
            fm[key] = val
    flush_block()
    return fm, body


def body_stats(body: str) -> tuple[int, int]:
    """Return (line_count, token_estimate) for a markdown body."""
    lines = len(body.splitlines())
    tokens = len(body.encode("utf-8")) // 4
    return lines, tokens


def check_repo(root: Path) -> Report:
    rep = Report()

    # 1. Skills (skills/*/SKILL.md)
    skills_dir = root / "skills"
    skill_desc_chars = 0
    skill_count = 0
    if skills_dir.is_dir():
        for skill_md in sorted(skills_dir.glob("*/SKILL.md")):
            skill_count += 1
            rel = skill_md.relative_to(root).as_posix()
            name = skill_md.parent.name
            fm, body = parse_markdown(skill_md.read_text(encoding="utf-8"))
            desc = fm.get("description", "")
            skill_desc_chars += len(desc)
            if not desc:
                rep.error(f"{rel}: missing frontmatter 'description'")
            elif len(desc) > SKILL_DESC_MAX_CHARS:
                rep.error(
                    f"{rel}: description is {len(desc)} chars (max {SKILL_DESC_MAX_CHARS})"
                )

            lines, tokens = body_stats(body)
            rep.metrics[rel] = FileMetric(
                rel, f"skill ({name})", lines, tokens, SKILL_MAX_LINES, SKILL_MAX_TOKENS
            )
            if lines > SKILL_MAX_LINES:
                rep.error(f"{rel}: body is {lines} lines (max {SKILL_MAX_LINES})")
            if tokens >= SKILL_MAX_TOKENS:
                rep.error(f"{rel}: body is ~{tokens} tokens (max {SKILL_MAX_TOKENS})")
            elif tokens >= SKILL_WARN_TOKENS:
                rep.warn(
                    f"{rel}: body is ~{tokens} tokens (warn at {SKILL_WARN_TOKENS})"
                )

    if skill_desc_chars > SKILL_TOTAL_DESC_MAX_CHARS:
        rep.error(
            f"skills/*/SKILL.md: aggregate frontmatter descriptions are {skill_desc_chars} chars "
            f"(~{skill_desc_chars // 4} tokens), max {SKILL_TOTAL_DESC_MAX_CHARS} chars"
        )
    elif skill_desc_chars >= SKILL_TOTAL_DESC_WARN_CHARS:
        rep.warn(
            f"skills/*/SKILL.md: aggregate frontmatter descriptions are {skill_desc_chars} chars "
            f"(warn at {SKILL_TOTAL_DESC_WARN_CHARS})"
        )
    rep.summary_rows.append(
        (
            f"Skill descriptions ({skill_count} skills)",
            f"{skill_desc_chars} chars (~{skill_desc_chars // 4} tok)",
            f"<= {SKILL_TOTAL_DESC_MAX_CHARS} chars",
        )
    )

    # 2. Agents (agents/*.agent.md)
    agents_dir = root / "agents"
    agent_desc_chars = 0
    agent_count = 0
    if agents_dir.is_dir():
        for agent_md in sorted(agents_dir.glob("*.agent.md")):
            agent_count += 1
            rel = agent_md.relative_to(root).as_posix()
            stem = agent_md.name[: -len(".agent.md")]
            fm, body = parse_markdown(agent_md.read_text(encoding="utf-8"))
            fm_name = fm.get("name", "")
            if fm_name not in (stem, f"{stem}-agent"):
                rep.error(
                    f"{rel}: frontmatter name {fm_name!r} does not match {stem!r} or '{stem}-agent'"
                )
            desc = fm.get("description", "")
            agent_desc_chars += len(desc)
            if not desc:
                rep.error(f"{rel}: missing frontmatter 'description'")
            elif len(desc) > AGENT_DESC_MAX_CHARS:
                rep.error(
                    f"{rel}: description is {len(desc)} chars (max {AGENT_DESC_MAX_CHARS})"
                )

            lines, tokens = body_stats(body)
            if stem == "nav-pilot":
                max_l, warn_l = PRIMARY_AGENT_MAX_LINES, PRIMARY_AGENT_WARN_LINES
                max_t, warn_t = PRIMARY_AGENT_MAX_TOKENS, PRIMARY_AGENT_WARN_TOKENS
                cat = "agent (primary)"
                rep.summary_rows.append(
                    (
                        "Primary agent (nav-pilot.agent.md)",
                        f"{lines} lines (~{tokens} tok)",
                        f"<= {max_l} lines / {max_t} tok",
                    )
                )
            elif stem in WORKER_AGENTS:
                max_l, warn_l = WORKER_AGENT_MAX_LINES, WORKER_AGENT_WARN_LINES
                max_t, warn_t = WORKER_AGENT_MAX_TOKENS, WORKER_AGENT_WARN_TOKENS
                cat = "agent (worker)"
            else:
                max_l, warn_l = SPECIALIST_AGENT_MAX_LINES, SPECIALIST_AGENT_WARN_LINES
                max_t, warn_t = (
                    SPECIALIST_AGENT_MAX_TOKENS,
                    SPECIALIST_AGENT_WARN_TOKENS,
                )
                cat = "agent (specialist)"

            rep.metrics[rel] = FileMetric(rel, cat, lines, tokens, max_l, max_t)
            if lines > max_l:
                rep.error(f"{rel}: body is {lines} lines (max {max_l} for {cat})")
            elif lines >= warn_l:
                rep.warn(f"{rel}: body is {lines} lines (warn at {warn_l} for {cat})")

            if tokens > max_t:
                rep.error(f"{rel}: body is ~{tokens} tokens (max {max_t} for {cat})")
            elif tokens >= warn_t:
                rep.warn(
                    f"{rel}: body is ~{tokens} tokens (warn at {warn_t} for {cat})"
                )

    if agent_desc_chars > AGENT_TOTAL_DESC_MAX_CHARS:
        rep.error(
            f"agents/*.agent.md: aggregate frontmatter descriptions are {agent_desc_chars} chars "
            f"(~{agent_desc_chars // 4} tokens), max {AGENT_TOTAL_DESC_MAX_CHARS} chars"
        )
    elif agent_desc_chars >= AGENT_TOTAL_DESC_WARN_CHARS:
        rep.warn(
            f"agents/*.agent.md: aggregate frontmatter descriptions are {agent_desc_chars} chars "
            f"(warn at {AGENT_TOTAL_DESC_WARN_CHARS})"
        )
    rep.summary_rows.append(
        (
            f"Agent descriptions ({agent_count} agents)",
            f"{agent_desc_chars} chars (~{agent_desc_chars // 4} tok)",
            f"<= {AGENT_TOTAL_DESC_MAX_CHARS} chars",
        )
    )

    # 3. Instructions (instructions/*.instructions.md)
    instr_dir = root / "instructions"
    always_on_files = 0
    always_on_lines = 0
    always_on_tokens = 0
    if instr_dir.is_dir():
        for instr_md in sorted(instr_dir.glob("*.instructions.md")):
            rel = instr_md.relative_to(root).as_posix()
            fm, body = parse_markdown(instr_md.read_text(encoding="utf-8"))
            apply_to = fm.get("applyTo", "")
            lines, tokens = body_stats(body)

            if not apply_to:
                rep.error(
                    f"{rel}: missing frontmatter 'applyTo' (empty applyTo is inlined into every session's AGENTS.md)"
                )
            elif apply_to == "**":
                always_on_files += 1
                always_on_lines += lines
                always_on_tokens += tokens
                rep.metrics[rel] = FileMetric(
                    rel,
                    "instruction (always-on)",
                    lines,
                    tokens,
                    ALWAYS_ON_MAX_LINES,
                    ALWAYS_ON_MAX_TOKENS,
                )
            else:
                desc = fm.get("description", "")
                if not desc:
                    rep.error(
                        f"{rel}: scoped instruction missing frontmatter 'description' (needed for AGENTS.md routing table)"
                    )
                elif len(desc) > SCOPED_INSTR_DESC_MAX_CHARS:
                    rep.error(
                        f"{rel}: description is {len(desc)} chars (max {SCOPED_INSTR_DESC_MAX_CHARS})"
                    )
                rep.metrics[rel] = FileMetric(
                    rel,
                    "instruction (scoped)",
                    lines,
                    tokens,
                    SCOPED_INSTR_MAX_LINES,
                    SCOPED_INSTR_MAX_TOKENS,
                )
                if lines > SCOPED_INSTR_MAX_LINES:
                    rep.error(
                        f"{rel}: body is {lines} lines (max {SCOPED_INSTR_MAX_LINES})"
                    )
                elif lines >= SCOPED_INSTR_WARN_LINES:
                    rep.warn(
                        f"{rel}: body is {lines} lines (warn at {SCOPED_INSTR_WARN_LINES})"
                    )
                if tokens > SCOPED_INSTR_MAX_TOKENS:
                    rep.error(
                        f"{rel}: body is ~{tokens} tokens (max {SCOPED_INSTR_MAX_TOKENS})"
                    )
                elif tokens >= SCOPED_INSTR_WARN_TOKENS:
                    rep.warn(
                        f"{rel}: body is ~{tokens} tokens (warn at {SCOPED_INSTR_WARN_TOKENS})"
                    )

    if always_on_files > ALWAYS_ON_MAX_FILES:
        rep.error(
            f"instructions/*.instructions.md: {always_on_files} always-on (applyTo: '**') files "
            f"(max {ALWAYS_ON_MAX_FILES})"
        )
    if always_on_lines > ALWAYS_ON_MAX_LINES:
        rep.error(
            f"instructions/*.instructions.md: always-on aggregate body is {always_on_lines} lines "
            f"(max {ALWAYS_ON_MAX_LINES})"
        )
    elif always_on_lines >= ALWAYS_ON_WARN_LINES:
        rep.warn(
            f"instructions/*.instructions.md: always-on aggregate body is {always_on_lines} lines "
            f"(warn at {ALWAYS_ON_WARN_LINES})"
        )
    if always_on_tokens > ALWAYS_ON_MAX_TOKENS:
        rep.error(
            f"instructions/*.instructions.md: always-on aggregate body is ~{always_on_tokens} tokens "
            f"(max {ALWAYS_ON_MAX_TOKENS})"
        )
    elif always_on_tokens >= ALWAYS_ON_WARN_TOKENS:
        rep.warn(
            f"instructions/*.instructions.md: always-on aggregate body is ~{always_on_tokens} tokens "
            f"(warn at {ALWAYS_ON_WARN_TOKENS})"
        )
    rep.summary_rows.append(
        (
            f"Always-on instructions ({always_on_files} files)",
            f"{always_on_lines} lines (~{always_on_tokens} tok)",
            f"<= {ALWAYS_ON_MAX_LINES} lines / {ALWAYS_ON_MAX_TOKENS} tok",
        )
    )

    # 4. GitHub review & repo instructions (.github/copilot-instructions.md, .github/instructions/*.instructions.md)
    repo_instr = root / ".github" / "copilot-instructions.md"
    if repo_instr.is_file():
        rel = repo_instr.relative_to(root).as_posix()
        _, body = parse_markdown(repo_instr.read_text(encoding="utf-8"))
        lines, tokens = body_stats(body)
        rep.metrics[rel] = FileMetric(
            rel,
            "github-instructions",
            lines,
            tokens,
            GITHUB_INSTR_MAX_LINES,
            GITHUB_INSTR_MAX_TOKENS,
        )
        if lines > GITHUB_INSTR_MAX_LINES:
            rep.error(f"{rel}: body is {lines} lines (max {GITHUB_INSTR_MAX_LINES})")
        if tokens > GITHUB_INSTR_MAX_TOKENS:
            rep.error(
                f"{rel}: body is ~{tokens} tokens (max {GITHUB_INSTR_MAX_TOKENS})"
            )
        rep.summary_rows.append(
            (
                "Repo instructions (.github/copilot-instructions.md)",
                f"{lines} lines (~{tokens} tok)",
                f"<= {GITHUB_INSTR_MAX_LINES} lines / {GITHUB_INSTR_MAX_TOKENS} tok",
            )
        )

    gh_instr_dir = root / ".github" / "instructions"
    if gh_instr_dir.is_dir():
        for gh_md in sorted(gh_instr_dir.glob("*.instructions.md")):
            rel = gh_md.relative_to(root).as_posix()
            fm, body = parse_markdown(gh_md.read_text(encoding="utf-8"))
            apply_to = fm.get("applyTo", "")
            if not apply_to:
                rep.error(f"{rel}: missing frontmatter 'applyTo'")
            elif apply_to == "**":
                rep.error(
                    f"{rel}: uses applyTo: '**'; repo-wide rules belong in .github/copilot-instructions.md"
                )
            lines, tokens = body_stats(body)
            rep.metrics[rel] = FileMetric(
                rel,
                "github-review-instructions",
                lines,
                tokens,
                GITHUB_INSTR_MAX_LINES,
                GITHUB_INSTR_MAX_TOKENS,
            )
            if lines > GITHUB_INSTR_MAX_LINES:
                rep.error(
                    f"{rel}: body is {lines} lines (max {GITHUB_INSTR_MAX_LINES})"
                )
            if tokens > GITHUB_INSTR_MAX_TOKENS:
                rep.error(
                    f"{rel}: body is ~{tokens} tokens (max {GITHUB_INSTR_MAX_TOKENS})"
                )

    # 5. Runtime hooks and provider JS bridges
    hooks_dir = root / "hooks"
    gh_hooks_dir = root / ".github" / "hooks"
    if hooks_dir.is_dir():
        for hook_py in sorted(hooks_dir.glob("*.py")):
            rel = hook_py.relative_to(root).as_posix()
            content = hook_py.read_text(encoding="utf-8")
            lines, tokens = body_stats(content)
            rep.metrics[rel] = FileMetric(
                rel, "hook (python)", lines, tokens, HOOK_PY_MAX_LINES, 0
            )
            if lines > HOOK_PY_MAX_LINES:
                rep.error(f"{rel}: {lines} lines (max {HOOK_PY_MAX_LINES})")
            elif lines >= HOOK_PY_WARN_LINES:
                rep.warn(f"{rel}: {lines} lines (warn at {HOOK_PY_WARN_LINES})")

            mirror = gh_hooks_dir / hook_py.name
            if gh_hooks_dir.is_dir():
                if not mirror.is_file():
                    rep.error(
                        f"{rel}: missing mirror at .github/hooks/{hook_py.name}"
                    )
                elif mirror.read_bytes() != hook_py.read_bytes():
                    rep.error(
                        f"{rel}: differs from .github/hooks/{hook_py.name} (copy hooks/{hook_py.name} to .github/hooks/)"
                    )

    provider_dir = root / "cli" / "nav-pilot" / "internal" / "provider"
    if provider_dir.is_dir():
        for bridge_js in sorted(provider_dir.glob("*.js")):
            rel = bridge_js.relative_to(root).as_posix()
            content = bridge_js.read_text(encoding="utf-8")
            lines, tokens = body_stats(content)
            rep.metrics[rel] = FileMetric(
                rel, "bridge (js)", lines, tokens, BRIDGE_JS_MAX_LINES, 0
            )
            if lines > BRIDGE_JS_MAX_LINES:
                rep.error(f"{rel}: {lines} lines (max {BRIDGE_JS_MAX_LINES})")
            elif lines >= BRIDGE_JS_WARN_LINES:
                rep.warn(f"{rel}: {lines} lines (warn at {BRIDGE_JS_WARN_LINES})")

    return rep


def is_measured_harness_path(rel: str) -> bool:
    """Return True if rel is a harness/customization file tracked by check_repo (or legacy review instructions)."""
    if rel in (
        ".github/copilot-instructions.md",
        ".github/copilot-review-instructions.md",
    ):
        return True
    if rel.startswith("skills/") and rel.endswith("/SKILL.md") and rel.count("/") == 2:
        return True
    if rel.startswith("agents/") and rel.endswith(".agent.md") and rel.count("/") == 1:
        return True
    if (
        rel.startswith("instructions/")
        and rel.endswith(".instructions.md")
        and rel.count("/") == 1
    ):
        return True
    if (
        rel.startswith(".github/instructions/")
        and rel.endswith(".instructions.md")
        and rel.count("/") == 2
    ):
        return True
    if rel.startswith("hooks/") and rel.endswith(".py") and rel.count("/") == 1:
        return True
    if (
        rel.startswith("cli/nav-pilot/internal/provider/")
        and rel.endswith(".js")
        and rel.count("/") == 4
    ):
        return True
    return False


def git_file_at(root: Path, rev: str, rel_path: str) -> str | None:
    proc = subprocess.run(
        ["git", "show", f"{rev}:{rel_path}"],
        cwd=root,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    if proc.returncode != 0:
        return None
    return proc.stdout


def check_diff(root: Path, base_ref: str, rep: Report) -> list[str]:
    """Compare changed harness/customization files against base_ref and return markdown table lines."""
    proc = subprocess.run(
        ["git", "diff", "--name-only", base_ref, "--"],
        cwd=root,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )
    if proc.returncode != 0:
        return []
    untracked = subprocess.run(
        ["git", "ls-files", "--others", "--exclude-standard"],
        cwd=root,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        check=False,
    )

    changed = [line.strip() for line in proc.stdout.splitlines() if line.strip()]
    if untracked.returncode == 0:
        changed.extend(
            line.strip() for line in untracked.stdout.splitlines() if line.strip()
        )
    rows: list[str] = []
    for rel in sorted(set(changed)):
        metric = rep.metrics.get(rel)
        if metric is None and not is_measured_harness_path(rel):
            continue
        base_text = git_file_at(root, base_ref, rel)
        if metric is None and base_text is None:
            continue
        if metric is not None and base_text is not None:
            if rel.endswith(".md"):
                _, base_body = parse_markdown(base_text)
                base_lines, base_tokens = body_stats(base_body)
            else:
                base_lines, base_tokens = body_stats(base_text)
            dl = metric.lines - base_lines
            dt = metric.tokens - base_tokens
            if dl > DIFF_WARN_DELTA_LINES or dt > DIFF_WARN_DELTA_TOKENS:
                rep.warn(
                    f"{rel}: grew by {dl:+d} lines / {dt:+d} tokens in this PR "
                    f"(warn threshold: +{DIFF_WARN_DELTA_LINES} lines or +{DIFF_WARN_DELTA_TOKENS} tokens)"
                )
            pct = f"{(metric.lines * 100) // metric.max_lines}%" if metric.max_lines else "-"
            rows.append(
                f"| `{rel}` | {metric.lines} ({dl:+d}) | ~{metric.tokens} ({dt:+d}) | {metric.max_lines} lines ({pct}) |"
            )
        elif metric is not None:
            pct = f"{(metric.lines * 100) // metric.max_lines}%" if metric.max_lines else "-"
            rows.append(
                f"| `{rel}` (new) | {metric.lines} (+{metric.lines}) | ~{metric.tokens} (+{metric.tokens}) | {metric.max_lines} lines ({pct}) |"
            )
        elif base_text is not None and not (root / rel).exists():
            if rel.endswith(".md"):
                _, base_body = parse_markdown(base_text)
                base_lines, base_tokens = body_stats(base_body)
            else:
                base_lines, base_tokens = body_stats(base_text)
            rows.append(
                f"| `{rel}` (removed) | 0 (-{base_lines}) | 0 (-{base_tokens}) | - |"
            )
    return rows


def format_markdown_summary(rep: Report, diff_rows: list[str]) -> str:
    out = [
        "### Harness & Customization Budget",
        "",
        "| Budget | Current | Ceiling |",
        "| :--- | :--- | :--- |",
    ]
    for label, cur, ceil in rep.summary_rows:
        out.append(f"| {label} | {cur} | {ceil} |")
    if diff_rows:
        out.extend(
            [
                "",
                "#### Changed in this PR",
                "",
                "| File | Lines (Δ) | Est. Tokens (Δ) | Line Ceiling |",
                "| :--- | :--- | :--- | :--- |",
                *diff_rows,
            ]
        )
    out.append("")
    return "\n".join(out)


def run_selftest() -> int:
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        (root / "skills" / "ok-skill").mkdir(parents=True)
        (root / "agents").mkdir(parents=True)
        (root / "instructions").mkdir(parents=True)
        (root / ".github" / "instructions").mkdir(parents=True)
        (root / "hooks").mkdir(parents=True)
        (root / ".github" / "hooks").mkdir(parents=True)

        # Valid baseline
        (root / "skills" / "ok-skill" / "SKILL.md").write_text(
            "---\nname: ok-skill\ndescription: Short skill description\n---\n# Body\n",
            encoding="utf-8",
        )
        (root / "agents" / "nav-pilot.agent.md").write_text(
            "---\nname: nav-pilot\ndescription: Primary coordinator\n---\n# Coordinator\n",
            encoding="utf-8",
        )
        (root / "agents" / "worker.agent.md").write_text(
            "---\nname: worker\ndescription: Fast worker\n---\n# Worker\n",
            encoding="utf-8",
        )
        (root / "instructions" / "core.instructions.md").write_text(
            '---\ndescription: Core\napplyTo: "**"\n---\n# Core\n',
            encoding="utf-8",
        )
        (root / "instructions" / "go.instructions.md").write_text(
            '---\ndescription: Go rules\napplyTo: "**/*.go"\n---\n# Go\n',
            encoding="utf-8",
        )
        (root / ".github" / "copilot-instructions.md").write_text(
            "# Repo instructions\n", encoding="utf-8"
        )
        (root / ".github" / "instructions" / "api.instructions.md").write_text(
            '---\napplyTo: "apps/**/*.go"\n---\n# API\n', encoding="utf-8"
        )
        (root / "hooks" / "gate.py").write_text("# hook\n", encoding="utf-8")
        (root / ".github" / "hooks" / "gate.py").write_text("# hook\n", encoding="utf-8")

        rep = check_repo(root)
        assert not rep.errors, f"expected clean baseline, got {rep.errors}"

        # 1. Missing applyTo on instruction must fail
        (root / "instructions" / "bad.instructions.md").write_text(
            "---\ndescription: Missing applyTo\n---\n# Oops\n", encoding="utf-8"
        )
        rep = check_repo(root)
        assert any("missing frontmatter 'applyTo'" in e for e in rep.errors), rep.errors
        (root / "instructions" / "bad.instructions.md").unlink()

        # 2. Oversized worker agent must fail
        big_worker = "---\nname: worker\ndescription: Fast worker\n---\n" + (
            "line\n" * (WORKER_AGENT_MAX_LINES + 5)
        )
        (root / "agents" / "worker.agent.md").write_text(big_worker, encoding="utf-8")
        rep = check_repo(root)
        assert any("worker.agent.md: body is" in e for e in rep.errors), rep.errors
        (root / "agents" / "worker.agent.md").write_text(
            "---\nname: worker\ndescription: Fast worker\n---\n# Worker\n",
            encoding="utf-8",
        )

        # 3. Always-on instruction budget exceeded must fail
        big_always = '---\ndescription: Core\napplyTo: "**"\n---\n' + (
            "line\n" * (ALWAYS_ON_MAX_LINES + 10)
        )
        (root / "instructions" / "core.instructions.md").write_text(
            big_always, encoding="utf-8"
        )
        rep = check_repo(root)
        assert any("always-on aggregate body" in e for e in rep.errors), rep.errors
        (root / "instructions" / "core.instructions.md").write_text(
            '---\ndescription: Core\napplyTo: "**"\n---\n# Core\n',
            encoding="utf-8",
        )

        # 4. Hook mirror drift must fail
        (root / ".github" / "hooks" / "gate.py").write_text(
            "# drifted\n", encoding="utf-8"
        )
        rep = check_repo(root)
        assert any("differs from .github/hooks/gate.py" in e for e in rep.errors), (
            rep.errors
        )

        # 5. Measured path filter excludes binary/unmeasured files
        assert is_measured_harness_path("skills/ok-skill/SKILL.md")
        assert is_measured_harness_path(".github/copilot-review-instructions.md")
        assert not is_measured_harness_path("docs/screenshot.png")
        assert not is_measured_harness_path("skills/ok-skill/references/guide.md")

    print("check-harness-budget.py: selftest passed")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--selftest", action="store_true", help="Run built-in unit tests"
    )
    parser.add_argument(
        "--diff",
        metavar="BASE_REF",
        help="Compare modified customization/harness files against BASE_REF",
    )
    args = parser.parse_args()

    if args.selftest:
        return run_selftest()

    root = Path(__file__).resolve().parent.parent
    rep = check_repo(root)
    diff_rows = check_diff(root, args.diff, rep) if args.diff else []

    for label, cur, ceil in rep.summary_rows:
        print(f"  ✓ {label}: {cur} (ceiling {ceil})")

    if diff_rows:
        print("\nChanged in this PR:")
        for row in diff_rows:
            print(f"  {row}")

    for w in rep.warnings:
        print(f"  ⚠ {w}")
    for e in rep.errors:
        print(f"  ❌ {e}", file=sys.stderr)

    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_path:
        try:
            with open(summary_path, "a", encoding="utf-8") as fh:
                fh.write(format_markdown_summary(rep, diff_rows) + "\n")
        except OSError as exc:
            # Step summary writing is best-effort and must not fail the budget check.
            print(f"  ⚠ could not write GITHUB_STEP_SUMMARY: {exc}", file=sys.stderr)

    if rep.errors:
        print(
            f"\nHarness budget check failed with {len(rep.errors)} error(s).",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
