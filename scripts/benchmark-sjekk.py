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
  prioritet TX SPEC...      each defect's row carries a high-priority marker;
                            a SPEC named `!navn` must not carry one
  svar    TX                print the answer without tool lines (not a check)
  taus    TX                no finding row carries a high-priority marker,
                            and the answer says there is nothing critical
  kritisk TX SPEC...        each defect's row is marked critical (🔴,
                            «kritisk», «critical»; «høy» is not enough)
  kritiske TX               count finding rows marked critical; passes on 0

SPEC is `navn=regex@linje[,linje]`: a transcript line matching regex names the
defect, and it is located when that same line cites one of the given line
numbers (a range of at most four lines that covers one counts too).
`navn=regex@Fil.kt:linje[,linje]` also requires the file name on that line,
so a right line in the wrong file does not count. Several SPECs with the same
navn are alternatives: a finding that can be cited in either of two files.
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
    fil = None
    if ":" in lines:
        fil, lines = lines.rsplit(":", 1)
    return name, re.compile(regex, re.IGNORECASE), {int(n) for n in lines.split(",")}, fil


# A file reference: «VedtakRepository.kt», «vedtak/VedtakConsumer.kt:31–33»,
# «nais.yaml:25,26».
FILE_REF = re.compile(
    r"([\w./-]+\.(?:kts?|ya?ml|tsx?|go|json))(?::(\d+(?:\s*[-–]\s*\d+)?(?:\s*,\s*\d+(?:\s*[-–]\s*\d+)?)*))?"
)


FILE_CELL = re.compile(r"[`*\s,/+]*")
NUMBERS = re.compile(r"\s*`?\d[\d\s,–-]*`?\s*")


def _block(text):
    """Lines a citation covers: a range of at most four lines whole, a longer
    block by its ends (a reviewer may cite «VedtakConsumer.kt | 23–28»)."""
    text = NOT_A_LINE.sub(" ", text)
    lines = cited_lines(text)
    return lines | {int(n) for a in RANGE.findall(text) for n in a}


def locations(row):
    """(file, line) pairs a row cites. One rule for every arm (#1443):
    a location is a file name and a line number, read from the File and Line
    cells, from a «Fil.kt:23» cell, or from a «Fil.kt:23» reference anywhere
    in the row. A file name without a line, or a YAML key in the Line cell
    («nais.yaml | inbound»), cites no line. A file named in the text of
    another file's row is not located there."""
    locs = set()
    for m in FILE_REF.finditer(row):
        if m.group(2):
            locs |= {(Path(m.group(1)).name.lower(), n) for n in _block(m.group(2))}
    if row.lstrip().startswith("|"):
        cells = row.split("|")
        # The File cell holds file names and separators only («`App.kt`,
        # `nais.yaml`»); the Line cell numbers only («25, 32–33»).
        named = {Path(m.group(1)).name.lower() for c in cells if FILE_CELL.fullmatch(FILE_REF.sub("", c))
                 for m in FILE_REF.finditer(c) if not m.group(2)}
        lines = set().union(*(_block(c) for c in cells if LINE_CELL.match(c) or NUMBERS.fullmatch(c)))
    else:
        named = {Path(m.group(1)).name.lower() for m in FILE_REF.finditer(row) if not m.group(2)}
        lines = _block(row) if len(named) == 1 else set()
    return locs | {(f, n) for f in named for n in lines}


def lines_in(row, fil):
    """Lines a row cites. With a file, only lines cited for that file."""
    if fil is None:
        return cited_lines(row)
    return {n for f, n in locations(row) if f == fil.lower()}


def located(rows, regex, want, fil):
    """Rows naming the defect within NEAR lines of it, in the right file."""
    hits = [r for r in rows if regex.search(r)]
    if want == {0}:
        return hits
    return [r for r in hits if any(abs(c - w) <= NEAR for c in lines_in(r, fil) for w in want)]


def grouped(specs):
    groups = {}
    for spec in specs:
        name, *rest = parse_spec(spec)
        groups.setdefault(name, []).append(rest)
    return groups


# funnet: the defect is named on a row that cites a line within NEAR of it,
# so a finding elsewhere that happens to use the word does not count. linje:
# the row cites the exact line. The gap between them is the off-by-one the
# 23 Sept screen found. Spec line 0 means the text only has to be named.
NEAR = 3


def review(mode, text, specs):
    rows = answer_lines(text)
    missing, wrong = [], []
    for name, alts in grouped(specs).items():
        near = [(r, want, fil) for regex, want, fil in alts for r in located(rows, regex, want, fil)]
        if all(want == {0} for _, want, _ in alts):
            if not near:
                missing.append(f"{name} (anywhere)")
            continue
        if not near:
            missing.append(name)
        elif not any(lines_in(r, fil) & want for r, want, fil in near):
            got = sorted(set().union(*(lines_in(r, fil) for r, _, fil in near)))
            want = sorted(set().union(*(w for _, w, _ in alts)))
            wrong.append(f"{name} (want {want}, cited {got})")
    if missing:
        return f"not named near its line: {', '.join(missing)}"
    if mode == "linje" and wrong:
        return f"wrong line: {'; '.join(wrong)}"
    return None


def words(text):
    # Counted the way a writer counts, like `wc -w`: «nav-pilot» and
    # `.nav-pilot/config.toml` are one word each, a lone «#» or «-» none.
    return sum(1 for t in text.split() if re.search(W, t))


# A high-priority marker: the persona's 🔴 Blocker and the words a reviewer
# uses for it. «høy» and not «høyt»: «høyt nivå» is not a priority.
HIGH = re.compile(r"(?<![\wæøå])(kritisk|blokker|blocker|critical|høy(?![\wæøå])|P0(?!\w))|🔴", re.IGNORECASE)
# «Ingen kritiske funn», «no blocking issues»: the clean verdict rv8 wants.
# «No blockers in `SakService.kt`» is Opus' wording (second check run, 7 Oct).
# «Jeg fant ingen konkrete feil», «Ingen konkrete funn» (GPT-6 Luna, 7 Oct, #1443).
CLEAN = re.compile(
    r"ingen\s+(kritiske|alvorlige|blokkerende|blokkere|blockere)|ingen\s+(konkrete\s+)?(feil|funn)"
    r"|no\s+(blocking|critical|blockers?)|ingen\s+🔴",
    re.IGNORECASE,
)
# A hedged clean verdict (GPT-6 Sol, 7 Oct, #1453): «Jeg fant ingen påvist
# tilgang …», «Ingen åpenbar tilgangslekkasje», «ingen bekreftet blokkering».
# Read only as taus' verdict, never to excuse a row: a 🔴 row still fails rv8.
HEDGED = re.compile(
    r"ingen\s+(tydelig|påvist|åpenbar|bekreftet|bekreftede)e?\s+"
    # A defect noun, not a missing safeguard: «tilgang til …», «…feil», not
    # «tilgangskontroll» or «feilhåndtering».
    r"([\wæøå-]*(feil|funn|blokkering|blokkere|lekkasje|sårbarhet)(?![\wæøå])|tilgang\s+til)",
    re.IGNORECASE,
)
PRIO_EMOJI = re.compile(r"🔴|🟠|🟡|🟢|💭|⚪")
PRIO_WORD = re.compile(
    r"\W*(kritisk|blokker\w*|blocker|critical|høy|high|middels|medium|lav|low|p[0-3]|forslag|nit|bør\s+\w+)\W*",
    re.IGNORECASE,
)


def high(row):
    """The row's priority is high. In a table only the Priority cell counts, so
    «blokkerende JDBC-kall» or «høy belastning» in a 🟡 row is not high
    (#1443). Outside a table only 🔴 marks a finding high: a prose «Blokkerende
    kall (linje 23)» names a topic, not a priority."""
    if row.lstrip().startswith("|"):
        cells = [c for c in row.split("|") if PRIO_EMOJI.search(c) or PRIO_WORD.fullmatch(c)]
        return any(HIGH.search(c) for c in cells[:1])
    return "🔴" in row


RED_ZONE = re.compile(r"rød\s+sone|red\s+zone", re.IGNORECASE)


def prioritet(text, specs):
    rows = answer_lines(text)
    bad = []
    for name, alts in grouped(specs).items():
        near = [r for regex, want, fil in alts for r in located(rows, regex, want, fil)]
        if name.startswith("!"):
            if any(high(r) for r in near):
                bad.append(f"{name[1:]} marked high")
        elif not near:
            bad.append(f"{name} not named near its line")
        elif not any(high(r) for r in near):
            bad.append(f"{name} not marked high")
    return f"priority: {', '.join(bad)}" if bad else None


def spurious(text):
    """Rows that put a high-priority marker on a line number."""
    # «🔴 Rød sone: … linje 50» is the persona's red-zone declaration, not a
    # finding (Opus check run on the clean file, 7 Oct).
    return [r for r in answer_lines(text)
            # A block counts as a citation too: «| 50–55 | 🔴 |» (GPT-6 Sol,
            # 7 Oct second run) is a high finding though cited_lines drops it.
            if high(r) and _block(r) and not CLEAN.search(r) and not RED_ZONE.search(r)]


def taus(text):
    rows = spurious(text)
    if rows:
        return f"{len(rows)} spurious high-priority row(s): {rows[0].strip()[:80]}"
    answer = "\n".join(answer_lines(text))
    if not (CLEAN.search(answer) or HEDGED.search(answer)):
        return "0 spurious high-priority rows, but no sentence says nothing is critical"
    return None


# @security-champion (sc2, sc3): critical, not merely high. A row is critical
# when its own priority says so (the Priority cell in a table, else the row
# text), or when it carries no priority and sits under a heading that does
# («### 🔴 Kritiske funn»). A clean verdict heading («Ingen kritiske funn»)
# marks nothing.
CRIT = re.compile(r"(?<![\wæøå])(kritisk\w*|critical)|🔴", re.IGNORECASE)
HEADING = re.compile(r"^\s*(#+\s|\*\*[^*]+\*\*:?\s*$)")


def critical_rows(text):
    # A heading with no priority of its own («### 1. SQL-injeksjon») under a
    # priority heading keeps it; one at the same level or above ends it.
    out, ctx, level = [], False, 0
    for r in answer_lines(text):
        own = None
        if r.lstrip().startswith("|"):
            cells = [c for c in r.split("|") if PRIO_EMOJI.search(c) or PRIO_WORD.fullmatch(c)]
            if cells:
                own = bool(CRIT.search(cells[0]))
        elif PRIO_EMOJI.search(r) or CRIT.search(r):
            own = bool(CRIT.search(r)) and not CLEAN.search(r)
        if HEADING.match(r):
            lv = len(r.lstrip()) - len(r.lstrip().lstrip("#")) or 7
            if own is not None:
                ctx, level = own, lv
            elif lv <= level:
                ctx, level = False, 0
        out.append((r, ctx if own is None else own))
    return out


def kritisk(text, specs):
    flags = critical_rows(text)
    rows = [r for r, _ in flags]
    crit = {r for r, c in flags if c}
    bad = []
    for name, alts in grouped(specs).items():
        near = [r for regex, want, fil in alts for r in located(rows, regex, want, fil)]
        if not near:
            bad.append(f"{name} not named near its line")
        elif not any(r in crit for r in near):
            bad.append(f"{name} not marked critical")
    return f"critical: {', '.join(bad)}" if bad else None


# A finding is a list item or a table row, with or without a line
# number. «📋 Funn: 0 kritiske, 0 høye …» is the persona's own progress line
# (agents/security-champion.agent.md) and counts nothing (#1459 review, #1462).
FINDING_ROW = re.compile(r"^\s*([-*•]|\d+[.)]|\||#+)\s*")
ZERO_COUNT = re.compile(r"(?<!\d)0\s+(kritisk\w*|critical)", re.IGNORECASE)


def kritiske(text):
    # A heading counts only when it cites a line («### 🔴 SQL i S.kt:26»).
    rows = [r for r, c in critical_rows(text)
            if c and ((FINDING_ROW.match(r) and not HEADING.match(r)) or _block(r))
            and not (RED_ZONE.search(r) or ZERO_COUNT.search(r) or CLEAN.search(r))]
    return f"{len(rows)} critical finding row(s): {rows[0].strip()[:80]}" if rows else None


LOCATION = re.compile(r"\.kts?:\d+|linje \d+|line \d+", re.IGNORECASE)


def check(cmd, args):
    """Return None when the check holds, else the reason."""
    if cmd in ("funnet", "linje"):
        return review(cmd, Path(args[0]).read_text(), args[1:])
    if cmd == "prioritet":
        return prioritet(Path(args[0]).read_text(), args[1:])
    if cmd == "taus":
        return taus(Path(args[0]).read_text())
    if cmd == "kritisk":
        return kritisk(Path(args[0]).read_text(), args[1:])
    if cmd == "kritiske":
        return kritiske(Path(args[0]).read_text())
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
    # rv6's dobbeltskriving spec, read from nav-pilot-golden.sh so the cases
    # test the pattern the suite runs.
    DW = [l.strip().strip("'") for l in (REPO / "scripts" / "nav-pilot-golden.sh").read_text().splitlines()
          if l.strip().startswith("'dobbeltskriving=")]
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
        # File-qualified spec: the right line in the wrong file is no finding.
        ("linje", "| `VedtakService.kt` | 15 | 🔴 | logger fnr |\n", ["logg=fnr@VedtakService.kt:15"], True),
        ("linje", "| `Routes.kt` | 15 | 🔴 | logger fnr |\n", ["logg=fnr@VedtakService.kt:15"], False),
        # Same name twice: either file will do.
        ("funnet", "| `VedtakRepository.kt` | 14 | 🟡 | ikke idempotent |\n",
         ["idem=idempoten@VedtakConsumer.kt:25", "idem=idempoten@VedtakRepository.kt:14"], True),
        ("funnet", "| `VedtakConsumer.kt` | 23–28 | 🔴 | Ikke idempotent |\n", ["idem=idempoten@VedtakConsumer.kt:25"], True),
        ("funnet", "| `VedtakConsumer.kt` | 23–28 | 🔴 | Ikke idempotent |\n", ["idem=idempoten@VedtakConsumer.kt:40"], False),
        ("prioritet", "| `R.kt` | 23 | 🔴 Blocker | SQL-injeksjon |\n| `S.kt` | 5 | 💭 | ubrukt import |\n",
         ["sql=injeksjon@R.kt:23", "!nit=ubrukt@S.kt:5"], True),
        ("prioritet", "| `R.kt` | 23 | 🟡 lav | SQL-injeksjon |\n", ["sql=injeksjon@R.kt:23"], False),
        ("prioritet", "| `R.kt` | 23 | 🔴 | SQL-injeksjon |\n| `S.kt` | 5 | 🔴 | ubrukt import |\n",
         ["sql=injeksjon@R.kt:23", "!nit=ubrukt@S.kt:5"], False),
        ("prioritet", "Ser bra ut.\n", ["sql=injeksjon@R.kt:23"], False),
        ("taus", "| `S.kt` | 37 | 💭 | vurder strukturert logging |\n\nIngen kritiske funn.\n", [], True),
        ("taus", "| `S.kt` | 26 | 🔴 | SQL-injeksjon |\n\nIngen kritiske funn ellers.\n", [], False),
        ("taus", "Koden ser fin ut.\n", [], False),
        ("taus", "Ingen kritiske funn.\n🔴 Rød sone: tilgangskontrollen på linje 50.\n", [], True),
        ("taus", "Høyt nivå: ingen kritiske funn. Linje 20 maskerer fnr.\n", [], True),
        # #1443: a location is a file and a line. «Fil.kt:23» in a cell or in
        # the text counts; a file named in another file's row, or a YAML key in
        # the Line cell, does not.
        ("linje", "| `Routes.kt` | 30 | 🔴 | Åpent. `nais.yaml:25–26` slipper inn alle |\n", ["inb=inbound|alle@nais.yaml:25"], True),
        ("linje", "| `Routes.kt` | 25 | 🔴 | Åpent, og nais.yaml slipper inn alle |\n", ["inb=inbound|alle@nais.yaml:25"], False),
        ("linje", "| `nais.yaml` | inbound | 🔴 | slipper inn alle |\n", ["inb=inbound|alle@nais.yaml:25"], False),
        ("linje", "| `vedtak/VedtakConsumer.kt:31–33` | 🔴 | nye vedtak ved retry |\n", ["id=vedtak@VedtakConsumer.kt:32"], True),
        # #1443: only the Priority cell carries the priority.
        ("taus", "| `S.kt` | 24 | 🟡 | Blokkerende JDBC-kall, høy belastning |\n\nIngen kritiske funn.\n", [], True),
        ("taus", "- **Blokkerende kall (linje 23):** bruk Dispatchers.IO.\n\nIngen kritiske funn.\n", [], True),
        ("prioritet", "| `R.kt` | 23 | 🟡 | SQL-injeksjon, ikke kritisk |\n", ["sql=injeksjon@R.kt:23"], False),
        ("prioritet", "| `R.kt:23` | 🔴 Blokkerer | SQL-injeksjon |\n", ["sql=injeksjon@R.kt:23"], True),
        # #1443: «ingen konkrete feil/funn» is a clean verdict.
        ("taus", "Jeg fant ingen konkrete feil i `S.kt`.\n", [], True),
        ("taus", "Ingen konkrete funn.\n", [], True),
        ("taus", "| `S.kt` | 50–55 | 🔴 Blokkerende | pid valideres ikke |\n\nIngen kritiske funn ellers.\n", [], False),
        # #1453: a hedged clean verdict counts; a 🔴 row beside it still fails.
        ("taus", "Jeg fant ingen påvist tilgang til andre personers saker.\n", [], True),
        ("taus", "Ingen åpenbar tilgangslekkasje i `S.kt`.\n", [], True),
        ("taus", "Jeg fant ingen bekreftet blokkering i `S.kt`.\n", [], True),
        ("taus", "Jeg fant ingen påvist tilgang til andre personers saker.\n| `S.kt` | 48 | 🔴 | azp sjekkes ikke |\n", [], False),
        ("taus", "| `S.kt` | 48 | 🔴 | Ingen påvist tilgang til andre saker, men azp sjekkes ikke |\n", [], False),
        ("taus", "Ingen påvist tilgangskontroll i `S.kt`.\n", [], False),
        ("taus", "Ingen tydelig feilhåndtering i `S.kt`.\n", [], False),
        # #1453: «save succeeds, publish fails» in either word order.
        ("funnet", "Ny UUID per forsøk betyr at retry etter feilet `send` gir dobbelt vedtak.\n", DW, True),
        ("funnet", "Feiler `send` eller `commitSync`, lagres vedtaket på nytt.\n", DW, True),
        ("funnet", "Vedtaket lagres før det publiseres.\n", DW, True),
        ("funnet", "Hvis prosessen dør før `commitSync`, fattes og publiseres vedtakene på nytt.\n", DW, False),
        ("funnet", "Kjør `retry` bare rundt `producer.send`. Fang deserialiseringsfeil og send til DLQ.\n", DW, False),
        ("funnet", "Feil: send til DLQ.\n", DW, False),
        ("funnet", "Meldinger som feilet sendes til DLQ.\n", DW, False),
        ("funnet", "Fang feil ved deserialisering, og send meldingen til en DLQ.\n", DW, False),
        # sc2/sc3: critical, not high; a heading carries it to rows without a priority.
        ("kritisk", "| `R.kt` | 23 | 🔴 Kritisk | SQL-injeksjon |\n", ["sql=injeksjon@R.kt:23"], True),
        ("kritisk", "| `R.kt` | 23 | 🟠 Høy | SQL-injeksjon |\n", ["sql=injeksjon@R.kt:23"], False),
        ("kritisk", "### 🔴 Kritiske funn\n\n1. SQL-injeksjon i `R.kt:23`\n", ["sql=injeksjon@R.kt:23"], True),
        ("kritisk", "### 🔴 Kritiske funn\n\n### 🟡 Medium\n\n1. SQL-injeksjon i `R.kt:23`\n", ["sql=injeksjon@R.kt:23"], False),
        ("kritisk", "Ser bra ut.\n", ["sql=injeksjon@R.kt:23"], False),
        ("kritisk", "## 🔴 Kritisk\n\n### 1. SQL-injeksjon i `R.kt:23`\n", ["sql=injeksjon@R.kt:23"], True),
        ("kritisk", "## 🔴 Kritisk\n\n## Øvrig\n\n### 1. SQL-injeksjon i `R.kt:23`\n", ["sql=injeksjon@R.kt:23"], False),
        ("kritiske", "### Ingen kritiske funn\n\n- `S.kt:37`: vurder strukturert logging (lav)\n", [], True),
        ("kritiske", "### 🔴 Kritisk\n\n- SQL-injeksjon i `S.kt:26`\n", [], False),
        ("kritiske", "| `S.kt` | 26 | 🟠 Høy | azp sjekkes ikke |\n", [], True),
        ("kritiske", "📋 Funn: 0 kritiske, 0 høye, 2 medium/lave, 5 god praksis.\n", [], True),
        ("kritiske", "### 🔴 Kritisk\n\n- SQL-injeksjon i spørringen\n", [], False),
        ("kritiske", "📋 Funn: 1 kritisk, 2 medium.\n\n- 🔴 SQL-injeksjon i `S.kt:26`\n", [], False),
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
    if sys.argv[1:2] == ["svar"] and len(sys.argv) == 3:
        # The answer alone, for the planning checks in nav-pilot-golden.sh.
        print("\n".join(answer_lines(Path(sys.argv[2]).read_text(encoding="utf-8"))))
        sys.exit(0)
    if len(sys.argv) < 3:
        sys.exit(__doc__)
    reason = check(sys.argv[1], sys.argv[2:])
    if reason:
        print(reason)
        sys.exit(1)
    if sys.argv[1] == "kritiske":
        print("0 critical finding rows")
    if sys.argv[1] == "taus":
        # rv8's verdict needs the count on a pass too (docs/modellvalg.md).
        print("0 spurious high-priority rows")
