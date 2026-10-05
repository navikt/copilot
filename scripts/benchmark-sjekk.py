#!/usr/bin/env python3
"""Deterministic checks for the review, norsk and research benchmark suites.

Called by scripts/nav-pilot-golden.sh. Each subcommand exits 0 when the check
holds and 1 when it does not, with the reason on stdout. `--selftest` runs a
passing and a failing case for every check: a check that cannot fail proves
nothing.

  funnet  TX SPEC...        every planted defect is named within 3 lines of it
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
# The skill's suffix rules: -ingar (endringar) and -ane (filane). A stem of
# three letters keeps «vane» and «plane» out; bokmål compounds on these heads
# («jernbane», «arbeidsvane») are ordinary words, not definite plurals.
SUFFIX_RE = re.compile(r"(?<!%s)(%s{3,}(?:ingar|ane))(?!%s)" % (W, W, W), re.IGNORECASE)
BOKMAL_ANE = ("bane", "vane", "fane", "hane", "mane", "svane", "trane", "plane", "orkane")


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


# What `copilot -p` prints besides the answer: a tool header («● Read
# App.kt», «✗ Read mise.toml», «/ Search (grep)») and its indented detail
# lines («  │ src/main/kotlin/no/nav/demo/Routes.kt», «  └ 3 lines found»).
# Read from the 163 kept transcripts of 23 and 30 Sept. A tree in a code block
# («│   ├── App.kt») starts in column 0 and is kept: that is the answer.
TOOL_LINE = re.compile(r"^(?:[●✗] |/ \S|  [│└] )")


def answer_lines(text):
    """The transcript without tool output, so a check reads what the agent said."""
    return [l for l in text.splitlines() if not TOOL_LINE.match(l)]


# Numbers that are not line references: WCAG criteria (2.1.1), Tailwind
# classes (p-4, mx-8), JSX literals (tabIndex={5}), versions.
NOT_A_LINE = re.compile(r"\d+(\.\d+)+|[A-Za-z]+-\d+|\{\d+\}|\bv\d+")
RANGE = re.compile(r"(\d+)\s*[-–]\s*(\d+)")
LINE_CELL = re.compile(r"^\s*`?(?:L|linje\s*)?(\d+(?:\s*[-–]\s*\d+)?)`?\s*$", re.IGNORECASE)


def cited_lines(text):
    # A table row cites its line in a Line cell. Taking every integer in the
    # row would let «tabIndex 5 … linje 11» or a number in the prose pass.
    # Every number-only cell counts, so an index column («| 3 | … |») beside
    # the Line column does not hide it.
    if text.lstrip().startswith("|"):
        cells = [m.group(1) for m in map(LINE_CELL.match, text.split("|")) if m]
        if cells:
            text = " ".join(cells)
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


# funnet: the defect is named on a row that cites a line within NEAR of it,
# so a finding elsewhere that happens to use the word does not count. linje:
# the row cites the exact line. The gap between them is the off-by-one the
# 23 Sept screen found. Spec line 0 means the text only has to be named.
NEAR = 3


def review(mode, text, specs):
    rows = answer_lines(text)
    missing, wrong = [], []
    for spec in specs:
        name, regex, want = parse_spec(spec)
        hits = [r for r in rows if regex.search(r)]
        if want == {0}:
            if not hits:
                missing.append(f"{name} (anywhere)")
            continue
        near = [r for r in hits if any(abs(c - w) <= NEAR for c in cited_lines(r) for w in want)]
        if not near:
            missing.append(name)
        elif not any(cited_lines(r) & want for r in near):
            got = sorted(set().union(*(cited_lines(r) for r in near)))
            wrong.append(f"{name} (want {sorted(want)}, cited {got})")
    if missing:
        return f"not named near its line: {', '.join(missing)}"
    if mode == "linje" and wrong:
        return f"wrong line: {'; '.join(wrong)}"
    return None


def words(text):
    # Counted the way a writer counts, like `wc -w`: «nav-pilot» and
    # `.nav-pilot/config.toml` are one word each, a lone «#» or «-» none.
    return sum(1 for t in text.split() if re.search(W, t))


LOCATION = re.compile(r"\.kts?:\d+|linje \d+|line \d+", re.IGNORECASE)


def check(cmd, args):
    """Return None when the check holds, else the reason."""
    if cmd in ("funnet", "linje"):
        return review(cmd, Path(args[0]).read_text(), args[1:])
    text = Path(args[0]).read_text()
    if cmd == "nynorsk":
        found = {m.group(1).lower() for m in NYNORSK_RE.finditer(text)}
        found |= {w for m in SUFFIX_RE.finditer(text) if not (w := m.group(1).lower()).endswith(BOKMAL_ANE)}
        found = sorted(found)
        return f"nynorsk forms: {', '.join(found)}" if found else None
    if cmd == "floskler":
        found = sorted({m.group(0).lower() for m in _gate_markers().finditer(text)})
        return f"KI markers: {', '.join(found)}" if found else None
    if cmd == "ki":
        # The house rule is «not AI»; spelling out or abbreviating is free.
        # Channel names like #ki-utvikling are not prose.
        if re.search(r"(?<!%s)AI(?!%s)" % (W, W), re.sub(r"#\S+", "", text)):
            return "uses «AI»; Norwegian text says «KI»"
        return None
    if cmd == "lengde":
        n, lo, hi = words(text), int(args[1]), int(args[2])
        return None if lo <= n <= hi else f"{n} words, want {lo}-{hi}"
    if cmd == "ingen":
        subject, neg = re.compile(args[1], re.I), re.compile(args[2], re.I)
        answer = "\n".join(answer_lines(text))
        # Per sentence, so «X kalles fra A.kt. Ingen andre kall.» is no «none».
        sentences = re.split(r"\.\s|[!?\n]|\.$", answer)
        claims = [x.strip() for x in sentences if subject.search(x) and LOCATION.search(x) and not neg.search(x)]
        if claims:
            return f"places a call that does not exist: {claims[0][:80]}"
        if not any(subject.search(x) and neg.search(x) for x in sentences):
            return "no sentence says there is none"
        real = {p.name for p in Path(args[3]).rglob("*") if p.is_file()}
        named = {Path(m).name for m in re.findall(r"[\w./-]+\.(?:kts?|go|tsx?|ya?ml|json)\b", answer)}
        invented = sorted(named - real)
        return f"names files that do not exist: {', '.join(invented)}" if invented else None
    if cmd == "punkter":
        # Top-level list items in the answer; an indented sub-point belongs
        # to the point above it.
        answer = "\n".join(answer_lines(text))
        n = len(re.findall(r"^(?:[-*•]|\d+[.)])\s+\S", answer, re.MULTILINE))
        lo, hi = int(args[1]), int(args[2])
        return None if lo <= n <= hi else f"{n} list items, want {lo}-{hi}"
    raise SystemExit(f"unknown check: {cmd}")


def selftest():
    import tempfile

    tsx = "| `StatusPanel.tsx` | %d | 🔴 | `tabIndex={5}` bryter tabrekkefølgen (WCAG 2.4.3) |\n"
    spec = ["tabindex=tabindex@11"]
    NEG = ["slettOppgave", "ingen", "FX"]
    NEGV = ["slettOppgave", r"ingen|(fant|finner|finnes|fins|ser) verken", "FX"]
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
        ("nynorsk", "Filane er endra.", [], False),
        ("nynorsk", "Endringar i koden.", [], False),
        ("nynorsk", "Filene er endret. Endringer i koden.", [], True),
        ("nynorsk", "En vane, en jernbane og en arbeidsvane. Plane flater.", [], True),
        ("floskler", "Endringen gjør bygget raskere.", [], True),
        ("floskler", "En banebrytende og sømløs endring.", [], False),
        ("ki", "KI-assistenten svarer på norsk.", [], True),
        ("ki", "AI-assistenten svarer på norsk.", [], False),
        ("ki", "Assistenten svarer på norsk. Spørsmål går til #ki-utvikling.", [], True),
        ("ki", "AI-assistenten svarer. Spørsmål går til #ki-utvikling.", [], False),
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
        ("punkter", "- én\n  - under\n  - under\n- to\n", ["1", "2"], True),
        ("punkter", "/ Search (grep)\n  │ \"x\"\n- én\n- to\n- tre\n- fire\n", ["1", "3"], False),
        ("ingen", "slettOppgave kalles fra Routes.kt:20.", NEG, False),
        ("ingen", "slettOppgave kalles i Config.kt linje 3. Det finnes ingen andre kall.", NEG, False),
        # Batch 2 Astra, re2: «verken ... eller» is a negation (RE_NONE in nav-pilot-golden.sh).
        ("ingen", "Jeg fant verken definisjoner eller kall til `slettOppgave` i kodebasen.", NEGV, True),
        ("ingen", "Jeg finner verken definisjonen av eller kall til `slettOppgave` her.", NEGV, True),
        ("ingen", "Jeg undersøkte verken tester eller dokumentasjon, men slettOppgave kalles fra Config.kt:22.", NEGV, False),
        ("ingen", "Jeg fant verken A eller B. slettOppgave kalles fra Config.kt:22.", NEGV, False),
        # Tool output alone is not an answer.
        ("funnet", "/ Search (grep)\n  │ rtk grep -n tabIndex src/StatusPanel.tsx:11\n", spec, False),
        # Found near the defect, but one line off: funnet holds, linje fails.
        ("funnet", tsx % 10, spec, True),
        # The word elsewhere, far from the defect, is not the finding.
        ("funnet", "| `StatusPanel.tsx` | 2 | 🟡 | fjern ubrukt tabIndex-import |\n", spec, False),
        # An index column beside the Line column.
        ("linje", "| 1 | `StatusPanel.tsx` | 11 | 🔴 | `tabIndex={5}` |\n", spec, True),
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
