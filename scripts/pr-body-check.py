#!/usr/bin/env python3
"""Validate a PR body against .github/pull_request_template.md.

Used by the pr-body-check job in .github/workflows/ci.yaml and by
`mise run pr:body-check`.

  pr-body-check.py FILE      read the body from FILE ("-" for stdin)
  pr-body-check.py --pr N    read the body of PR N with gh
  pr-body-check.py --selftest

In CI the body comes from PR_BODY, refreshed from the API when REPO and
PR_NUMBER are set, and bot authors (ACTOR ending in [bot]) are skipped.
"""
import os
import re
import subprocess
import sys

REQUIRED = [
    ("## Summary", ("## Summary", "## Oppsummering", "## Hva", "## Bakgrunn"), False),
    (
        "## Risk profile & rollout",
        ("## Risk profile & rollout", "## Risikoprofil og utrulling", "## Risiko og utrulling", "## Risiko"),
        True,
    ),
    ("## Security considerations", ("## Security considerations", "## Sikkerhetsvurderinger", "## Sikkerhet"), False),
    ("## Test plan", ("## Test plan", "## Testplan", "## Verifisering"), False),
]


def validate(body):
    stripped = re.sub(r"<!--.*?-->", "", body, flags=re.S)
    errors = []

    for label, aliases, needs_checkbox in REQUIRED:
        pattern = r"^(?:" + "|".join(re.escape(a) for a in aliases) + r")\s*$"
        m = re.search(pattern, stripped, flags=re.M | re.I)
        if not m:
            errors.append(f"Missing required heading: '{label}' (accepted: {', '.join(aliases)})")
            continue
        start = m.end()
        next_h = re.search(r"^##\s+\S", stripped[start:], flags=re.M)
        section = stripped[start : start + next_h.start()] if next_h else stripped[start:]
        content = re.sub(r"-\s*\[\s*\]\s*.*", "", section).strip()
        if not content:
            errors.append(f"Section '{label}' is empty (please fill it in)")
        if needs_checkbox and not re.search(r"-\s*\[[xX]\]", section):
            errors.append(f"Section '{label}' must have at least one checked item (- [x])")

    prose = re.sub(r"```.*?```|`[^`\n]+`", "", stripped, flags=re.S)
    bad_close = re.findall(r"\b(?:lukker|fikser|løser)\s+#\d+\b", prose, flags=re.I)
    if bad_close:
        errors.append(
            f"Use English issue-closing keywords ('Closes #NNN' or 'Fixes #NNN') instead of {', '.join(bad_close)}"
        )
    return errors


def selftest():
    good = "## Oppsummering\nx\n## Risiko og utrulling\n- [x] Lav\n## Sikkerhet\ny\n## Verifisering\nz\n"
    assert validate(good) == [], validate(good)
    assert any("checked item" in e for e in validate(good.replace("[x]", "[ ]")))
    assert any("Missing" in e for e in validate(good.replace("## Sikkerhet\n", "")))
    assert any("closing" in e for e in validate(good + "Lukker #1\n"))
    assert validate(good + "`Lukker #1`\n") == []
    print("pr-body-check selftest ok")


def main(argv):
    if argv[:1] == ["--selftest"]:
        selftest()
        return 0
    if argv[:1] == ["--pr"] and len(argv) == 2:
        body = subprocess.run(
            ["gh", "pr", "view", argv[1], "--json", "body", "--jq", ".body"],
            capture_output=True, text=True, check=True,
        ).stdout
    elif len(argv) == 1:
        if argv[0] == "-":
            body = sys.stdin.read()
        else:
            with open(argv[0], encoding="utf-8") as f:
                body = f.read()
    elif not argv and "PR_BODY" in os.environ:
        actor = os.environ.get("ACTOR", "")
        if actor.endswith("[bot]"):
            print(f"Skipping PR body check for automated bot PR (actor={actor})")
            return 0
        body = os.environ.get("PR_BODY") or ""
        repo = os.environ.get("REPO", "")
        pr_number = os.environ.get("PR_NUMBER", "")
        if repo and pr_number:
            res = subprocess.run(
                ["gh", "api", f"repos/{repo}/pulls/{pr_number}", "--jq", '.body // ""'],
                capture_output=True, text=True,
            )
            if res.returncode == 0:
                body = res.stdout
    else:
        print(__doc__, file=sys.stderr)
        return 2

    errors = validate(body)
    if errors:
        for err in errors:
            print(f"::error::{err}")
        print("\nPlease fill in all sections of .github/pull_request_template.md.", file=sys.stderr)
        return 1
    print("PR body matches .github/pull_request_template.md")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
