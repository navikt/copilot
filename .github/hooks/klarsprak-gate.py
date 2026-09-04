#!/usr/bin/env python3
"""preToolUse-gate: norsk AI-markør i tekst som publiseres skal nektes.

`skills/klarsprak/SKILL.md` lister markørene, og `forfatter.agent.md` sier det
samme i persona-form. Personaen leses av modellen; den kan overses. Denne porten
sitter i verktøylaget og gjelder bare der teksten faktisk når et menneske: en
commit-melding, en `gh issue`- eller `gh pr`-tekst. Alt annet slipper gjennom.

Samme form som `ask-first-aria.py`: `permissionDecision: "deny"` framfor "ask",
fordi "ask" spør interaktivt og oppførselen i `-p` er udokumentert. "deny" er lik
i begge modus, og `permissionDecisionReason` når fram til modellen, så
blokkeringen blir selve omskrivingsoppfordringen.

Matcheren dekker `bash|shell|execute`, og navnet er målt, ikke gjettet.

Copilot CLI 1.0.83-3, prompt-modus, `GITHUB_COPILOT_PROMPT_MODE_REPO_HOOKS=true`
i et ferskt mktemp-repo med denne porten installert. Ett shell-kall ga én payload
i NAV_PILOT_HOOK_DEBUG-loggen:

    {"sessionId": ..., "timestamp": ..., "cwd": ".../ws",
     "toolName": "bash",
     "toolArgs": {"command": "rtk ls -la", "description": "List files ..."}}

Verktøynavnet er altså `bash`, og kommandoteksten ligger under `command`.
Transkriptet skriver `(shell)` for det samme kallet — 99 slike under
`docs/golden-baselines/2026-09-03-604-skill-selvutlosning/`, ved siden av
`(glob)` og `(grep)` — så `(shell)` er en visningsetikett, ikke verktøynavnet.
`shell` og `execute` (navnet agent-frontmatteren gir tillatelsen,
`code-review.agent.md:6`) står likevel i matcheren: de koster ingenting, og en
CLI som døper om verktøyet igjen skal ikke gjøre porten stum.

Kommandoteksten leses fra `command`, `commandLine`, `cmd` og `script`, rekursivt.
`command` er den målte; de tre andre er billig sikring. Fant porten ingen tekst,
gjør den ingenting — den gjetter ikke.

Kjente kanter, valgt og ikke oversett:

  * Porten parser ikke shell-sitering. Finner den et publiseringsverb, skanner
    den hele kommandostrengen. `git commit -m "..." && echo "banebrytende"`
    nektes altså på ekkoet. Over-nekting er den trygge retningen: alternativet
    er en sitat-parser som tar feil på heredoc, `$()` og nøstede fnutter.
  * `--body-file`, `-F` og `git commit -F` leser fra fil. Innholdet er ikke i
    payloaden, så porten ser det ikke. Den nekter ikke det den ikke kan lese.
  * `git commit` uten `-m` åpner en editor. Teksten skrives da utenfor kallet.
  * Ordlista er smal med vilje: bare markører som ikke finnes i vanlig norsk
    fagspråk. Kandidatene er sjekket mot `git log -200` (null treff) og mot
    resten av repoet, der de bare står i dokumentene som lister dem opp
    (klarsprak-skillen, forfatter-agenten, output-style, review-instruksjonen).
    «robust» sto i to commit-meldinger og er derfor ute, selv om skillen
    nevner den.
  * Én markør er nok. Terskelen er ikke et poengsystem: markørene under er
    ikke ord man skriver ved et uhell.
"""

import json
import os
import re
import sys

# Nøklene kommandoteksten kan komme under. Ingen av dem finnes på et
# redigeringskall, så en skriving kan ikke treffe her ved et uhell.
COMMAND_KEYS = ("command", "commandLine", "cmd", "script")

# Kallet publiserer tekst bare hvis det gjør en av disse tingene. Uten et treff
# her er kommandoen ikke porten sitt bord, uansett hva den ellers inneholder.
PUBLISHES = re.compile(
    r"""(?x)
    \bgit\s+(-\S+\s+)*commit\b
  | \bgh\s+(issue|pr)\s+(create|comment|edit|review)\b
    """,
    re.IGNORECASE,
)

# Markørene. Kilde: «Svulstige ord og uttrykk», «Åpnings- og avslutningsfraser»
# og «Engelske AI-ord» i skills/klarsprak/SKILL.md.
MARKERS = [
    r"banebrytende",
    r"revolusjonerende",
    r"sømløs\w*",
    r"holistisk\w*",
    r"helhetlig\w*",
    r"paradigmeskifte\w*",
    r"digital\w*\s+transformasjon\w*",
    r"spiller\s+en\s+avgjørende\s+rolle",
    r"et\s+betydelig\s+skritt\s+fram?over",
    r"understreker\s+behovet\s+for",
    r"tatt\s+verden\s+med\s+storm",
    r"et\s+vitnesbyrd\s+om",
    r"det\s+er\s+verdt\s+å\s+merke\s+seg",
    r"det\s+er\s+viktig\s+å\s+påpeke",
    r"i\s+dagens\s+verden",
    r"i\s+en\s+(tid|verden)\s+der",
    r"la\s+oss\s+(dykke\s+ned\s+i|utforske)",
    r"oppsummert\s+kan\s+man\s+si",
    r"avslutningsvis",
    r"taler\s+for\s+seg\s+selv",
    r"fremtiden\s+ser\s+lys\s+ut",
    r"håper\s+dette\s+hjelper",
    r"fordype\s+seg\s+i",
    r"sette\s+brukeren\s+i\s+sentrum",
    # Retoriske mønstre. Avstanden er begrenset så to urelaterte setninger ikke
    # slås sammen til et falskt treff.
    r"ikke\s+bare\b[^.\n]{0,80}\bmen\s+også",
    r"handler\s+ikke\s+om\b[^.\n]{0,80}\bmen\s+om",
]

MARKER_RE = re.compile("|".join(r"(?<![\wæøå])(?:%s)" % m for m in MARKERS), re.IGNORECASE)

REASON_HEAD = (
    "Teksten som publiseres her bærer norske AI-markører, og "
    "skills/klarsprak/SKILL.md ber deg fjerne dem før teksten når et menneske. "
    "Funnet: "
)

REASON_TAIL = (
    ". Skriv om: si hva som faktisk skjedde eller hva endringen gjør, med "
    "vanlige ord og aktiv form, og kjør kommandoen på nytt. Trenger du hele "
    "sjekklista, les skills/klarsprak/SKILL.md."
)


def commands_of(args):
    """→ [kommandostreng] for hver kommando-nøkkel i payloaden, uansett nesting."""
    out = []
    if isinstance(args, dict):
        for k, v in args.items():
            if k in COMMAND_KEYS and isinstance(v, str):
                out.append(v)
            else:
                out.extend(commands_of(v))
    elif isinstance(args, list):
        for v in args:
            out.extend(commands_of(v))
    return out


def markers_in(text):
    """→ sorterte, unike markørtreff i teksten."""
    return sorted({m.group(0).lower() for m in MARKER_RE.finditer(text)})


def decide(payload):
    """→ grunntekst hvis kallet skal nektes, ellers None."""
    args = payload.get("toolArgs")
    if args is None:
        args = payload.get("tool_input", {})

    for command in commands_of(args):
        if not PUBLISHES.search(command):
            continue
        found = markers_in(command)
        if found:
            return REASON_HEAD + ", ".join("«%s»" % f for f in found) + REASON_TAIL
    return None


def main():
    try:
        raw = sys.stdin.read()
        # Samme feilsøkingskrok som ask-first-aria.py, og av samme grunn: uten
        # den er "porten lastet ikke" og "porten traff ikke" samme observasjon
        # fra utsiden. Logg utenfor arbeidsmappa.
        debug = os.environ.get("NAV_PILOT_HOOK_DEBUG")
        if debug:
            with open(debug, "a", encoding="utf8") as fh:
                fh.write(raw.rstrip("\n") + "\n")
        payload = json.loads(raw)
        reason = decide(payload) if isinstance(payload, dict) else None
    except Exception:
        # Fail-open. En preToolUse-hook som feiler nekter kallet, og en port som
        # nekter alt er verre enn ingen port.
        reason = None

    if reason:
        json.dump(
            {
                "permissionDecision": "deny",
                "permissionDecisionReason": reason,
                "hookSpecificOutput": {
                    "hookEventName": "preToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": reason,
                },
            },
            sys.stdout,
        )
    sys.exit(0)


# ─── Selvtest ────────────────────────────────────────────────────────────────
# Kjører skriptet som subprosess med ekte stdin, slik at JSON-inn, JSON-ut og
# exitkoden er dekket. `mise run hooks:test`.

def _sh(command, tool="shell"):
    return {"toolName": tool, "toolArgs": {"command": command}}


HEREDOC = (
    "gh pr create --title 'Ny port' --body \"$(cat <<'EOF'\n"
    "Denne endringen er banebrytende.\nEOF\n)\""
)

SELFTEST = [
    # ── grunntilfellene ──────────────────────────────────────────────────────
    ("nekter commit-melding med markør",
     _sh('git commit -m "sømløs integrasjon mot Kafka"'), True),
    ("slipper gjennom ren norsk commit-melding",
     _sh('git commit -m "fix(hooks): porten leser begge dialektene"'), False),
    ("nekter gh issue create med markør",
     _sh("gh issue create --title T --body 'Dette understreker behovet for en port'"), True),
    ("nekter gh pr comment med markør",
     _sh("gh pr comment 569 --body 'Avslutningsvis: dette ser bra ut'"), True),

    # ── bare publiserende kommandoer er portens bord ─────────────────────────
    ("slipper gjennom markør i en kommando som ikke publiserer",
     _sh("grep -rn banebrytende docs/"), False),
    ("slipper gjennom en helt vanlig kommando",
     _sh("go test ./..."), False),
    ("slipper gjennom gh pr list, som ikke skriver tekst",
     _sh("gh pr list --search 'sømløs'"), False),

    # ── heredoc, som er formen en lengre PR-tekst faktisk sendes i ───────────
    ("nekter markør i heredoc til gh pr create", _sh(HEREDOC), True),

    # ── den målte payloaden, ordrett fra CLI 1.0.83-3 ───────────────────────
    ("nekter den målte bash-payloaden",
     {"sessionId": "x", "timestamp": 1788465083337, "cwd": "/tmp/ws",
      "toolName": "bash",
      "toolArgs": {"command": "git commit -m 'en helhetlig løsning'",
                   "description": "Commit changes"}}, True),
    ("slipper gjennom den målte payloaden uten markør",
     {"sessionId": "x", "timestamp": 1788465083337, "cwd": "/tmp/ws",
      "toolName": "bash",
      "toolArgs": {"command": "rtk ls -la", "description": "List files in current directory"}}, False),

    # ── begge payload-dialektene ─────────────────────────────────────────────
    ("nekter PascalCase-payloaden (tool_name/tool_input)",
     {"tool_name": "shell",
      "tool_input": {"command": 'git commit -m "et vitnesbyrd om godt arbeid"'}}, True),
    ("nekter når kommandoen ligger nøstet",
     {"toolName": "execute",
      "toolArgs": {"exec": {"cmd": "git commit -m 'i en tid der alt endres'"}}}, True),
    ("slipper gjennom payload uten kommandotekst",
     {"toolName": "shell", "toolArgs": {"description": "sømløs"}}, False),

    # ── ord repoet bruker normalt skal ikke stoppe en commit ────────────────
    ("slipper gjennom «robust», som står i ekte commit-meldinger",
     _sh('git commit -m "gjør parsingen mer robust"'), False),
    ("slipper gjennom «avgjørende» alene",
     _sh('git commit -m "avgjørende feil i merge-logikken"'), False),
    ("slipper gjennom «navigere», som er vanlig fagspråk",
     _sh('git commit -m "brukeren kan navigere mellom fanene"'), False),
    ("nekter bøyd form av en markør (\\w* i mønsteret)",
     _sh('git commit -m "en helhetlig gjennomgang av porten"'), True),

    # ── retoriske mønstre ────────────────────────────────────────────────────
    ("nekter «ikke bare X, men også Y»",
     _sh('git commit -m "porten nekter ikke bare skriving, men også shell"'), True),
    ("slipper gjennom «ikke bare» uten «men også» i nærheten",
     _sh('git commit -m "ikke bare porten. Neste steg er noe helt annet"'), False),

    # ── kjent kant: hele strengen skannes når verbet er der ──────────────────
    ("nekter markør utenfor selve meldingen når kallet også committer",
     _sh('git commit -m "ok" && echo "banebrytende"'), True),
    ("slipper gjennom --body-file, som porten ikke kan lese",
     _sh("gh issue create --title T --body-file /tmp/tekst.md"), False),
]


def selftest():
    import subprocess

    failed = 0
    for name, payload, want_deny in SELFTEST:
        p = subprocess.run(
            [sys.executable, __file__],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
        )
        got_deny = False
        if p.stdout.strip():
            got_deny = json.loads(p.stdout).get("permissionDecision") == "deny"
        ok = p.returncode == 0 and got_deny == want_deny
        print(f"{'✅' if ok else '❌'} {name}")
        if not ok:
            failed += 1
            print(f"   exit={p.returncode} deny={got_deny} want={want_deny}")
            print(f"   stdout={p.stdout!r} stderr={p.stderr!r}")
    print(f"\n{len(SELFTEST) - failed}/{len(SELFTEST)} ok")
    return 1 if failed else 0


if __name__ == "__main__":
    if "--selftest" in sys.argv:
        sys.exit(selftest())
    main()
