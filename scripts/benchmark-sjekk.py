#!/usr/bin/env python3
"""Deterministic checks for the review, norsk and research benchmark suites.

Called by scripts/nav-pilot-golden.sh. Each subcommand exits 0 when the check
holds and 1 when it does not, with the reason on stdout. `--selftest` runs a
passing and a failing case for every check: a check that cannot fail proves
nothing.

  funnet  TX SPEC...        every planted defect is named in the review
  linje   TX SPEC...        every planted defect is named with its line
  nynorsk FILE              no nynorsk forms (list: skills/klarsprak/SKILL.md)
  floskler FILE             no KI markers (list: hooks/klarsprak-gate.py)
  ki      FILE              says «KI», never «AI»
  lengde  FILE MIN MAX      word count within bounds
  ingen   TX SUBJ NEG DIR   a line naming SUBJ says there is none (NEG),
                            and the answer names no file DIR lacks
  punkter TX MIN MAX        MIN to MAX list items in the answer

SPEC is `navn=regex@linje[,linje]`: a transcript line matching regex names the
defect, and it is located when that same line cites one of the given line
numbers (a range of at most four lines that covers one counts too).
"""

import importlib.util
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
W = r"[\wæøåÆØÅ]"

# From «Nynorsk → bokmål» in skills/klarsprak/SKILL.md. Whole words only.
NYNORSK = (
    "oppgåve oppgåver eigenskap eigenskapar eigentleg handtere handtering "
    "tilgjengeleg tydeleg mogleg moglegheit moglegheiter viktigaste løysing "
    "løysingar brukaren brukarane teneste tenester endringar innstillingar "
    "oppdateringar naudsynt kjeldekode sjølv nokon kvar kvifor korleis fleire "
    "meir framleis ikkje medan mykje berre difor vart vorte dei eg kva frå "
    "òg noko"
).split() + ["til dømes", "mellom anna"]
NYNORSK_RE = re.compile(
    r"(?<!%s)(%s)(?!%s)" % (W, "|".join(re.escape(w) for w in NYNORSK), W),
    re.IGNORECASE,
)


def _gate_markers():
    spec = importlib.util.spec_from_file_location(
        "klarsprak_gate", REPO / "hooks" / "klarsprak-gate.py"
    )
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    # «helhetlig» is ordinary Norwegian in a text about a whole service; the
    # gate can afford the false positive on a commit, a score cannot.
    kept = [m for m in mod.MARKERS if not m.startswith("helhetlig")]
    return re.compile("|".join(r"(?<![\wæøå])(?:%s)" % m for m in kept), re.IGNORECASE)


# Numbers that are not line references: WCAG criteria (2.1.1), Tailwind
# classes (p-4, mx-8), JSX literals (tabIndex={5}), versions.
NOT_A_LINE = re.compile(r"\d+(\.\d+)+|[A-Za-z]+-\d+|\{\d+\}|\bv\d+")
RANGE = re.compile(r"(\d+)\s*[-–]\s*(\d+)")
LINE_CELL = re.compile(r"^\s*`?(?:L|linje\s*)?(\d+(?:\s*[-–]\s*\d+)?)`?\s*$", re.IGNORECASE)


def cited_lines(text):
    # A table row cites its line in the Line cell. Taking every integer in the
    # row would let «tabIndex 5 … linje 11» or a number in the prose pass.
    if text.lstrip().startswith("|"):
        cells = [m.group(1) for m in map(LINE_CELL.match, text.split("|")) if m]
        if cells:
            text = cells[0]
    text = NOT_A_LINE.sub(" ", text)
    lines = set()
    for a, b in RANGE.findall(text):
        a, b = int(a), int(b)
        if 0 <= b - a <= 3:
            lines.update(range(a, b + 1))
    for n in re.findall(r"\d+", RANGE.sub(" ", text)):
        lines.add(int(n))
    return lines


def parse_spec(spec):
    name, rest = spec.split("=", 1)
    regex, lines = rest.rsplit("@", 1)
    return name, re.compile(regex, re.IGNORECASE), {int(n) for n in lines.split(",")}


def review(mode, text, specs):
    rows = text.splitlines()
    missing, wrong = [], []
    for spec in specs:
        name, regex, want = parse_spec(spec)
        hits = [r for r in rows if regex.search(r)]
        if not hits:
            missing.append(name)
        elif not any(cited_lines(r) & want for r in hits):
            got = sorted(set().union(*(cited_lines(r) for r in hits)))
            wrong.append(f"{name} (want {sorted(want)}, cited {got or 'none'})")
    if missing:
        return f"not named: {', '.join(missing)}"
    if mode == "linje" and wrong:
        return f"wrong or missing line: {'; '.join(wrong)}"
    return None


def words(text):
    # Counted the way a writer counts, like `wc -w`: «nav-pilot» and
    # `.nav-pilot/config.toml` are one word each, a lone «#» or «-» none.
    return sum(1 for t in text.split() if re.search(W, t))


def check(cmd, args):
    """Return None when the check holds, else the reason."""
    if cmd in ("funnet", "linje"):
        return review(cmd, Path(args[0]).read_text(), args[1:])
    text = Path(args[0]).read_text()
    if cmd == "nynorsk":
        found = sorted({m.group(1).lower() for m in NYNORSK_RE.finditer(text)})
        return f"nynorsk forms: {', '.join(found)}" if found else None
    if cmd == "floskler":
        found = sorted({m.group(0).lower() for m in _gate_markers().finditer(text)})
        return f"KI markers: {', '.join(found)}" if found else None
    if cmd == "ki":
        # The rule is «not AI». Spelling out «kunstig intelligens» keeps it;
        # the prompt does not name «KI», or this would test obedience instead.
        if re.search(r"(?<!%s)AI(?!%s)" % (W, W), text):
            return "uses «AI»; Norwegian text says «KI»"
        if not re.search(r"(?<!%s)KI(?!%s)|kunstig intelligens" % (W, W), text, re.IGNORECASE):
            return "never names KI, so the choice was not tested"
        return None
    if cmd == "lengde":
        n, lo, hi = words(text), int(args[1]), int(args[2])
        return None if lo <= n <= hi else f"{n} words, want {lo}-{hi}"
    if cmd == "ingen":
        subject, neg = re.compile(args[1], re.I), re.compile(args[2], re.I)
        # Per sentence, so «X kalles fra A.kt. Ingen andre kall.» is no «none».
        sentences = re.split(r"\.\s|[!?\n]|\.$", text)
        if not any(subject.search(s) and neg.search(s) for s in sentences):
            return "no sentence says there is none"
        real = {p.name for p in Path(args[3]).rglob("*") if p.is_file()}
        # The answer only: a tool line («✗ Read package.json … does not
        # exist») may name a missing file, and that is the agent looking.
        answer = "\n".join(l for l in text.splitlines() if not re.match(r"\s*[●✗│└]", l))
        named = {Path(m).name for m in re.findall(r"[\w./-]+\.(?:kts?|go|tsx?|ya?ml|json)\b", answer)}
        invented = sorted(named - real)
        return f"names files that do not exist: {', '.join(invented)}" if invented else None
    if cmd == "punkter":
        # List items in the answer. Tool lines start with ●, │ or └, not these.
        n = len(re.findall(r"^\s*(?:[-*•]|\d+[.)])\s+\S", text, re.MULTILINE))
        lo, hi = int(args[1]), int(args[2])
        return None if lo <= n <= hi else f"{n} list items, want {lo}-{hi}"
    raise SystemExit(f"unknown check: {cmd}")


def selftest():
    import tempfile

    tsx = "| `StatusPanel.tsx` | %d | 🔴 | `tabIndex={5}` bryter tabrekkefølgen (WCAG 2.4.3) |\n"
    spec = ["tabindex=tabindex@11"]
    NEG = ["slettOppgave", "ingen", "FX"]
    cases = [
        # (cmd, file text, extra args, expect pass)
        ("funnet", tsx % 11, spec, True),
        ("funnet", "Ingen funn.\n", spec, False),
        ("linje", tsx % 11, spec, True),
        ("linje", tsx % 10, spec, False),  # Opus 5.5 Medium, 23 Sept: one line up
        ("linje", "| a.kt | 12-13 | 🟡 | catch svelger feil |\n", ["catch=catch@12,13"], True),
        ("linje", "| a.kt | 1-20 | 🟡 | catch svelger feil |\n", ["catch=catch@12,13"], False),
        ("nynorsk", "Vi retter feilen og sender den ut.", [], True),
        ("nynorsk", "Vi rettar ikkje feilen.", [], False),
        ("floskler", "Endringen gjør bygget raskere.", [], True),
        ("floskler", "En banebrytende og sømløs endring.", [], False),
        ("ki", "KI-assistenten svarer på norsk.", [], True),
        ("ki", "AI-assistenten svarer på norsk.", [], False),
        ("ki", "Assistenten svarer på norsk.", [], False),
        ("ki", "Assistenten bruker kunstig intelligens.", [], True),
        ("floskler", "En helhetlig tjeneste.", [], True),
        ("linje", "| a.tsx | 10 | 🔴 | `tabIndex={5}`, se linje 11 |\n", spec, False),
        ("linje", "| a.tsx | L11 | 🔴 | `tabIndex={5}` |\n", spec, True),
        ("lengde", "ett to tre", ["3", "5"], True),
        ("lengde", "ett to", ["3", "5"], False),
        ("ingen", "✗ Read Slett.kt\nIngen kaller slettOppgave. Config.kt definerer maksAntall.", NEG, True),
        ("ingen", "slettOppgave kalles fra Config.kt:22. Ingen andre kall.", NEG, False),
        ("ingen", "Ingen kall til slettOppgave, men OppgaveService.kt:14 sletter.", NEG, False),
        ("punkter", "Oppsummert:\n- én\n- to\n1. tre\n", ["1", "3"], True),
        ("punkter", "- én\n- to\n- tre\n- fire\n", ["1", "3"], False),
    ]
    failed = 0
    with tempfile.TemporaryDirectory() as tmp:
        f = Path(tmp) / "t.txt"
        (Path(tmp) / "fx").mkdir()
        (Path(tmp) / "fx" / "Config.kt").write_text("")
        for cmd, text, extra, expect in cases:
            f.write_text(text)
            extra = [str(Path(tmp) / "fx") if a == "FX" else a for a in extra]
            ok = check(cmd, [str(f), *extra]) is None
            if ok != expect:
                failed += 1
                print(f"FAIL {cmd} {text!r}: expected {'pass' if expect else 'fail'}")
    print(f"{len(cases) - failed}/{len(cases)} selftest cases passed")
    return 1 if failed else 0


if __name__ == "__main__":
    if sys.argv[1:2] == ["--selftest"]:
        sys.exit(selftest())
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    reason = check(sys.argv[1], sys.argv[2:])
    if reason:
        print(reason)
        sys.exit(1)
